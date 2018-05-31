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
	listType      depguard.ListType
}

func parseConfigFile() (*config, error) {
	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var c *config
	err = json.NewDecoder(file).Decode(&c)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(c.Type) {
	case "whitelist":
		c.listType = depguard.LTWhitelist
	case "blacklist":
		c.listType = depguard.LTBlacklist
	default:
		return nil, fmt.Errorf("Unsure what list type %s is", c.Type)
	}
	return c, nil
}

func main() {
	flag.Parse()
	config, err := parseConfigFile()
	if err != nil {
		log.Fatalln(err)
	}
	conf, prog, err := getConfigAndProgram()
	if err != nil {
		log.Fatalln(err)
	}
	dg := &depguard.Depguard{
		Packages:      config.Packages,
		IncludeGoRoot: config.IncludeGoRoot,
		ListType:      config.listType,
	}
	issues, err := dg.Run(conf, prog)
	if err != nil {
		log.Fatalln(err)
	}
	printIssues(config, issues)
}

func getConfigAndProgram() (*loader.Config, *loader.Program, error) {
	paths := gotool.ImportPaths(flag.Args())
	conf := new(loader.Config)
	conf.ParserMode = parser.ImportsOnly
	conf.AllowErrors = true
	conf.TypeChecker = types.Config{
		Error: eatErrors,
	}
	rest, err := conf.FromArgs(paths, false)
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

//Since I am allowing errors to happen, I don't want them to print to screen.
func eatErrors(err error) {}
