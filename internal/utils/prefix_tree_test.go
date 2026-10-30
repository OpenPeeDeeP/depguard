package utils

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// buildPrefixTree inserts each key with the span [i, i+1), so a span reported
// by a walk names the key it came from.
func buildPrefixTree(keys ...string) *CompressedPrefixTree {
	tree := NewCompressedPrefixTree()
	for i, key := range keys {
		tree.Insert(key, uint32(i), uint32(i+1))
	}
	return tree
}

// walkStep is one node with rules reported by a walk: the key inserted there
// and whether the walked string ends at that node.
type walkStep struct {
	key   string
	atEnd bool
}

func walkPrefixTree(t *testing.T, keys []string, str string) []walkStep {
	t.Helper()
	var steps []walkStep
	buildPrefixTree(keys...).Walk(str, func(start, end uint32, atEnd bool) {
		if end != start+1 || int(start) >= len(keys) {
			t.Fatalf("walk reported span [%d, %d), which wasn't inserted", start, end)
		}
		steps = append(steps, walkStep{key: keys[start], atEnd: atEnd})
	})
	return steps
}

// A walk reports, from the root down, every inserted key that's a prefix of
// the string, plus whether the string ends there. Exactness and match length
// are the solver's job, not the tree's.
type prefixTreeWalkScenario struct {
	name     string
	keys     []string
	str      string
	expected []walkStep
}

