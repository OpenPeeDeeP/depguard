package depguard

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/OpenPeeDeeP/depguard/v2/internal/utils"
	"github.com/gobwas/glob"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

type listCompileScenario struct {
	name   string
	list   *List
	exp    *list
	allow  []string          // entries (as written) that must be allowed by the compiled tree
	deny   map[string]string // entries (as written) that must be denied, with their suggestion
	expErr error
}

type settingsCompileScenario struct {
	name     string
	settings LinterSettings
	exp      linterSettings
	allow    map[string][]string // allow entries (as written) expected per list name
	expErr   error
}

func cmpGlobPattern(a, b *glob.Pattern) bool {
	return a.String() == b.String()
}

var (
	listCompileScenarios = []*listCompileScenario{
		{
			name: "Requires Allow And/Or Deny",
			list: &List{
				Files: []string{"**/*.go"},
			},
			expErr: errors.New("must have an Allow and/or Deny package list"),
		},
		{
			name: "No Files",
			list: &List{
				Allow: []string{"os"},
				Deny: map[string]string{
					"reflect": "Don't use Reflect",
				},
			},
			exp: &list{
				anyAllows: true,
			},
			allow: []string{"os"},
			deny: map[string]string{
				"reflect": "Don't use Reflect",
			},
		},
		{
			name: "Expanded Files",
			list: &List{
				Files: []string{"$all"},
				Allow: []string{"os"},
			},
			exp: &list{
				files: []*glob.Pattern{
					glob.MustCompile("**/*.go", '/'),
				},
				anyAllows: true,
			},
			allow: []string{"os"},
		},
		{
			name: "Expanded Negate Files",
			list: &List{
				Files: []string{"!$test"},
				Allow: []string{"os"},
			},
			exp: &list{
				negFiles: []*glob.Pattern{
					glob.MustCompile("**/*_test.go", '/'),
				},
				anyAllows: true,
			},
			allow: []string{"os"},
		},
		{
			name: "Normal and Negatable Files",
			list: &List{
				Files: []string{"**/foo.go", "!**/bar.go"},
				Allow: []string{"os"},
			},
			exp: &list{
				files: []*glob.Pattern{
					glob.MustCompile("**/foo.go", '/'),
				},
				negFiles: []*glob.Pattern{
					glob.MustCompile("**/bar.go", '/'),
				},
				anyAllows: true,
			},
			allow: []string{"os"},
		},
		{
			name: "Failure to Compile File Glob",
			list: &List{
				Files: []string{"[a-]/*.go"},
			},
			expErr: errors.New("[a-]/*.go could not be compiled"),
		},
		{
			name: "Expanded Allow",
			list: &List{
				Allow: []string{"$gostd"},
			},
			exp: &list{
				anyAllows: true,
			},
			allow: []string{"FIND ME", "FIND ME TOO"},
		},
		{
			name: "Expanded Deny",
			list: &List{
				Deny: map[string]string{"$gostd": "Don't use standard"},
			},
			exp: &list{},
			deny: map[string]string{
				"FIND ME":     "Don't use standard",
				"FIND ME TOO": "Don't use standard",
			},
		},
		{
			name: "Only Deny",
			list: &List{
				Deny: map[string]string{
					"reflect": "Don't use Reflect",
				},
			},
			exp: &list{},
			deny: map[string]string{
				"reflect": "Don't use Reflect",
			},
		},
		{
			name: "Only Allow",
			list: &List{
				Allow: []string{"os"},
			},
			exp: &list{
				anyAllows: true,
			},
			allow: []string{"os"},
		},
		{
			name: "Allow And Deny",
			list: &List{
				Files: []string{"**/*.go", "!**/*_test.go"},
				Allow: []string{"os"},
				Deny: map[string]string{
					"reflect": "Don't use Reflect",
				},
			},
			exp: &list{
				files: []*glob.Pattern{
					glob.MustCompile("**/*.go", '/'),
				},
				negFiles: []*glob.Pattern{
					glob.MustCompile("**/*_test.go", '/'),
				},
				anyAllows: true,
			},
			allow: []string{"os"},
			deny: map[string]string{
				"reflect": "Don't use Reflect",
			},
		},
		{
			name: "Original Mode Default",
			list: &List{
				Allow: []string{"os"},
				Deny: map[string]string{
					"reflect": "Don't use Reflect",
				},
			},
			exp: &list{
				listMode:  listModeOriginal,
				anyAllows: true,
			},
			allow: []string{"os"},
			deny: map[string]string{
				"reflect": "Don't use Reflect",
			},
		},
		{
			name: "Set Original Mode",
			list: &List{
				ListMode: "oRiGinal",
				Allow:    []string{"os"},
				Deny: map[string]string{
					"reflect": "Don't use Reflect",
				},
			},
			exp: &list{
				listMode:  listModeOriginal,
				anyAllows: true,
			},
			allow: []string{"os"},
			deny: map[string]string{
				"reflect": "Don't use Reflect",
			},
		},
		{
			name: "Set Strict Mode",
			list: &List{
				ListMode: "sTrIct",
				Allow:    []string{"os"},
				Deny: map[string]string{
					"reflect": "Don't use Reflect",
				},
			},
			exp: &list{
				listMode:  listModeStrict,
				anyAllows: true,
			},
			allow: []string{"os"},
			deny: map[string]string{
				"reflect": "Don't use Reflect",
			},
		},
		{
			name: "Set Lax Mode",
			list: &List{
				ListMode: "lAx",
				Allow:    []string{"os"},
				Deny: map[string]string{
					"reflect": "Don't use Reflect",
				},
			},
			exp: &list{
				listMode:  listModeLax,
				anyAllows: true,
			},
			allow: []string{"os"},
			deny: map[string]string{
				"reflect": "Don't use Reflect",
			},
		},
		{
			name: "Unknown List Mode",
			list: &List{
				ListMode: "MiddleOut",
				Allow:    []string{"os"},
				Deny: map[string]string{
					"reflect": "Don't use Reflect",
				},
			},
			expErr: errors.New("MiddleOut is not a known list mode"),
		},
		{
			name: "Empty Allow Package",
			list: &List{
				Allow: []string{""},
			},
			expErr: errors.New("allow contains an empty package"),
		},
		{
			name: "Empty Deny Package",
			list: &List{
				Deny: map[string]string{"": "no pkg"},
			},
			expErr: errors.New("deny contains an empty package"),
		},
		{
			name: "Allow Package Only $",
			list: &List{
				Allow: []string{"os", "$"},
			},
			expErr: errors.New("allow contains a package with only $"),
		},
		{
			name: "Deny Package Only $",
			list: &List{
				Deny: map[string]string{"os": "no os", "$": "no pkg"},
			},
			expErr: errors.New("deny contains a package with only $"),
		},
	}
	settingsCompileScenarios = []*settingsCompileScenario{
		{
			name: "Zero State",
			exp: []*list{
				{
					name: "Main",
					files: []*glob.Pattern{
						glob.MustCompile("**/*.go", '/'),
					},
					anyAllows: true,
				},
			},
			allow: map[string][]string{"Main": {"FIND ME", "FIND ME TOO"}},
		},
		{
			name: "Name is injected",
			settings: LinterSettings{
				"Test": &List{
					Files: []string{"$test"},
					Allow: []string{"os"},
				},
				"Main": &List{
					Files: []string{"$all"},
					Allow: []string{"os"},
				},
			},
			exp: []*list{
				{
					name: "Main",
					files: []*glob.Pattern{
						glob.MustCompile("**/*.go", '/'),
					},
					anyAllows: true,
				},
				{
					name: "Test",
					files: []*glob.Pattern{
						glob.MustCompile("**/*_test.go", '/'),
					},
					anyAllows: true,
				},
			},
			allow: map[string][]string{"Main": {"os"}, "Test": {"os"}},
		},
		{
			name: "Error names the list",
			settings: LinterSettings{
				"bad": &List{
					Deny: map[string]string{"": "no pkg"},
				},
			},
			expErr: errors.New("bad: deny contains an empty package"),
		},
	}
)

