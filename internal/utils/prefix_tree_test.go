package utils

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"testing"
)

type prefixTreeInsert struct {
	str  string
	deny bool
	sug  int
}

func allowEntries(strs ...string) []prefixTreeInsert {
	inserts := make([]prefixTreeInsert, 0, len(strs))
	for _, str := range strs {
		inserts = append(inserts, prefixTreeInsert{str: str})
	}
	return inserts
}

func denyEntry(str string, sug int) prefixTreeInsert {
	return prefixTreeInsert{str: str, deny: true, sug: sug}
}

func buildPrefixTree(inserts []prefixTreeInsert) *CompressedPrefixTree {
	tree := NewCompressedPrefixTree()
	for _, ins := range inserts {
		if ins.deny {
			tree.InsertDeny(ins.str, ins.sug)
			continue
		}
		tree.InsertAllow(ins.str)
	}
	return tree
}

// Search walks the import once and returns the length of the longest matching
// allow entry and deny entry (-1 for no match), plus the suggestion index of the
// matching deny entry. Lengths count the $ of exact entries, so an exact entry
// beats a prefix entry for the same string, like len(entry) does today.
type prefixTreeSearchScenario struct {
	name    string
	inserts []prefixTreeInsert
	search  string
	allow   string // allow entry (as inserted) that should match, "" for none
	deny    string // deny entry (as inserted) that should match, "" for none
	sug     int    // suggestion index expected when deny is set
}