var (
	radixWords  = []string{"romane", "romanus", "romulus", "rubens", "ruber", "rubicon", "rubicundus"}
	importPaths = []string{
		"github.com/org/legacy/",
		"github.com/org/legacy/auth/",
		"github.com/org/v2",
	}
	stdBesideDomains = []string{"go/ast", "go/token", "go.opentelemetry.io", "golang.org/x"}

	prefixTreeWalkScenarios = []*prefixTreeWalkScenario{
		{name: "empty tree", keys: nil, str: "abc"},
		{name: "empty string", keys: []string{"abc"}, str: ""},
		{name: "same as inserted", keys: []string{"abc"}, str: "abc", expected: []walkStep{{"abc", true}}},
		{name: "shorter than inserted", keys: []string{"abc"}, str: "ab"},
		{name: "longer than inserted", keys: []string{"abc"}, str: "abcd", expected: []walkStep{{"abc", false}}},
		{name: "diverges from inserted", keys: []string{"abc"}, str: "abd"},
		{name: "raw prefix", keys: []string{"github.com/bob"}, str: "github.com/bob2", expected: []walkStep{{"github.com/bob", false}}},
		{name: "no shared first byte", keys: []string{"abc", "xyz"}, str: "xyz", expected: []walkStep{{"xyz", true}}},
		{name: "split keeps original", keys: []string{"abc", "abd"}, str: "abc", expected: []walkStep{{"abc", true}}},
		{name: "split adds new", keys: []string{"abc", "abd"}, str: "abd", expected: []walkStep{{"abd", true}}},
		{name: "split node has no rules", keys: []string{"abc", "abd"}, str: "ab"},
		{name: "split node is skipped", keys: []string{"abc", "abd"}, str: "abcd", expected: []walkStep{{"abc", false}}},
		{name: "shorter inserted after longer", keys: []string{"abc", "ab"}, str: "abc", expected: []walkStep{{"ab", false}, {"abc", true}}},
		{name: "longer inserted after shorter", keys: []string{"ab", "abc"}, str: "abc", expected: []walkStep{{"ab", false}, {"abc", true}}},
		{name: "insert ends at existing split node", keys: []string{"abc", "abd", "ab"}, str: "abd", expected: []walkStep{{"ab", false}, {"abd", true}}},
		{name: "insert ends above split node", keys: []string{"abc", "abd", "a"}, str: "abc", expected: []walkStep{{"a", false}, {"abc", true}}},
		{name: "every prefix on the path", keys: []string{"a", "ab", "abc"}, str: "abcd", expected: []walkStep{{"a", false}, {"ab", false}, {"abc", false}}},
		{name: "stops partway into child", keys: []string{"ab", "abcd"}, str: "abcx", expected: []walkStep{{"ab", false}}},
		{name: "string ends inside child", keys: []string{"ab", "abcd"}, str: "abc", expected: []walkStep{{"ab", false}}},
		{name: "radix words first", keys: radixWords, str: "romane", expected: []walkStep{{"romane", true}}},
		{name: "radix words middle", keys: radixWords, str: "rubens", expected: []walkStep{{"rubens", true}}},
		{name: "radix words last", keys: radixWords, str: "rubicundus", expected: []walkStep{{"rubicundus", true}}},
		{name: "radix words shared prefix", keys: radixWords, str: "rom"},
		{name: "radix words partial branch", keys: radixWords, str: "rubic"},
		{name: "import path nested", keys: importPaths, str: "github.com/org/legacy/auth/x", expected: []walkStep{{"github.com/org/legacy/", false}, {"github.com/org/legacy/auth/", false}}},
		{name: "import path shared prefix", keys: importPaths, str: "github.com/org/"},
		{name: "import path without trailing slash", keys: importPaths, str: "github.com/org/legacy"},
		// The nested-entry bug (#115): siblings of a nested key, sorted either side of it
		{name: "import path sibling sorted before nested", keys: importPaths, str: "github.com/org/legacy/api", expected: []walkStep{{"github.com/org/legacy/", false}}},
		{name: "import path sibling sorted after nested", keys: importPaths, str: "github.com/org/legacy/db", expected: []walkStep{{"github.com/org/legacy/", false}}},
		{name: "unrelated import", keys: importPaths, str: "zilld.io/pkg/e"},
		{name: "relative import of root key", keys: []string{"os/exec", "path"}, str: "./os/exec"},
		{name: "absolute import of root key", keys: []string{"os/exec", "path"}, str: "/os/exec"},
		{name: "relative path key", keys: []string{"./relative/path"}, str: "./relative/path/nested", expected: []walkStep{{"./relative/path", false}}},
		{name: "std beside domain", keys: stdBesideDomains, str: "go/token", expected: []walkStep{{"go/token", true}}},
		{name: "domain beside std", keys: stdBesideDomains, str: "go.opentelemetry.io/otel", expected: []walkStep{{"go.opentelemetry.io", false}}},
		{name: "root only", keys: []string{""}, str: "abc", expected: []walkStep{{"", false}}},
		{name: "root with empty string", keys: []string{""}, str: "", expected: []walkStep{{"", true}}},
		{name: "root then path", keys: []string{"ab", ""}, str: "abc", expected: []walkStep{{"", false}, {"ab", false}}},
		{name: "root when nothing else matches", keys: []string{"ab", ""}, str: "xyz", expected: []walkStep{{"", false}}},
	}
)

func testPrefixTreeWalk(s *prefixTreeWalkScenario) func(*testing.T) {
	return func(t *testing.T) {
		act := walkPrefixTree(t, s.keys, s.str)
		if !slices.Equal(act, s.expected) {
			t.Errorf("walk of %q: expected %v - got %v", s.str, s.expected, act)
		}
	}
}

func TestPrefixTreeWalk(t *testing.T) {
	for _, s := range prefixTreeWalkScenarios {
		t.Run(s.name, testPrefixTreeWalk(s))
	}
}

// TestPrefixTreeWalkSpan checks spans wider than one rule come back as
// inserted, and survive the node being split.
func TestPrefixTreeWalkSpan(t *testing.T) {
	tree := NewCompressedPrefixTree()
	tree.Insert("abc", 0, 3)
	tree.Insert("", 3, 4)
	tree.Insert("ab", 4, 9) // splits "abc"
	type span struct {
		start, end uint32
		atEnd      bool
	}
	var act []span
	tree.Walk("abc", func(start, end uint32, atEnd bool) {
		act = append(act, span{start, end, atEnd})
	})
	exp := []span{{3, 4, false}, {4, 9, false}, {0, 3, true}}
	if !slices.Equal(act, exp) {
		t.Errorf("expected spans %v - got %v", exp, act)
	}
}