func testListCompile(s *listCompileScenario) func(*testing.T) {
	return func(t *testing.T) {
		act, err := s.list.compile()
		if s.expErr != nil {
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), s.expErr.Error()) {
				t.Errorf("error does not contain expected string: Exp %s, Act %s", s.expErr, err)
			}
			return
		}
		if err != nil {
			t.Fatal("not expecting an error")
		}
		diff := cmp.Diff(s.exp, act, cmp.AllowUnexported(list{}), cmp.Comparer(cmpGlobPattern), ignoreCompiledEntries)
		if diff != "" {
			t.Errorf("compiled list is not what was expected\n%s", diff)
		}
		checkCompiledEntries(t, act, s.allow, s.deny)
	}
}

// The tree, suggestions and rules are checked by behavior in
// checkCompiledEntries, as the suggestion indexes depend on map iteration
// order.
var ignoreCompiledEntries = cmpopts.IgnoreFields(list{}, "tree", "suggestions", "rules")

// keyRules returns the rules on the tree node for key: the span the walk
// reports where the key ends.
func keyRules(li *list, key string) []rule {
	var rules []rule
	li.tree.Walk(key, func(start, end uint32, atEnd bool) {
		if atEnd {
			rules = li.rules[start:end]
		}
	})
	return rules
}

// hasRule reports whether entry (as written, with any $) compiled to a rule of
// kind on its key's node, and returns it.
func hasRule(li *list, entry string, kind ruleKind) (rule, bool) {
	key, exact := strings.CutSuffix(entry, "$")
	for _, r := range keyRules(li, key) {
		if r.kind == kind && r.exact == exact && int(r.length) == len(entry) {
			return r, true
		}
	}
	return rule{}, false
}

