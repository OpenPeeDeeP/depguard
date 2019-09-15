package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/parser"
	"go/types"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"text/template"

	"github.com/OpenPeeDeeP/depguard"
	"github.com/kisielk/gotool"
	"golang.org/x/tools/go/loader"
)

const (
	whitelistMsg = " was not in the whitelist"
	blacklistMsg = " was in the blacklist"
)

var (
	configFile string
	cpuProfile string
	memProfile string
)

func init() {
	flag.StringVar(&configFile, "c", ".depguard.json", "Location of the config file")
	flag.StringVar(&cpuProfile, "cpu", "", "write cpu profile to `file`")
	flag.StringVar(&memProfile, "mem", "", "write memory profile to `file`")
}

type config struct {
	Type                      string            `json:"type"`
	Packages                  []string          `json:"packages"`
	PackagesWithErrorMessages map[string]string `json:"packagesWithErrorMessages"`
	IncludeGoRoot             bool              `json:"includeGoRoot"`
	IncludeGoStdLib           bool              `json:"includeGoStdLib"`
	InTests                   []string          `json:"inTests"`
	listType                  depguard.ListType
}

func parseConfigFile() (*config, error) {
	file, err := os.Open(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			if configFile != ".depguard.json" {
				return nil, err
			}
			return &config{}, nil
		}
		return nil, err
	}
	defer file.Close()
	var c *config
	err = json.NewDecoder(file).Decode(&c)
	if err != nil {
		return nil, err
	}

	c.IncludeGoRoot = c.IncludeGoRoot || c.IncludeGoStdLib

	var found bool
	c.listType, found = depguard.StringToListType[strings.ToLower(c.Type)]
	if !found {
		if c.Type != "" {
			return nil, fmt.Errorf("Unsure what list type %s is", c.Type)
		}
		c.listType = depguard.LTBlacklist
	}

	if c.listType == depguard.LTBlacklist && c.PackagesWithErrorMessages != nil {
		// add any packages that are only in PackgesWithErrorMessages
		// to the packages list to be blacklisted
		for _, pkg := range c.Packages {
			if _, ok := c.PackagesWithErrorMessages[pkg]; !ok {
				c.PackagesWithErrorMessages[pkg] = ""
			}
		}

		// recreate the packages list so that it has all packages
		c.Packages = make([]string, 0, len(c.PackagesWithErrorMessages))

		for pkg := range c.PackagesWithErrorMessages {
			c.Packages = append(c.Packages, pkg)
		}
	}

	return c, nil
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
	conf, prog, err := getConfigAndProgram(config)
	if err != nil {
		log.Fatalln(err)
	}
	dg := &depguard.Depguard{
		Packages:      config.Packages,
		IncludeGoRoot: config.IncludeGoRoot,
		ListType:      config.listType,
		TestPackages:  config.InTests,
	}
	issues, err := dg.Run(conf, prog)
	if err != nil {
		log.Fatalln(err)
	}
	printIssues(config, issues)

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

func getConfigAndProgram(depguardConf *config) (*loader.Config, *loader.Program, error) {
	paths := gotool.ImportPaths(flag.Args())
	conf := new(loader.Config)
	conf.ParserMode = parser.ImportsOnly
	conf.AllowErrors = true
	conf.TypeChecker = types.Config{
		Error: eatErrors,
	}
	rest, err := conf.FromArgs(paths, len(depguardConf.InTests) > 0)
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

func printIssues(c *config, issues []*depguard.Issue) {
	if len(issues) == 0 {
		return
	}
	temp := template.Must(template.New("issues").Parse(`{{ .Position.Filename }}:{{ .Position.Line }}:{{ .Position.Column }}:{{ .PackageName }}`))
	buf := new(bytes.Buffer)
	var str strings.Builder
	for _, issue := range issues {
		temp.Execute(buf, issue)
		if c.listType == depguard.LTWhitelist {
			str.WriteString(buf.String() + whitelistMsg)
		} else {
			str.WriteString(buf.String() + blacklistMsg)
		}

		// check to see if an additional error message was supplied for the package
		if msg, ok := c.PackagesWithErrorMessages[issue.PackageName]; ok && msg != "" {
			str.WriteString(": " + msg)
		}
		fmt.Println(str.String())
		str.Reset()
		buf.Reset()
	}
}

// Since I am allowing errors to happen, I don't want them to print to screen.
func eatErrors(err error) {}