type prefixTreeInsertPanicScenario struct {
	name string
	keys []string
}

// compile inserts each key once, so a second insert is a bug that would
// otherwise drop rules without a trace.
var prefixTreeInsertPanicScenarios = []*prefixTreeInsertPanicScenario{
	{name: "duplicate leaf", keys: []string{"abc", "abc"}},
	{name: "duplicate after split", keys: []string{"abc", "abd", "abc"}},
	{name: "duplicate split node", keys: []string{"abc", "abd", "ab", "ab"}},
	{name: "duplicate shorter key", keys: []string{"ab", "abc", "ab"}},
	{name: "duplicate root", keys: []string{"", "a", ""}},
}

func testPrefixTreeInsertPanic(s *prefixTreeInsertPanicScenario) func(*testing.T) {
	return func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Errorf("expected inserting %q to panic", s.keys)
			}
		}()
		buildPrefixTree(s.keys...)
	}
}

func TestPrefixTreeInsertPanic(t *testing.T) {
	for _, s := range prefixTreeInsertPanicScenarios {
		t.Run(s.name, testPrefixTreeInsertPanic(s))
	}
}

type prefixTreeStructureScenario struct {
	name     string
	keys     []string
	expected string
}

// Trees are written as {child,child}, where each child is its prefix, the
// index of the key inserted there in brackets, and its own children. Children
// are sorted by their first byte. A key on the root shows before the braces.
var prefixTreeStructureScenarios = []*prefixTreeStructureScenario{
	{name: "empty", keys: nil, expected: "{}"},
	{name: "single", keys: []string{"abc"}, expected: "{abc[0]}"},
	{name: "no shared first byte", keys: []string{"abc", "xyz"}, expected: "{abc[0],xyz[1]}"},
	{name: "split", keys: []string{"abc", "abd"}, expected: "{ab{c[0],d[1]}}"},
	{name: "shorter after longer", keys: []string{"abc", "ab"}, expected: "{ab[1]{c[0]}}"},
	{name: "longer after shorter", keys: []string{"ab", "abc"}, expected: "{ab[0]{c[1]}}"},
	{name: "split keeps children", keys: []string{"abcd", "abce", "ax"}, expected: "{a{bc{d[0],e[1]},x[2]}}"},
	{name: "insert ends at existing split node", keys: []string{"abc", "abd", "ab"}, expected: "{ab[2]{c[0],d[1]}}"},
	{name: "insert ends above split node", keys: []string{"abc", "abd", "a"}, expected: "{a[2]{b{c[0],d[1]}}}"},
	{name: "radix words", keys: radixWords, expected: "{r{om{an{e[0],us[1]},ulus[2]},ub{e{ns[3],r[4]},ic{on[5],undus[6]}}}}"},
	{name: "root", keys: []string{""}, expected: "[0]{}"},
	{name: "root and children", keys: []string{"ab", "", "ac"}, expected: "[1]{a{b[0],c[2]}}"},
}

func dumpPrefixTreeChildren(n *compressedPrefixTreeNode) string {
	sorted := slices.Clone(n.children)
	slices.SortFunc(sorted, func(a, b *compressedPrefixTreeNode) int {
		return cmp.Compare(a.prefix[0], b.prefix[0])
	})
	children := make([]string, 0, len(sorted))
	for _, child := range sorted {
		children = append(children, dumpPrefixTreeNode(child))
	}
	return "{" + strings.Join(children, ",") + "}"
}