func checkCompiledEntries(t *testing.T, li *list, allow []string, deny map[string]string) {
	t.Helper()
	// Entries from one variable share a suggestion, so count the deny rules
	denyRules := 0
	for _, r := range li.rules {
		if r.kind == ruleDeny {
			denyRules++
		}
	}
	if denyRules != len(deny) {
		t.Errorf("expected %d deny entries - got %d deny rules", len(deny), denyRules)
	}
	for _, a := range allow {
		if _, ok := hasRule(li, a, ruleAllow); !ok {
			t.Errorf("expected %q to be an allow entry: rules on its node %v", a, keyRules(li, strings.TrimSuffix(a, "$")))
		}
	}
	for d, sug := range deny {
		r, ok := hasRule(li, d, ruleDeny)
		if !ok {
			t.Errorf("expected %q to be a deny entry: rules on its node %v", d, keyRules(li, strings.TrimSuffix(d, "$")))
			continue
		}
		if r.sug < 0 || int(r.sug) >= len(li.suggestions) || li.suggestions[r.sug] != sug {
			t.Errorf("expected %q to suggest %q: suggestion index %d of %v", d, sug, r.sug, li.suggestions)
		}
	}
}

func testSettingsCompile(s *settingsCompileScenario) func(*testing.T) {
	return func(t *testing.T) {
		act, err := s.settings.compile()
		if s.expErr != nil {
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), s.expErr.Error()) {
				t.Errorf("error does not contain expected string: Exp %s, Act %s", s.expErr, err)
			}
			return
		}
		if err != nil {
			t.Fatal("not expecting an error")
		}
		diff := cmp.Diff(s.exp, act, cmp.AllowUnexported(list{}), cmp.Comparer(cmpGlobPattern), ignoreCompiledEntries)
		if diff != "" {
			t.Errorf("compiled settings is not what was expected\n%s", diff)
		}
		for _, li := range act {
			checkCompiledEntries(t, li, s.allow[li.name], nil)
		}
	}
}

type expanderTest struct{}

func (*expanderTest) Expand() ([]string, error) {
	return []string{"FIND ME", "FIND ME TOO"}, nil
}

func init() {
	// Only doing this so I have a controlled list of expansions for packages
	utils.PackageExpandable["$gostd"] = &expanderTest{}
}

func TestListCompile(t *testing.T) {
	for _, s := range listCompileScenarios {
		t.Run(s.name, testListCompile(s))
	}
}

func TestLinterSettingsCompile(t *testing.T) {
	for _, s := range settingsCompileScenarios {
		t.Run(s.name, testSettingsCompile(s))
	}
}

type expanderFailTest struct{}

func (*expanderFailTest) Expand() ([]string, error) {
	return nil, errors.New("expected error")
}

type listCompileVariablesScenario struct {
	name  string
	list  func() *List      // fresh each run, map order changes between runs
	rules []ruleSugExpected // in sorted order; keys are only comments
}

type ruleSugExpected struct {
	kind   ruleKind
	length int32
	sug    string
}

var listCompileVariablesScenarios = []*listCompileVariablesScenario{
	{
		// $gostd expands to "FIND ME" and "FIND ME TOO" in these tests
		name: "explicit deny keeps its suggestion",
		list: func() *List {
			return &List{Deny: map[string]string{"$gostd": "no std", "FIND ME": "explicit"}}
		},
		rules: []ruleSugExpected{
			{kind: ruleDeny, length: 7, sug: "explicit"}, // FIND ME
			{kind: ruleDeny, length: 11, sug: "no std"},  // FIND ME TOO
		},
	},
	{
		name: "explicit allow next to a variable is one rule",
		list: func() *List {
			return &List{Allow: []string{"FIND ME", "$gostd"}}
		},
		rules: []ruleSugExpected{
			{kind: ruleAllow, length: 7},  // FIND ME
			{kind: ruleAllow, length: 11}, // FIND ME TOO
		},
	},
	{
		name: "same entry in allow and deny is two rules",
		list: func() *List {
			return &List{Allow: []string{"$gostd"}, Deny: map[string]string{"FIND ME": "explicit"}}
		},
		rules: []ruleSugExpected{
			{kind: ruleAllow, length: 7},                 // FIND ME
			{kind: ruleDeny, length: 7, sug: "explicit"}, // FIND ME
			{kind: ruleAllow, length: 11},                // FIND ME TOO
		},
	},
}

func testListCompileVariables(s *listCompileVariablesScenario) func(*testing.T) {
	return func(t *testing.T) {
		for i := range 50 {
			li, err := s.list().compile()
			if err != nil {
				t.Fatalf("not expecting an error: %v", err)
			}
			act := make([]ruleSugExpected, 0, len(li.rules))
			for _, r := range li.rules {
				rs := ruleSugExpected{kind: r.kind, length: r.length}
				if r.kind == ruleDeny {
					rs.sug = li.suggestions[r.sug]
				}
				act = append(act, rs)
			}
			if diff := cmp.Diff(s.rules, act, cmp.AllowUnexported(ruleSugExpected{})); diff != "" {
				t.Fatalf("compile %d: rules don't match\n%s", i, diff)
			}
		}
	}
}

