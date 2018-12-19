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

var configFile string

func init() {
	flag.StringVar(&configFile, "c", ".depguard.json", "Location of the config file")
}

type config struct {
	Type          string   `json:"type"`
	Packages      []string `json:"packages"`
	IncludeGoRoot bool     `json:"includeGoRoot"`
	InTests       []string `json:"inTests"`
	listType      depguard.ListType
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
	var found bool
	c.listType, found = depguard.StringToListType[strings.ToLower(c.Type)]
	if !found {
		if c.Type != "" {
			return nil, fmt.Errorf("Unsure what list type %s is", c.Type)
		}
		c.listType = depguard.LTBlacklist
	}
	return c, nil
}

func main() {
	flag.Parse()
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
	for _, issue := range issues {
		temp.Execute(buf, issue)
		if c.listType == depguard.LTWhitelist {
			fmt.Println(buf.String() + whitelistMsg)
		} else {
			fmt.Println(buf.String() + blacklistMsg)
		}
		buf.Reset()
	}
}

// Since I am allowing errors to happen, I don't want them to print to screen.
func eatErrors(err error) {}
