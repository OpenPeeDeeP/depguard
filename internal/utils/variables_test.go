package utils

import (
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/gobwas/glob"
	"github.com/google/go-cmp/cmp"
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
	if !slices.Contains(pre, "os$") || !slices.Contains(pre, "strings$") || !slices.Contains(pre, "io/fs$") {
		t.Error("could not find some of the expected packages")
	}
}

type findStdPkgsScenario struct {
	name      string
	goroot    string
	expectErr bool
	contains  []string
	excludes  []string
}

func testFindStdPkgs(s *findStdPkgsScenario) func(*testing.T) {
	return func(t *testing.T) {
		t.Setenv("GOROOT", s.goroot)
		pre, err := findStdPkgs()
		if s.expectErr {
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), "GOROOT") {
				t.Error("error string should mention GOROOT")
			}
			return
		}
		if err != nil {
			t.Fatal("finding std packages returned an error")
		}
		for _, pkg := range s.contains {
			if !slices.Contains(pre, pkg) {
				t.Errorf("expected %s to be in the std packages", pkg)
			}
		}
		for _, pkg := range s.excludes {
			if slices.Contains(pre, pkg) {
				t.Errorf("expected %s to not be in the std packages", pkg)
			}
		}
	}
}

func TestFindStdPkgs(t *testing.T) {
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		t.Fatal("could not get GOROOT from go env")
	}
	goroot := strings.TrimSpace(string(out))
	// "archive" is a directory with no go files, so neither path should return it.
	// "syscall/js" only builds for GOOS=js, so only the GOROOT walk returns it
	// as it does not check build constraints.
	scenarios := []*findStdPkgsScenario{
		{
			name:     "goroot",
			goroot:   goroot,
			contains: []string{"os$", "strings$", "io/fs$", "syscall/js$"},
			excludes: []string{"archive$", "cmd/go$", "vendor$", "internal/abi$", "net/http/internal$"},
		},
		{
			name:     "go_list",
			goroot:   "",
			contains: []string{"os$", "strings$", "io/fs$"},
			excludes: []string{"archive$", "syscall/js$", "cmd/go$", "vendor$", "internal/abi$", "net/http/internal$"},
		},
		{
			name:      "bad_goroot",
			goroot:    "/nonexistent/goroot",
			expectErr: true,
		},
	}
	for _, s := range scenarios {
		t.Run(s.name, testFindStdPkgs(s))
	}
}

type isStdPkgScenario struct {
	name     string
	pkg      string
	expected bool
}

var (
	isStdPkgScenarios = []*isStdPkgScenario{
		{name: "root", pkg: "os", expected: true},
		{name: "nested", pkg: "net/http", expected: true},
		{name: "cmd", pkg: "cmd", expected: false},
		{name: "cmd_nested", pkg: "cmd/go", expected: false},
		{name: "vendor", pkg: "vendor", expected: false},
		{name: "vendor_nested", pkg: "vendor/golang.org/x/net/dns/dnsmessage", expected: false},
		{name: "internal", pkg: "internal", expected: false},
		{name: "internal_root", pkg: "internal/abi", expected: false},
		{name: "internal_middle", pkg: "crypto/internal/fips140", expected: false},
		{name: "internal_leaf", pkg: "net/http/internal", expected: false},
		{name: "internal_lookalike", pkg: "notinternal", expected: true},
	}
)

func testIsStdPkg(s *isStdPkgScenario) func(*testing.T) {
	return func(t *testing.T) {
		if isStdPkg(s.pkg) != s.expected {
			t.Errorf("expected isStdPkg(%q) to be %v", s.pkg, s.expected)
		}
	}
}

func TestIsStdPkg(t *testing.T) {
	for _, s := range isStdPkgScenarios {
		t.Run(s.name, testIsStdPkg(s))
	}
}

type expanderTest struct{}

func (*expanderTest) Expand() ([]string, error) {
	return []string{"FIND ME", "FIND ME TOO"}, nil
}

type expanderFailTest struct{}

func (*expanderFailTest) Expand() ([]string, error) {
	return nil, errors.New("expected error")
}

var (
	expandables = ExpanderMap{
		"$succ": &expanderTest{},
		"$fail": &expanderFailTest{},
	}
)

type expanderMapExpandScenario struct {
	name    string
	entry   string
	exp     []string
	fromVar bool
	expErr  string // substring the error must contain
}

var expanderMapExpandScenarios = []*expanderMapExpandScenario{
	{
		name:    "variable",
		entry:   "$succ",
		exp:     []string{"FIND ME", "FIND ME TOO"},
		fromVar: true,
	},
	{
		name:  "plain entry",
		entry: "github.com/org/",
	},
	{
		name:  "unknown variable is a plain entry",
		entry: "$nope",
	},
	{
		name:  "variable must be the whole entry",
		entry: "$succ/x",
	},
	{
		name:    "failure names the variable",
		entry:   "$fail",
		fromVar: true,
		expErr:  "$fail",
	},
}

func testExpanderMapExpand(s *expanderMapExpandScenario) func(*testing.T) {
	return func(t *testing.T) {
		act, fromVar, err := expandables.Expand(s.entry)
		if s.expErr != "" {
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), s.expErr) {
				t.Errorf("error should contain %q: %v", s.expErr, err)
			}
		} else if err != nil {
			t.Fatalf("not expecting an error: %v", err)
		}
		if fromVar != s.fromVar {
			t.Errorf("fromVar: got %v, want %v", fromVar, s.fromVar)
		}
		if diff := cmp.Diff(s.exp, act); diff != "" {
			t.Errorf("expansion doesn't match\n%s", diff)
		}
	}
}

func TestExpanderMapExpand(t *testing.T) {
	for _, s := range expanderMapExpandScenarios {
		t.Run(s.name, testExpanderMapExpand(s))
	}
}