func TestListCompileVariables(t *testing.T) {
	for _, s := range listCompileVariablesScenarios {
		t.Run(s.name, testListCompileVariables(s))
	}
}

// TestListCompileVariableErrors checks every variable that fails to expand is
// reported, not just the first.
func TestListCompileVariableErrors(t *testing.T) {
	utils.PackageExpandable["$fail1"] = &expanderFailTest{}
	utils.PackageExpandable["$fail2"] = &expanderFailTest{}
	t.Cleanup(func() {
		delete(utils.PackageExpandable, "$fail1")
		delete(utils.PackageExpandable, "$fail2")
	})
	_, err := (&List{
		Allow: []string{"$fail1", "os"},
		Deny:  map[string]string{"$fail2": "nope"},
	}).compile()
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, v := range []string{"$fail1", "$fail2"} {
		if !strings.Contains(err.Error(), v) {
			t.Errorf("error should name %s: %v", v, err)
		}
	}
}

// TestListCompileLeavesSettings checks compile only reads the List it's
// given. NewUncompiledAnalyzer compiles once per package until Compile() is
// called, and packages run in parallel on the same *List; run with -race.
func TestListCompileLeavesSettings(t *testing.T) {
	newList := func() *List {
		return &List{
			ListMode: "Strict",
			Files:    []string{"$all", "!$test"},
			Allow:    []string{"$gostd", "github.com/org/"},
			Deny:     map[string]string{"$gostd": "no std", "FIND ME": "explicit", "github.com/org/legacy/": "use v2"},
		}
	}
	l := newList()
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := l.compile(); err != nil {
				t.Errorf("not expecting an error: %v", err)
			}
		}()
	}
	wg.Wait()
	if diff := cmp.Diff(newList(), l); diff != "" {
		t.Errorf("compile changed the List it was given\n%s", diff)
	}
}

// TestListCompileRuleOrder checks the rules come out sorted by key, then allow
// before deny, then prefix before exact. Each tree node owns a span of
// rules with the same key, so this order must not depend on map iteration.
// Compiled a few times, as one compile can get the right order by luck.
//
// Rules don't keep their key, so the key is only a comment here. Each deny
// rule has its own suggestion and each allow rule a distinct length or
// exactness next to its neighbours, so a wrong order still shows up.
func TestListCompileRuleOrder(t *testing.T) {
	type ruleSug struct {
		kind   ruleKind
		exact  bool
		length int32
		sug    string
	}
	exp := []ruleSug{
		{kind: ruleAllow, length: 1},                             // a
		{kind: ruleDeny, length: 1, sug: "a prefix"},             // a
		{kind: ruleDeny, exact: true, length: 2, sug: "a exact"}, // a$
		{kind: ruleAllow, exact: true, length: 4},                // a/b$
		{kind: ruleDeny, length: 3, sug: "a/b prefix"},           // a/b
		{kind: ruleAllow, length: 1},                             // b
		{kind: ruleAllow, exact: true, length: 2},                // b$
		{kind: ruleDeny, exact: true, length: 2, sug: "c exact"}, // c$
	}
	for i := range 20 {
		li, err := (&List{
			Allow: []string{"b$", "a/b$", "b", "a"},
			Deny: map[string]string{
				"c$":  "c exact",
				"a$":  "a exact",
				"a/b": "a/b prefix",
				"a":   "a prefix",
			},
		}).compile()
		if err != nil {
			t.Fatalf("not expecting an error: %v", err)
		}
		act := make([]ruleSug, 0, len(li.rules))
		for _, r := range li.rules {
			rs := ruleSug{kind: r.kind, exact: r.exact, length: r.length}
			if r.kind == ruleDeny {
				rs.sug = li.suggestions[r.sug]
			} else if r.sug != -1 {
				t.Errorf("compile %d: allow rule has suggestion index %d", i, r.sug)
			}
			act = append(act, rs)
		}
		if diff := cmp.Diff(exp, act, cmp.AllowUnexported(ruleSug{})); diff != "" {
			t.Fatalf("compile %d: rules not in the expected order\n%s", i, diff)
		}
	}
}

var (
	globList = []*glob.Pattern{
		glob.MustCompile("some/*/a", '/'),
		glob.MustCompile("some/**/a", '/'),
	}
)

func testStrInGlobList(str string, expect bool) func(t *testing.T) {
	return func(t *testing.T) {
		if strInGlobList(str, globList) != expect {
			t.Fail()
		}
	}
}

func TestStrInGlobList(t *testing.T) {
	t.Run("match_first", testStrInGlobList("some/foo/a", true))
	t.Run("match", testStrInGlobList("some/foo/bar/a", true))
	t.Run("no_match", testStrInGlobList("some/foo/b", false))
}

type listFileMatchScenario struct {
	name  string
	setup *list
	tests []*listFileMatchScenarioInner
}
type listFileMatchScenarioInner struct {
	name     string
	input    string
	expected bool
}

