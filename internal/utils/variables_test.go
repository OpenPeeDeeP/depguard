package utils

import (
	"errors"
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
	if !contains(pre, "os") && !contains(pre, "strings") {
		t.Error("could not find some of the expected packages")
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
	t.Run("successful", func(ts *testing.T) {
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
	t.Run("failure", func(ts *testing.T) {
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
	t.Run("successful", func(ts *testing.T) {
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
	t.Run("failure", func(ts *testing.T) {
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

func contains(sl []string, str string) bool {
	for _, s := range sl {
		if s == str {
			return true
		}
	}
	return false
}
