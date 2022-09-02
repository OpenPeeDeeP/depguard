package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	depguard "github.com/OpenPeeDeeP/depguard/v2"
	"golang.org/x/tools/go/analysis/singlechecker"
	"gopkg.in/yaml.v3"
)

var configFileRE = regexp.MustCompile(`^\.?depguard\.(yaml|yml|json|toml)$`)

var (
	fileTypes = map[string]configurator{
		"toml": &tomlConfigurator{},
		"yaml": &yamlConfigurator{},
		"yml":  &yamlConfigurator{},
		"json": &jsonConfigurator{},
	}
)

func main() {
	settings, err := getSettings()
	if err != nil {
		fmt.Printf("Could not find or read configuration file: %s\nUsing default configuration\n", err)
		settings = &depguard.LinterSettings{}
	}
	analyzer, err := depguard.NewAnalyzer(settings)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	singlechecker.Main(analyzer)
}

type configurator interface {
	parse(io.Reader) (*depguard.LinterSettings, error)
}

type jsonConfigurator struct{}

func (*jsonConfigurator) parse(r io.Reader) (*depguard.LinterSettings, error) {
	set := &depguard.LinterSettings{}
	err := json.NewDecoder(r).Decode(set)
	if err != nil {
		return nil, fmt.Errorf("could not parse json file: %w", err)
	}
	return set, nil
}

type tomlConfigurator struct{}

func (*tomlConfigurator) parse(r io.Reader) (*depguard.LinterSettings, error) {
	set := &depguard.LinterSettings{}
	_, err := toml.NewDecoder(r).Decode(set)
	if err != nil {
		return nil, fmt.Errorf("could not parse toml file: %w", err)
	}
	return set, nil
}

type yamlConfigurator struct{}

func (*yamlConfigurator) parse(r io.Reader) (*depguard.LinterSettings, error) {
	set := &depguard.LinterSettings{}
	err := yaml.NewDecoder(r).Decode(set)
	if err != nil {
		return nil, fmt.Errorf("could not parse yaml file: %w", err)
	}
	return set, nil
}

func getSettings() (*depguard.LinterSettings, error) {
	fs := os.DirFS(".")
	f, ft, err := findFile(fs)
	if err != nil {
		return nil, err
	}
	file, err := fs.Open(f)
	if err != nil {
		return nil, fmt.Errorf("could not open %s to read: %w", f, err)
	}
	defer file.Close()
	return ft.parse(file)
}

func findFile(fsys fs.FS) (string, configurator, error) {
	cwd, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return "", nil, fmt.Errorf("could not read cwd: %w", err)
	}
	for _, entry := range cwd {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		matches := configFileRE.FindStringSubmatch(name)
		if len(matches) != 2 {
			continue
		}
		return matches[0], fileTypes[matches[1]], nil
	}
	return "", nil, errors.New("unable to find a configuration file")
}