var listFileMatchScenarios = []*listFileMatchScenario{
	{
		name:  "Empty lists matches everything",
		setup: &list{},
		tests: []*listFileMatchScenarioInner{
			{
				name:     "go files",
				input:    "foo/somefile.go",
				expected: true,
			},
			{
				name:     "test go files",
				input:    "foo/somefile_test.go",
				expected: true,
			},
			{
				name:     "not a go file",
				input:    "foo/somefile_test.file",
				expected: true,
			},
		},
	},
	{
		name: "Empty allow matches anything not in deny",
		setup: &list{
			negFiles: []*glob.Pattern{
				glob.MustCompile("**/*_test.go", '/'),
			},
		},
		tests: []*listFileMatchScenarioInner{
			{
				name:     "not in deny",
				input:    "foo/somefile.go",
				expected: true,
			},
			{
				name:     "in deny",
				input:    "foo/somefile_test.go",
				expected: false,
			},
			{
				name:     "not a go file",
				input:    "foo/somefile_test.file",
				expected: true,
			},
		},
	},
	{
		name: "Empty deny only matches what is in allowed",
		setup: &list{
			files: []*glob.Pattern{
				glob.MustCompile("**/*_test.go", '/'),
			},
		},
		tests: []*listFileMatchScenarioInner{
			{
				name:     "not in allow",
				input:    "foo/somefile.go",
				expected: false,
			},
			{
				name:     "in allow",
				input:    "foo/somefile_test.go",
				expected: true,
			},
			{
				name:     "not a go file",
				input:    "foo/somefile_test.file",
				expected: false,
			},
		},
	},
	{
		name: "Both only allows what is in allow and not in deny",
		setup: &list{
			files: []*glob.Pattern{
				glob.MustCompile("**/*.go", '/'),
			},
			negFiles: []*glob.Pattern{
				glob.MustCompile("**/*_test.go", '/'),
			},
		},
		tests: []*listFileMatchScenarioInner{
			{
				name:     "in allow but not deny",
				input:    "foo/somefile.go",
				expected: true,
			},
			{
				name:     "in allow and in deny",
				input:    "foo/somefile_test.go",
				expected: false,
			},
			{
				name:     "in neither allow or deny",
				input:    "foo/somefile_test.file",
				expected: false,
			},
		},
	},
	{
		name: "Double star between slashes needs at least one folder",
		setup: &list{
			files: []*glob.Pattern{
				glob.MustCompile("**/internal/**/*.go", '/'),
			},
		},
		tests: []*listFileMatchScenarioInner{
			{
				name:     "zero folders",
				input:    "/home/user/project/internal/a.go",
				expected: false,
			},
			{
				name:     "one folder",
				input:    "/home/user/project/internal/foo/a.go",
				expected: true,
			},
			{
				name:     "many folders",
				input:    "/home/user/project/internal/foo/bar/a.go",
				expected: true,
			},
		},
	},
	{
		name: "Optional double star matches zero or more folders",
		setup: &list{
			files: []*glob.Pattern{
				glob.MustCompile("**/internal/{**/,}*.go", '/'),
			},
		},
		tests: []*listFileMatchScenarioInner{
			{
				name:     "zero folders",
				input:    "/home/user/project/internal/a.go",
				expected: true,
			},
			{
				name:     "one folder",
				input:    "/home/user/project/internal/foo/a.go",
				expected: true,
			},
			{
				name:     "many folders",
				input:    "/home/user/project/internal/foo/bar/a.go",
				expected: true,
			},
			{
				name:     "outside the folder",
				input:    "/home/user/project/external/a.go",
				expected: false,
			},
		},
	},
	{
		name: "Optional double star with the slash outside the braces still needs a folder",
		setup: &list{
			files: []*glob.Pattern{
				glob.MustCompile("**/internal/{**,}/*.go", '/'),
			},
		},
		tests: []*listFileMatchScenarioInner{
			{
				name:     "zero folders",
				input:    "/home/user/project/internal/a.go",
				expected: false,
			},
			{
				name:     "one folder",
				input:    "/home/user/project/internal/foo/a.go",
				expected: true,
			},
		},
	},
	{
		name: "Optional double star in a negated glob",
		setup: &list{
			negFiles: []*glob.Pattern{
				glob.MustCompile("**/internal/{**/,}*.go", '/'),
			},
		},
		tests: []*listFileMatchScenarioInner{
			{
				name:     "zero folders",
				input:    "/home/user/project/internal/a.go",
				expected: false,
			},
			{
				name:     "one folder",
				input:    "/home/user/project/internal/foo/a.go",
				expected: false,
			},
			{
				name:     "outside the folder",
				input:    "/home/user/project/external/a.go",
				expected: true,
			},
		},
	},
}

