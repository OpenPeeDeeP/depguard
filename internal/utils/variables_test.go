package utils

import (
	"testing"

	"github.com/gobwas/glob"
)

func TestAllExpander(t *testing.T) {
	exp := &allExpander{}
	pre, err := exp.Expand()
	if err != nil {
		t.Fatal("expansion method returned an error")
	}
	if len(pre) != 1 {
		t.Fatal("expected only 1 expansion")
	}
	g, err := glob.Compile(pre[0], '/')
	if err != nil {
		t.Fatal("glob is not compilable")
	}
	if !g.Match("/some/folder/system/some_test.go") {
		t.Error("glob should match a test file")
	}
	if !g.Match("/some/folder/system/some.go") {
		t.Error("glob should not match a normal go file")
	}
}

func TestTestExpander(t *testing.T) {
	exp := &testExpander{}
	pre, err := exp.Expand()
	if err != nil {
		t.Fatal("expansion method returned an error")
	}
	if len(pre) != 1 {
		t.Fatal("expected only 1 expansion")
	}
	g, err := glob.Compile(pre[0], '/')
	if err != nil {
		t.Fatal("glob is not compilable")
	}
	if g.Match("/some/folder/system/some.go") {
		t.Error("glob should not match a normal go file")
	}
	if !g.Match("/some/folder/system/some_test.go") {
		t.Error("glob doesn't match a test file")
	}
}

func TestGoStdExpander(t *testing.T) {
	exp := &gostdExpander{}
	pre, err := exp.Expand()
	if err != nil {
		t.Fatal("expansion method returned an error")
	}
	if len(pre) == 0 {
		t.Fatal("expected more than 1 expansion")
	}
	// Just make sure a few are in there
	if !contains(pre, "os") && !contains(pre, "strings") {
		t.Error("could not find some of the expected packages")
	}
}

func contains(sl []string, str string) bool {
	for _, s := range sl {
		if s == str {
			return true
		}
	}
	return false
}