var (
	radixWords  = []string{"romane", "romanus", "romulus", "rubens", "ruber", "rubicon", "rubicundus"}
	importPaths = []string{
		"github.com/org/legacy/",
		"github.com/org/legacy/auth/",
		"github.com/org/v2",
	}
	nestedDeny = []prefixTreeInsert{
		denyEntry("some/pkg/a", 0),
		denyEntry("some/pkg/a/foo", 1),
		denyEntry("some/pkg/b/", 2),
		denyEntry("some/pkg/b/foo$", 3),
	}
	allowAndDeny = []prefixTreeInsert{{str: "some/pkg/a/foo"}, denyEntry("some/pkg/a", 0)}
	exactTie     = []prefixTreeInsert{{str: "some/pkg/e$"}, denyEntry("some/pkg/e", 0)}
	denyBothKind = []prefixTreeInsert{denyEntry("ab", 0), denyEntry("ab$", 1)}
	dotEntries   = []prefixTreeInsert{{str: "path"}, {str: "go.opentelemetry.io"}}

	prefixTreeSearchScenarios = []*prefixTreeSearchScenario{
		{name: "empty tree", inserts: nil, search: "abc"},
		{name: "same as inserted", inserts: allowEntries("abc"), search: "abc", allow: "abc"},
		{name: "shorter than inserted", inserts: allowEntries("abc"), search: "ab"},
		{name: "longer than inserted", inserts: allowEntries("abc"), search: "abcd", allow: "abc"},
		{name: "diverges from inserted", inserts: allowEntries("abc"), search: "abd"},
		{name: "raw prefix", inserts: allowEntries("github.com/bob"), search: "github.com/bob2", allow: "github.com/bob"},
		{name: "no shared first byte", inserts: allowEntries("abc", "xyz"), search: "xyz", allow: "xyz"},
		{name: "split keeps original", inserts: allowEntries("abc", "abd"), search: "abc", allow: "abc"},
		{name: "split adds new", inserts: allowEntries("abc", "abd"), search: "abd", allow: "abd"},
		{name: "split node is not an entry", inserts: allowEntries("abc", "abd"), search: "ab"},
		{name: "shorter inserted after longer", inserts: allowEntries("abc", "ab"), search: "ab", allow: "ab"},
		{name: "longer kept after shorter inserted", inserts: allowEntries("abc", "ab"), search: "abc", allow: "abc"},
		{name: "longer inserted after shorter", inserts: allowEntries("ab", "abc"), search: "abc", allow: "abc"},
		{name: "duplicate insert", inserts: allowEntries("abc", "abc"), search: "abc", allow: "abc"},
		{name: "insert ends at existing split node", inserts: allowEntries("abc", "abd", "ab"), search: "ab", allow: "ab"},
		{name: "insert ends above split node", inserts: allowEntries("abc", "abd", "a"), search: "ab", allow: "a"},
		{name: "longest prefix wins", inserts: allowEntries("ab", "abc"), search: "abcd", allow: "abc"},
		{name: "prefix stops partway into child", inserts: allowEntries("ab", "abcd"), search: "abcx", allow: "ab"},
		{name: "exact matches itself", inserts: allowEntries("abc$"), search: "abc", allow: "abc$"},
		{name: "exact does not match longer", inserts: allowEntries("abc$"), search: "abcd"},
		{name: "exact passed through to longer entry", inserts: allowEntries("ab$", "abc"), search: "abc", allow: "abc"},
		{name: "exact passed through to longer prefix", inserts: allowEntries("ab$", "abc"), search: "abcd", allow: "abc"},
		{name: "longer exact falls back to prefix", inserts: allowEntries("ab", "abc$"), search: "abcd", allow: "ab"},
		{name: "exact inserted at split", inserts: allowEntries("abc", "ab$"), search: "abcd", allow: "abc"},
		{name: "exact split keeps exactness", inserts: allowEntries("abc$", "abd"), search: "abcd"},
		{name: "prefix and exact same entry longer", inserts: allowEntries("ab$", "ab"), search: "abc", allow: "ab"},
		{name: "prefix and exact same entry exact wins", inserts: allowEntries("ab$", "ab"), search: "ab", allow: "ab$"},
		{name: "radix words first", inserts: allowEntries(radixWords...), search: "romane", allow: "romane"},
		{name: "radix words middle", inserts: allowEntries(radixWords...), search: "rubens", allow: "rubens"},
		{name: "radix words last", inserts: allowEntries(radixWords...), search: "rubicundus", allow: "rubicundus"},
		{name: "radix words shared prefix", inserts: allowEntries(radixWords...), search: "rom"},
		{name: "radix words partial branch", inserts: allowEntries(radixWords...), search: "rubic"},
		{name: "import path root", inserts: allowEntries(importPaths...), search: "github.com/org/legacy/", allow: "github.com/org/legacy/"},
		{name: "import path nested", inserts: allowEntries(importPaths...), search: "github.com/org/legacy/auth/", allow: "github.com/org/legacy/auth/"},
		{name: "import path sibling", inserts: allowEntries(importPaths...), search: "github.com/org/v2", allow: "github.com/org/v2"},
		{name: "import path shared prefix", inserts: allowEntries(importPaths...), search: "github.com/org/"},
		{name: "import path sibling sorted before nested", inserts: allowEntries(importPaths...), search: "github.com/org/legacy/api", allow: "github.com/org/legacy/"},
		{name: "import path sibling sorted after nested", inserts: allowEntries(importPaths...), search: "github.com/org/legacy/db", allow: "github.com/org/legacy/"},
		{name: "import path below nested", inserts: allowEntries(importPaths...), search: "github.com/org/legacy/auth/x", allow: "github.com/org/legacy/auth/"},
		{name: "unrelated import", inserts: allowEntries(importPaths...), search: "zilld.io/pkg/e"},
		{name: "relative import of root entry", inserts: allowEntries("os/exec", "path"), search: "./os/exec"},
		{name: "absolute import of root entry", inserts: allowEntries("os/exec", "path"), search: "/os/exec"},
		{name: "import path without trailing slash", inserts: allowEntries(importPaths...), search: "github.com/org/legacy"},
		{name: "relative path entry", inserts: allowEntries("./relative/path"), search: "./relative/path/nested", allow: "./relative/path"},
		{name: "absolute path entry", inserts: allowEntries("/absolute/path"), search: "/absolute/path/nested", allow: "/absolute/path"},
		{name: "exact std entry beside domain", inserts: allowEntries("go/ast$", "go/token$", "go.opentelemetry.io", "golang.org/x"), search: "go/token", allow: "go/token$"},
		{name: "exact std entry subpackage", inserts: allowEntries("go/ast$", "go/token$", "go.opentelemetry.io", "golang.org/x"), search: "go/token/fake"},
		{name: "domain beside exact std entries", inserts: allowEntries("go/ast$", "go/token$", "go.opentelemetry.io", "golang.org/x"), search: "go.opentelemetry.io/otel", allow: "go.opentelemetry.io"},
		{name: "import path nested exact", inserts: allowEntries("github.com/org/legacy/", "github.com/org/legacy/auth$"), search: "github.com/org/legacy/auth/x", allow: "github.com/org/legacy/"},
		{name: "deny nested uses nested suggestion", inserts: nestedDeny, search: "some/pkg/a/foo/bar", deny: "some/pkg/a/foo", sug: 1},
		{name: "deny sibling sorted before nested", inserts: nestedDeny, search: "some/pkg/a/bar", deny: "some/pkg/a", sug: 0},
		{name: "deny sibling sorted after nested", inserts: nestedDeny, search: "some/pkg/a/zed", deny: "some/pkg/a", sug: 0},
		{name: "deny nested exact uses exact suggestion", inserts: nestedDeny, search: "some/pkg/b/foo", deny: "some/pkg/b/foo$", sug: 3},
		{name: "deny sibling sorted after nested exact", inserts: nestedDeny, search: "some/pkg/b/zed", deny: "some/pkg/b/", sug: 2},
		{name: "deny below nested exact", inserts: nestedDeny, search: "some/pkg/b/foo/bar", deny: "some/pkg/b/", sug: 2},
		{name: "allow and deny on different nodes", inserts: allowAndDeny, search: "some/pkg/a/foo/bar", allow: "some/pkg/a/foo", deny: "some/pkg/a", sug: 0},
		{name: "allow and deny only deny matches", inserts: allowAndDeny, search: "some/pkg/a/baz", deny: "some/pkg/a", sug: 0},
		{name: "exact allow and prefix deny same entry", inserts: exactTie, search: "some/pkg/e", allow: "some/pkg/e$", deny: "some/pkg/e", sug: 0},
		{name: "exact allow and prefix deny subpackage", inserts: exactTie, search: "some/pkg/e/x", deny: "some/pkg/e", sug: 0},
		{name: "deny prefix and exact same entry exact", inserts: denyBothKind, search: "ab", deny: "ab$", sug: 1},
		{name: "deny prefix and exact same entry prefix", inserts: denyBothKind, search: "ab/x", deny: "ab", sug: 0},
		{name: "root entry matches subpackage", inserts: dotEntries, search: "path/filepath", allow: "path"},
		{name: "domain entry matches with dot", inserts: dotEntries, search: "go.opentelemetry.io/otel", allow: "go.opentelemetry.io"},
	}
)