func TestListFileMatch(t *testing.T) {
	for _, s := range listFileMatchScenarios {
		t.Run(s.name, func(ts *testing.T) {
			for _, sc := range s.tests {
				ts.Run(sc.name, func(tst *testing.T) {
					act := s.setup.fileMatch(sc.input)
					if act != sc.expected {
						tst.Error("Did not return expected result")
					}
				})
			}
		})
	}
}

type listImportAllowedScenario struct {
	name  string
	setup *List
	tests []*listImportAllowedScenarioInner
}

type listImportAllowedScenarioInner struct {
	name       string
	input      string
	allowed    bool
	suggestion string
}

var listImportAllowedScenarios = []*listImportAllowedScenario{
	{
		name: "Empty allow in Original matches anything not in deny",
		setup: &List{
			Deny: map[string]string{
				"some/pkg/a":  "because I said so",
				"some/pkg/b$": "please use newer version",
			},
		},
		tests: []*listImportAllowedScenarioInner{
			{
				name:       "in deny",
				input:      "some/pkg/a/bar",
				allowed:    false,
				suggestion: "because I said so",
			},
			{
				name:    "not in deny suffixed by exact match",
				input:   "some/pkg/b/foo/bar",
				allowed: true,
			},
			{
				name:       "in deny exact match",
				input:      "some/pkg/b",
				allowed:    false,
				suggestion: "please use newer version",
			},
		},
	},
	{
		name: "Empty deny in Original only matches what is in allow",
		setup: &List{
			Allow: []string{"some/pkg/a", "some/pkg/b$"},
		},
		tests: []*listImportAllowedScenarioInner{
			{
				name:    "in allow",
				input:   "some/pkg/a/bar",
				allowed: true,
			},
			{
				name:    "not in allow suffixed by exact match",
				input:   "some/pkg/b/foo/bar",
				allowed: false,
			},
			{
				name:    "in allow exact match",
				input:   "some/pkg/b",
				allowed: true,
			},
		},
	},
	{
		name: "Both in Original mode allows what is in allow and not in deny",
		setup: &List{
			ListMode: "Original",
			Allow:    []string{"some/pkg/a/foo", "some/pkg/b", "some/pkg/c"},
			Deny: map[string]string{
				"some/pkg/a":     "because I said so",
				"some/pkg/b/foo": "really don't use",
				"some/pkg/d":     "common",
			},
		},
		tests: []*listImportAllowedScenarioInner{
			{
				name:    "in allow but not in deny",
				input:   "some/pkg/c/alpha",
				allowed: true,
			},
			{
				name:       "subpackage allowed but root denied",
				input:      "some/pkg/a/foo/bar",
				allowed:    false,
				suggestion: "because I said so",
			},
			{
				name:       "subpackage not in allowed but root denied",
				input:      "some/pkg/a/baz",
				allowed:    false,
				suggestion: "because I said so",
			},
			{
				name:       "subpackage denied but root allowed",
				input:      "some/pkg/b/foo/bar",
				allowed:    false,
				suggestion: "really don't use",
			},
			{
				name:    "subpackage not denied but root allowed",
				input:   "some/pkg/b/baz",
				allowed: true,
			},
			{
				name:       "in deny but not in allow",
				input:      "some/pkg/d/baz",
				allowed:    false,
				suggestion: "common",
			},
			{
				name:    "not in allow nor in deny",
				input:   "some/pkg/e/alpha",
				allowed: false,
			},
			{
				name:    "check for out of bounds",
				input:   "aaa/pkg/e/alpha",
				allowed: false,
			},
		},
	},
	{
		name: "Empty allow in Strict matches nothing",
		setup: &List{
			ListMode: "Strict",
			Deny: map[string]string{
				"some/pkg/a":  "because I said so",
				"some/pkg/b$": "please use newer version",
			},
		},
		tests: []*listImportAllowedScenarioInner{
			{
				name:       "in deny",
				input:      "some/pkg/a/bar",
				allowed:    false,
				suggestion: "because I said so",
			},
			{
				name:    "not in deny suffixed by exact match",
				input:   "some/pkg/b/foo/bar",
				allowed: false,
			},
			{
				name:       "in deny exact match",
				input:      "some/pkg/b",
				allowed:    false,
				suggestion: "please use newer version",
			},
		},
	},
	{
		name: "Empty deny in Strict only matches what is in allow",
		setup: &List{
			ListMode: "Strict",
			Allow:    []string{"some/pkg/a", "some/pkg/b$"},
		},
		tests: []*listImportAllowedScenarioInner{
			{
				name:    "in allow",
				input:   "some/pkg/a/bar",
				allowed: true,
			},
			{
				name:    "not in allow suffixed by exact match",
				input:   "some/pkg/b/foo/bar",
				allowed: false,
			},
			{
				name:    "in allow exact match",
				input:   "some/pkg/b",
				allowed: true,
			},
		},
	},
	{
		name: "Both in Strict mode allows what is in allow and not in deny",
		setup: &List{
			ListMode: "Strict",
			Allow:    []string{"some/pkg/a/foo", "some/pkg/b", "some/pkg/c"},
			Deny: map[string]string{
				"some/pkg/a":     "because I said so",
				"some/pkg/b/foo": "really don't use",
				"some/pkg/d":     "common",
			},
		},
		tests: []*listImportAllowedScenarioInner{
			{
				name:    "in allow but not in deny",
				input:   "some/pkg/c/alpha",
				allowed: true,
			},
			{
				name:    "subpackage allowed but root denied",
				input:   "some/pkg/a/foo/bar",
				allowed: true,
			},
			{
				name:       "subpackage not in allowed but root denied",
				input:      "some/pkg/a/baz",
				allowed:    false,
				suggestion: "because I said so",
			},
			{
				name:       "subpackage denied but root allowed",
				input:      "some/pkg/b/foo/bar",
				allowed:    false,
				suggestion: "really don't use",
			},
			{
				name:    "subpackage not denied but root allowed",
				input:   "some/pkg/b/baz",
				allowed: true,
			},
			{
				name:       "in deny but not in allow",
				input:      "some/pkg/d/baz",
				allowed:    false,
				suggestion: "common",
			},
			{
				name:    "not in allow nor in deny",
				input:   "some/pkg/e/alpha",
				allowed: false,
			},
			{
				name:    "check for out of bounds",
				input:   "aaa/pkg/e/alpha",
				allowed: false,
			},
		},
	},
	{
		name: "Empty allow in Lax matches anything not in deny",
		setup: &List{
			ListMode: "Lax",
			Deny: map[string]string{
				"some/pkg/a":  "because I said so",
				"some/pkg/b$": "please use newer version",
			},
		},
		tests: []*listImportAllowedScenarioInner{
			{
				name:       "in deny",
				input:      "some/pkg/a/bar",
				allowed:    false,
				suggestion: "because I said so",
			},
			{
				name:    "not in deny suffixed by exact match",
				input:   "some/pkg/b/foo/bar",
				allowed: true,
			},
			{
				name:       "in deny exact match",
				input:      "some/pkg/b",
				allowed:    false,
				suggestion: "please use newer version",
			},
		},
	},
	{
		name: "Empty deny in Lax matches everything",
		setup: &List{
			ListMode: "Lax",
			Allow:    []string{"some/pkg/a", "some/pkg/b$"},
		},
		tests: []*listImportAllowedScenarioInner{
			{
				name:    "in allow",
				input:   "some/pkg/a/bar",
				allowed: true,
			},
			{
				name:    "not in allow suffixed by exact match",
				input:   "some/pkg/b/foo/bar",
				allowed: true,
			},
			{
				name:    "in allow exact match",
				input:   "some/pkg/b",
				allowed: true,
			},
		},
	},
	{
		name: "Both in Lax mode allows what is in allow and not in deny",
		setup: &List{
			ListMode: "Lax",
			Allow:    []string{"some/pkg/a/foo", "some/pkg/b", "some/pkg/c"},
			Deny: map[string]string{
				"some/pkg/a":     "because I said so",
				"some/pkg/b/foo": "really don't use",
				"some/pkg/d":     "common",
			},
		},
		tests: []*listImportAllowedScenarioInner{
			{
				name:    "in allow but not in deny",
				input:   "some/pkg/c/alpha",
				allowed: true,
			},
			{
				name:    "subpackage allowed but root denied",
				input:   "some/pkg/a/foo/bar",
				allowed: true,
			},
			{
				name:       "subpackage not in allowed but root denied",
				input:      "some/pkg/a/baz",
				allowed:    false,
				suggestion: "because I said so",
			},
			{
				name:       "subpackage denied but root allowed",
				input:      "some/pkg/b/foo/bar",
				allowed:    false,
				suggestion: "really don't use",
			},
			{
				name:    "subpackage not denied but root allowed",
				input:   "some/pkg/b/baz",
				allowed: true,
			},
			{
				name:       "in deny but not in allow",
				input:      "some/pkg/d/baz",
				allowed:    false,
				suggestion: "common",
			},
			{
				name:    "not in allow nor in deny",
				input:   "some/pkg/e/alpha",
				allowed: true,
			},
			{
				name:    "check for out of bounds",
				input:   "aaa/pkg/e/alpha",
				allowed: true,
			},
		},
	},
	{
		name: "Nested deny in Lax mode denies every subpackage of the root",
		setup: &List{
			ListMode: "Lax",
			Deny: map[string]string{
				"some/pkg/a":      "root a",
				"some/pkg/a/foo":  "nested a",
				"some/pkg/b/":     "root b",
				"some/pkg/b/foo$": "exact b",
			},
		},
		tests: []*listImportAllowedScenarioInner{
			{
				name:       "nested subpackage uses nested suggestion",
				input:      "some/pkg/a/foo/bar",
				allowed:    false,
				suggestion: "nested a",
			},
			{
				name:       "sibling sorted before nested",
				input:      "some/pkg/a/bar",
				allowed:    false,
				suggestion: "root a",
			},
			{
				name:       "sibling sorted after nested",
				input:      "some/pkg/a/zed",
				allowed:    false,
				suggestion: "root a",
			},
			{
				name:       "nested exact match uses exact suggestion",
				input:      "some/pkg/b/foo",
				allowed:    false,
				suggestion: "exact b",
			},
			{
				name:       "sibling sorted before nested exact",
				input:      "some/pkg/b/bar",
				allowed:    false,
				suggestion: "root b",
			},
			{
				name:       "sibling sorted after nested exact",
				input:      "some/pkg/b/zed",
				allowed:    false,
				suggestion: "root b",
			},
			{
				name:       "subpackage of nested exact",
				input:      "some/pkg/b/foo/bar",
				allowed:    false,
				suggestion: "root b",
			},
		},
	},
	{
		name: "Nested allow in Strict mode allows every subpackage of the root",
		setup: &List{
			ListMode: "Strict",
			Allow:    []string{"some/pkg/a/", "some/pkg/a/foo/"},
			Deny: map[string]string{
				"some/pkg/a/bar/": "not bar",
			},
		},
		tests: []*listImportAllowedScenarioInner{
			{
				name:    "nested subpackage",
				input:   "some/pkg/a/foo/x",
				allowed: true,
			},
			{
				name:    "sibling sorted before nested",
				input:   "some/pkg/a/baz",
				allowed: true,
			},
			{
				name:    "sibling sorted after nested",
				input:   "some/pkg/a/zed",
				allowed: true,
			},
			{
				name:       "denied subpackage of allowed root",
				input:      "some/pkg/a/bar/x",
				allowed:    false,
				suggestion: "not bar",
			},
		},
	},
	{
		// Matching rules from the README not covered by the scenarios above
		name: "Settled matching rules in Strict mode",
		setup: &List{
			ListMode: "Strict",
			Allow:    []string{"github.com/bob", "github.com/team/", "same/len"},
			Deny:     map[string]string{"same/len": "tie goes to deny"},
		},
		tests: []*listImportAllowedScenarioInner{
			{name: "raw prefix matches a longer name (#2)", input: "github.com/bob2", allowed: true},
			{name: "trailing slash skips the root package (#113)", input: "github.com/team", allowed: false},
			{name: "equal length allow and deny is denied", input: "same/len", allowed: false, suggestion: "tie goes to deny"},
		},
	},
	{
		name: "Exact match wins a tie in Strict mode",
		setup: &List{
			ListMode: "Strict",
			Allow:    []string{"some/pkg/e$"},
			Deny: map[string]string{
				"some/pkg/e": "use some/pkg/f",
			},
		},
		tests: []*listImportAllowedScenarioInner{
			{
				name:    "exact allow beats same prefix deny",
				input:   "some/pkg/e",
				allowed: true,
			},
			{
				name:       "subpackage only matches deny",
				input:      "some/pkg/e/x",
				allowed:    false,
				suggestion: "use some/pkg/f",
			},
		},
	},
}