func dumpPrefixTreeSpan(n *compressedPrefixTreeNode) string {
	if !n.spanSet() {
		return ""
	}
	return fmt.Sprintf("[%d]", n.start)
}

func dumpPrefixTreeNode(n *compressedPrefixTreeNode) string {
	out := n.prefix + dumpPrefixTreeSpan(n)
	if len(n.children) > 0 {
		out += dumpPrefixTreeChildren(n)
	}
	return out
}

func dumpPrefixTree(tree *CompressedPrefixTree) string {
	return dumpPrefixTreeSpan(tree.root) + dumpPrefixTreeChildren(tree.root)
}

// checkPrefixTreeNodes reports nodes with a backwards span, and nodes with
// children that would hide each other (empty prefix or same first byte).
func checkPrefixTreeNodes(t *testing.T, path string, n *compressedPrefixTreeNode) {
	t.Helper()
	path += n.prefix
	if n.start > n.end {
		t.Errorf("node %q: span [%d, %d) is backwards", path, n.start, n.end)
	}
	firstBytes := make(map[byte]bool, len(n.children))
	for _, child := range n.children {
		if child.prefix == "" {
			t.Errorf("node %q: has a child with an empty prefix", path)
			continue
		}
		if firstBytes[child.prefix[0]] {
			t.Errorf("node %q: has more than one child starting with %q", path, child.prefix[0])
		}
		firstBytes[child.prefix[0]] = true
		checkPrefixTreeNodes(t, path, child)
	}
}

func testPrefixTreeStructure(s *prefixTreeStructureScenario) func(*testing.T) {
	return func(t *testing.T) {
		tree := buildPrefixTree(s.keys...)
		act := dumpPrefixTree(tree)
		if act != s.expected {
			t.Errorf("tree structure differs: Exp %s, Act %s", s.expected, act)
		}
		checkPrefixTreeNodes(t, "", tree.root)
	}
}

func TestPrefixTreeStructure(t *testing.T) {
	for _, s := range prefixTreeStructureScenarios {
		t.Run(s.name, testPrefixTreeStructure(s))
	}
}

// TestPrefixTreeInsertOrder checks the shape doesn't depend on insert order.
// Spans name keys by insert position, so they're left out of the comparison.
func TestPrefixTreeInsertOrder(t *testing.T) {
	shape := func(keys []string) string {
		tree := NewCompressedPrefixTree()
		for _, key := range keys {
			tree.Insert(key, 1, 2)
		}
		return dumpPrefixTree(tree)
	}
	reversed := slices.Clone(radixWords)
	slices.Reverse(reversed)
	if forward, backward := shape(radixWords), shape(reversed); forward != backward {
		t.Errorf("tree depends on insert order: forward %s, reversed %s", forward, backward)
	}
}

type commonPrefixLengthScenario struct {
	name     string
	a        string
	b        string
	expected int
}

var commonPrefixLengthScenarios = []*commonPrefixLengthScenario{
	{name: "both empty", a: "", b: "", expected: 0},
	{name: "one empty", a: "abc", b: "", expected: 0},
	{name: "no common", a: "abc", b: "xyz", expected: 0},
	{name: "partial", a: "abc", b: "abd", expected: 2},
	{name: "a is prefix of b", a: "ab", b: "abc", expected: 2},
	{name: "b is prefix of a", a: "abc", b: "ab", expected: 2},
	{name: "equal", a: "abc", b: "abc", expected: 3},
}

func testCommonPrefixLength(s *commonPrefixLengthScenario) func(*testing.T) {
	return func(t *testing.T) {
		if act := commonPrefixLength(s.a, s.b); act != s.expected {
			t.Errorf("expected commonPrefixLength(%q, %q) to be %d - got %d", s.a, s.b, s.expected, act)
		}
	}
}

func TestCommonPrefixLength(t *testing.T) {
	for _, s := range commonPrefixLengthScenarios {
		t.Run(s.name, testCommonPrefixLength(s))
	}
}
