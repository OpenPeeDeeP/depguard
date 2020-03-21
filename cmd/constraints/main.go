package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/parser"
	"go/types"
	"html/template"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"

	"github.com/OpenPeeDeeP/depguard"
	"github.com/kisielk/gotool"
	"golang.org/x/tools/go/loader"
)

const (
	whitelistMsg          = " was not in the whitelist"
	blacklistMsg          = " was in the blacklist"
	fileNotConstrainedMsg = " does not have a corresponding constraint"
)

var (
	configFile  string
	cpuProfile  string
	memProfile  string
	includeTest bool
)

func init() {
	flag.StringVar(&configFile, "c", ".depguard.json", "Location of the config file")
	flag.StringVar(&cpuProfile, "cpu", "", "write cpu profile to `file`")
	flag.StringVar(&memProfile, "mem", "", "write memory profile to `file`")
	flag.BoolVar(&includeTest, "test", false, "include test")
}

func main() {
	flag.Parse()
	if cpuProfile != "" {
		f, err := os.Create(cpuProfile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}

	config, err := parseConfigFile()
	if err != nil {
		log.Fatalln(err)
	}
	conf, prog, err := getConfigAndProgram()
	if err != nil {
		log.Fatalln(err)
	}

	issues, err := config.Run(conf, prog)
	if err != nil {
		log.Fatalln(err)
	}
	printIssues(issues)

	if memProfile != "" {
		f, err := os.Create(memProfile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}
		defer f.Close()
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
	}
}

type config struct {
	AllowMissing bool                       `json:"allowMissing"`
	Constraints  map[string]*confConstraint `json:"constraints"`
}

type confConstraint struct {
	Type          string   `json:"type"`
	Packages      []string `json:"packages"`
	Priority      int      `json:"priority"`
	IncludeGoRoot bool     `json:"includeGoRoot"`
}

func parseConfigFile() (*depguard.Constraints, error) {
	file, err := os.Open(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			if configFile != ".depguard.json" {
				return nil, err
			}
			return &depguard.Constraints{}, nil
		}
		return nil, err
	}
	defer file.Close()
	var c *config
	err = json.NewDecoder(file).Decode(&c)
	if err != nil {
		return nil, err
	}

	conf := &depguard.Constraints{
		Constraints: make(map[string]*depguard.Constraint),
	}
	conf.AllowMissing = c.AllowMissing

	for key, val := range c.Constraints {
		constr := &depguard.Constraint{
			IncludeGoRoot: val.IncludeGoRoot,
			Priority:      val.Priority,
			Packages:      val.Packages,
		}
		var found bool
		constr.ListType, found = depguard.StringToListType[strings.ToLower(val.Type)]
		if !found {
			if val.Type != "" {
				return nil, fmt.Errorf("Unsure what list type %s is", val.Type)
			}
			constr.ListType = depguard.LTBlacklist
		}
		conf.Constraints[key] = constr
	}
	return conf, nil
}

func getConfigAndProgram() (*loader.Config, *loader.Program, error) {
	paths := gotool.ImportPaths(flag.Args())
	conf := new(loader.Config)
	conf.ParserMode = parser.ImportsOnly
	conf.AllowErrors = true
	conf.TypeChecker = types.Config{
		Error: eatErrors,
	}
	rest, err := conf.FromArgs(paths, includeTest)
	if err != nil {
		return nil, nil, err
	}
	if len(rest) > 0 {
		return nil, nil, fmt.Errorf("Too many args: %v", rest)
	}
	prog, err := conf.Load()
	if err != nil {
		return nil, nil, err
	}
	return conf, prog, nil
}

var defIssue = template.Must(template.New("issues").Parse(`{{ .Position.Filename }}:{{ .Position.Line }}:{{ .Position.Column }}:`))

func printIssues(issues []*depguard.Issue) {
	if len(issues) == 0 {
		return
	}
	buf := new(bytes.Buffer)
	var str strings.Builder
	for _, issue := range issues {
		defIssue.Execute(buf, issue)
		if issue.PackageName == "" {
			str.WriteString(issue.Position.Filename + fileNotConstrainedMsg)
		} else if issue.ListType == depguard.LTWhitelist {
			str.WriteString(buf.String() + issue.PackageName + " " + whitelistMsg)
		} else {
			str.WriteString(buf.String() + issue.PackageName + " " + blacklistMsg)
		}
		fmt.Println(str.String())
		str.Reset()
		buf.Reset()
	}
}

// Since I am allowing errors to happen, I don't want them to print to screen.
func eatErrors(err error) {}