func expectedSearchLen(entry string) int {
	if entry == "" {
		return -1
	}
	return len(entry)
}

func testPrefixTreeSearch(s *prefixTreeSearchScenario) func(*testing.T) {
	return func(t *testing.T) {
		allowLen, denyLen, sugIdx := buildPrefixTree(s.inserts).Search(s.search)
		if exp := expectedSearchLen(s.allow); allowLen != exp {
			t.Errorf("allow length: expected %d (%q) - got %d", exp, s.allow, allowLen)
		}
		if exp := expectedSearchLen(s.deny); denyLen != exp {
			t.Errorf("deny length: expected %d (%q) - got %d", exp, s.deny, denyLen)
		}
		expSug := -1
		if s.deny != "" {
			expSug = s.sug
		}
		if sugIdx != expSug {
			t.Errorf("suggestion index: expected %d - got %d", expSug, sugIdx)
		}
	}
}

func TestPrefixTreeSearch(t *testing.T) {
	for _, s := range prefixTreeSearchScenarios {
		t.Run(s.name, testPrefixTreeSearch(s))
	}
}

type prefixTreeStructureScenario struct {
	name     string
	inserts  []prefixTreeInsert
	expected string
}

// Trees are written as {child,child}, where each child is its prefix, the
// entries that end there in brackets, and its own children. Children are sorted
// by their first byte. Entries are a* (allow prefix), a$ (allow exact),
// d*:n (deny prefix with suggestion index n) and d$:n (deny exact with
// suggestion index n).
var prefixTreeStructureScenarios = []*prefixTreeStructureScenario{
	{name: "empty", inserts: nil, expected: "{}"},
	{name: "single", inserts: allowEntries("abc"), expected: "{abc[a*]}"},
	{name: "no shared first byte", inserts: allowEntries("abc", "xyz"), expected: "{abc[a*],xyz[a*]}"},
	{name: "duplicate insert", inserts: allowEntries("abc", "abc"), expected: "{abc[a*]}"},
	{name: "split", inserts: allowEntries("abc", "abd"), expected: "{ab{c[a*],d[a*]}}"},
	{name: "shorter after longer", inserts: allowEntries("abc", "ab"), expected: "{ab[a*]{c[a*]}}"},
	{name: "longer after shorter", inserts: allowEntries("ab", "abc"), expected: "{ab[a*]{c[a*]}}"},
	{name: "split keeps children", inserts: allowEntries("abcd", "abce", "ax"), expected: "{a{bc{d[a*],e[a*]},x[a*]}}"},
	{name: "insert ends at existing split node", inserts: allowEntries("abc", "abd", "ab"), expected: "{ab[a*]{c[a*],d[a*]}}"},
	{name: "insert ends above split node", inserts: allowEntries("abc", "abd", "a"), expected: "{a[a*]{b{c[a*],d[a*]}}}"},
	{name: "radix words", inserts: allowEntries(radixWords...), expected: "{r{om{an{e[a*],us[a*]},ulus[a*]},ub{e{ns[a*],r[a*]},ic{on[a*],undus[a*]}}}}"},
	{name: "exact single", inserts: allowEntries("abc$"), expected: "{abc[a$]}"},
	{name: "exact split keeps exactness", inserts: allowEntries("abc$", "abd"), expected: "{ab{c[a$],d[a*]}}"},
	{name: "exact inserted at split", inserts: allowEntries("abc", "ab$"), expected: "{ab[a$]{c[a*]}}"},
	{name: "prefix inserted at exact split", inserts: allowEntries("abc$", "ab"), expected: "{ab[a*]{c[a$]}}"},
	{name: "exact inserted at existing split node", inserts: allowEntries("abc", "abd", "ab$"), expected: "{ab[a$]{c[a*],d[a*]}}"},
	{name: "exact then longer", inserts: allowEntries("ab$", "abc"), expected: "{ab[a$]{c[a*]}}"},
	{name: "exact then prefix same entry", inserts: allowEntries("ab$", "ab"), expected: "{ab[a*,a$]}"},
	{name: "prefix then exact same entry", inserts: allowEntries("ab", "ab$"), expected: "{ab[a*,a$]}"},
	{name: "deny single", inserts: []prefixTreeInsert{denyEntry("abc", 0)}, expected: "{abc[d*:0]}"},
	{name: "deny exact single", inserts: []prefixTreeInsert{denyEntry("abc$", 0)}, expected: "{abc[d$:0]}"},
	{name: "deny split", inserts: []prefixTreeInsert{denyEntry("abc", 0), denyEntry("abd", 1)}, expected: "{ab{c[d*:0],d[d*:1]}}"},
	{name: "deny at existing split node", inserts: []prefixTreeInsert{denyEntry("abc", 0), denyEntry("abd", 1), denyEntry("ab", 2)}, expected: "{ab[d*:2]{c[d*:0],d[d*:1]}}"},
	{name: "deny then longer", inserts: []prefixTreeInsert{denyEntry("ab", 0), denyEntry("abc", 1)}, expected: "{ab[d*:0]{c[d*:1]}}"},
	{name: "allow and deny same entry", inserts: []prefixTreeInsert{{str: "ab"}, denyEntry("ab", 0)}, expected: "{ab[a*,d*:0]}"},
	{name: "deny and allow same entry", inserts: []prefixTreeInsert{denyEntry("ab", 0), {str: "ab"}}, expected: "{ab[a*,d*:0]}"},
	{name: "allow root deny nested", inserts: []prefixTreeInsert{{str: "ab"}, denyEntry("abc", 0)}, expected: "{ab[a*]{c[d*:0]}}"},
	{name: "deny nested allow root", inserts: []prefixTreeInsert{denyEntry("abc", 0), {str: "ab"}}, expected: "{ab[a*]{c[d*:0]}}"},
	{name: "allow exact deny prefix same entry", inserts: []prefixTreeInsert{{str: "ab$"}, denyEntry("ab", 0)}, expected: "{ab[a$,d*:0]}"},
	{name: "deny prefix and exact same entry", inserts: []prefixTreeInsert{denyEntry("ab", 0), denyEntry("ab$", 1)}, expected: "{ab[d*:0,d$:1]}"},
	{name: "deny exact and prefix same entry", inserts: []prefixTreeInsert{denyEntry("ab$", 1), denyEntry("ab", 0)}, expected: "{ab[d*:0,d$:1]}"},
	{name: "deny exact split keeps index", inserts: []prefixTreeInsert{denyEntry("abc$", 0), denyEntry("abd", 1)}, expected: "{ab{c[d$:0],d[d*:1]}}"},
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

func dumpPrefixTreeNode(n *compressedPrefixTreeNode) string {
	var entries []string
	if n.flags&allowPrefix != 0 {
		entries = append(entries, "a*")
	}
	if n.flags&allowExact != 0 {
		entries = append(entries, "a$")
	}
	if n.flags&denyPrefix != 0 {
		entries = append(entries, fmt.Sprintf("d*:%d", n.prefixSugIdx))
	}
	if n.flags&denyExact != 0 {
		entries = append(entries, fmt.Sprintf("d$:%d", n.exactSugIdx))
	}
	out := n.prefix
	if len(entries) > 0 {
		out += "[" + strings.Join(entries, ",") + "]"
	}
	if len(n.children) > 0 {
		out += dumpPrefixTreeChildren(n)
	}
	return out
}

// checkPrefixTreeNodes reports nodes whose suggestion indexes disagree with
// their flags (a deny prefix entry has a prefix suggestion index, a deny exact
// entry has an exact suggestion index, and every other index is -1), and nodes
// with children that would hide each other (empty prefix or same first byte).
func checkPrefixTreeNodes(t *testing.T, path string, n *compressedPrefixTreeNode) {
	t.Helper()
	path += n.prefix
	if (n.flags&denyPrefix != 0) != (n.prefixSugIdx != -1) {
		t.Errorf("node %q: deny prefix is %v but prefix suggestion index is %d", path, n.flags&denyPrefix != 0, n.prefixSugIdx)
	}
	if (n.flags&denyExact != 0) != (n.exactSugIdx != -1) {
		t.Errorf("node %q: deny exact is %v but exact suggestion index is %d", path, n.flags&denyExact != 0, n.exactSugIdx)
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
		tree := buildPrefixTree(s.inserts)
		act := dumpPrefixTreeChildren(tree.root)
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

func TestPrefixTreeInsertOrder(t *testing.T) {
	reversed := slices.Clone(radixWords)
	slices.Reverse(reversed)
	forward := dumpPrefixTreeChildren(buildPrefixTree(allowEntries(radixWords...)).root)
	backward := dumpPrefixTreeChildren(buildPrefixTree(allowEntries(reversed...)).root)
	if forward != backward {
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