func TestListImportAllowed(t *testing.T) {
	for _, s := range listImportAllowedScenarios {
		t.Run(s.name, func(ts *testing.T) {
			li, err := s.setup.compile()
			if err != nil {
				ts.Fatalf("could not compile list: %s", err)
			}
			for _, sc := range s.tests {
				ts.Run(sc.name, func(tst *testing.T) {
					act, sugg := li.importAllowed(sc.input)
					if act != sc.allowed {
						tst.Error("Did not return expected result")
					}
					if sugg != sc.suggestion {
						tst.Errorf("Suggestion didn't match expected: Exp %s: Act: %s", sc.suggestion, sugg)
					}
				})
			}
		})
	}
}

type linterSettingsWhichListsScenario struct {
	name     string
	input    string
	expected []string
}

var linterSettingsWhichListsSetup = linterSettings{
	{
		name: "Main",
		files: []*glob.Pattern{
			glob.MustCompile("**/*.go", '/'),
		},
	},
	{
		name: "Test",
		files: []*glob.Pattern{
			glob.MustCompile("**/*_test.go", '/'),
		},
	},
}

var linterSettingsWhichListsScenarios = []*linterSettingsWhichListsScenario{
	{
		name:     "return none",
		input:    "some/randome.file",
		expected: []string{},
	},
	{
		name:     "return single",
		input:    "some/random.go",
		expected: []string{"Main"},
	},
	{
		name:     "return multiple",
		input:    "some/random_test.go",
		expected: []string{"Main", "Test"},
	},
}

func TestLinterSettingsWhichLists(t *testing.T) {
	for _, s := range linterSettingsWhichListsScenarios {
		t.Run(s.name, func(ts *testing.T) {
			act := linterSettingsWhichListsSetup.whichLists(s.input)
			if len(act) != len(s.expected) {
				ts.Fatal("List is not of expected length")
			}
			for i, a := range act {
				if a.name != s.expected[i] {
					t.Errorf("List at index %d is not named %s but instead is %s", i, s.expected[i], a.name)
				}
			}
		})
	}
}
