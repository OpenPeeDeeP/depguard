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

type insertSliceScenario struct {
	name     string
	first    []string
	second   []string
	idx      int
	expected []string
}

var (
	insertSliceScenarios = []*insertSliceScenario{
		{
			name:     "start",
			first:    []string{"a", "b", "c", "d", "e"},
			second:   []string{"f", "g", "h"},
			idx:      0,
			expected: []string{"f", "g", "h", "b", "c", "d", "e"},
		},
		{
			name:     "middle",
			first:    []string{"a", "b", "c", "d", "e"},
			second:   []string{"f", "g", "h"},
			idx:      2,
			expected: []string{"a", "b", "f", "g", "h", "d", "e"},
		},
		{
			name:     "end",
			first:    []string{"a", "b", "c", "d", "e"},
			second:   []string{"f", "g", "h"},
			idx:      4,
			expected: []string{"a", "b", "c", "d", "f", "g", "h"},
		},
	}
)

func testInsertSlice(s *insertSliceScenario) func(*testing.T) {
	return func(t *testing.T) {
		act := insertSlice(s.first, s.idx, s.second...)
		diff := cmp.Diff(s.expected, act)
		if diff != "" {
			t.Errorf("actual slice differs from expected\n%s", diff)
		}
	}
}

func TestInsertSlice(t *testing.T) {
	for _, s := range insertSliceScenarios {
		t.Run(s.name, testInsertSlice(s))
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

func TestExpandSlice(t *testing.T) {
	t.Run("successful", func(t *testing.T) {
		some := []string{"a", "$succ", "b"}
		exp := []string{"a", "FIND ME", "FIND ME TOO", "b"}
		act, err := ExpandSlice(some, expandables)
		if err != nil {
			t.Fatal("should not get an error")
		}
		diff := cmp.Diff(exp, act)
		if diff != "" {
			t.Errorf("slices don't match\n%s", diff)
		}
	})
	t.Run("failure", func(t *testing.T) {
		some := []string{"a", "$fail", "b"}
		_, err := ExpandSlice(some, expandables)
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "$fail") {
			t.Error("error string should contain the key that failed")
		}
	})
}

func TestExpandMap(t *testing.T) {
	t.Run("successful", func(t *testing.T) {
		some := map[string]string{
			"a":     "Use b",
			"$succ": "Use stdlib",
			"b":     "Use a",
		}
		exp := map[string]string{
			"a":           "Use b",
			"FIND ME":     "Use stdlib",
			"FIND ME TOO": "Use stdlib",
			"b":           "Use a",
		}
		err := ExpandMap(some, expandables)
		if err != nil {
			t.Fatal("should not get an error")
		}
		diff := cmp.Diff(exp, some)
		if diff != "" {
			t.Errorf("maps don't match\n%s", diff)
		}
	})
	t.Run("failure", func(t *testing.T) {
		some := map[string]string{
			"a":     "Use b",
			"$fail": "Use stdlib",
			"b":     "Use a",
		}
		err := ExpandMap(some, expandables)
		if err == nil {
			t.Fatal("expected and error")
		}
		if !strings.Contains(err.Error(), "$fail") {
			t.Error("error string should contain the key that failed")
		}
	})
}
