package main

import (
	"embed"
	"io/fs"
	"testing"

	"github.com/OpenPeeDeeP/depguard/v2"
	"github.com/google/go-cmp/cmp"
)

//go:embed testfiles/*
var testfiles embed.FS

var expectedConfigStruct = &depguard.LinterSettings{
	"main": &depguard.List{
		Files: []string{"$all", "!$test"},
		Allow: []string{"$gostd", "github.com/"},
		Deny: map[string]string{
			"github.com/**/test":     "No test packages allowed",
			"reflect":                "Who needs reflection",
			"github.com/OpenPeeDeeP": "Use Something Else",
		},
	},
	"tests": &depguard.List{
		Files: []string{"$test"},
		Allow: []string{"github.com/test"},
		Deny: map[string]string{
			"github.com/OpenPeeDeeP/": "Use Something Else",
		},
	},
}

func TestConfigurators(t *testing.T) {
	mustGetFile := func(name string) fs.File {
		f, err := testfiles.Open(name)
		if err != nil {
			t.Fatal("could not read embedded file")
		}
		return f
	}
	cases := []struct {
		name         string
		inputFile    fs.File
		configurator configurator
	}{
		{
			name:         "json",
			inputFile:    mustGetFile("testfiles/.depguard.json"),
			configurator: &jsonConfigurator{},
		},
		{
			name:         "yaml",
			inputFile:    mustGetFile("testfiles/.depguard.yaml"),
			configurator: &yamlConfigurator{},
		},
		{
			name:         "toml",
			inputFile:    mustGetFile("testfiles/.depguard.toml"),
			configurator: &tomlConfigurator{},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			set, err := c.configurator.parse(c.inputFile)
			if err != nil {
				t.Fatalf("file is not a valid json file: %s", err)
			}
			diff := cmp.Diff(expectedConfigStruct, set)
			if diff != "" {
				t.Errorf("did not create expected config\n%s", diff)
			}
		})
	}
}
