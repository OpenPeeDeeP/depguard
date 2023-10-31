package main

import (
	"embed"
	"testing"

	"github.com/OpenPeeDeeP/depguard/v2"
	"github.com/google/go-cmp/cmp"
)

//go:embed testfiles/*
var testfiles embed.FS

var expectedConfigStruct = &depguard.LinterSettings{
	"main": &depguard.List{
		ListMode: "Strict",
		Files:    []string{"$all", "!$test"},
		Allow:    []string{"$gostd", "github.com/"},
		Deny: map[string]string{
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

func TestJsonConfigurator(t *testing.T) {
	con := &jsonConfigurator{}
	f, err := testfiles.Open("testfiles/.depguard.json")
	if err != nil {
		t.Fatal("could not read embedded file")
	}
	set, err := con.parse(f)
	if err != nil {
		t.Fatalf("file is not a valid json file: %s", err)
	}
	diff := cmp.Diff(expectedConfigStruct, set)
	if diff != "" {
		t.Errorf("did not create expected config\n%s", diff)
	}
}

func TestYamlConfigurator(t *testing.T) {
	con := &yamlConfigurator{}
	f, err := testfiles.Open("testfiles/.depguard.yaml")
	if err != nil {
		t.Fatal("could not read embedded file")
	}
	set, err := con.parse(f)
	if err != nil {
		t.Fatalf("file is not a valid yaml file: %s", err)
	}
	diff := cmp.Diff(expectedConfigStruct, set)
	if diff != "" {
		t.Errorf("did not create expected config\n%s", diff)
	}
}

func TestTomlConfigurator(t *testing.T) {
	con := &tomlConfigurator{}
	f, err := testfiles.Open("testfiles/.depguard.toml")
	if err != nil {
		t.Fatal("could not read embedded file")
	}
	set, err := con.parse(f)
	if err != nil {
		t.Fatalf("file is not a valid toml file: %s", err)
	}
	diff := cmp.Diff(expectedConfigStruct, set)
	if diff != "" {
		t.Errorf("did not create expected config\n%s", diff)
	}
}
