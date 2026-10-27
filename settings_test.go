package depguard

import (
	"errors"
	"strings"
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

// The tree and suggestions are checked by behavior in checkCompiledEntries, as
// the suggestion indexes depend on map iteration order.
var ignoreCompiledEntries = cmpopts.IgnoreFields(list{}, "tree", "suggestions")

func checkCompiledEntries(t *testing.T, li *list, allow []string, deny map[string]string) {
	t.Helper()
	if len(li.suggestions) != len(deny) {
		t.Errorf("expected %d deny entries - got %d suggestions %v", len(deny), len(li.suggestions), li.suggestions)
	}
	for _, a := range allow {
		allowLen, _, _ := li.tree.Search(strings.TrimSuffix(a, "$"))
		if allowLen != len(a) {
			t.Errorf("expected %q to be an allow entry: allow length %d", a, allowLen)
		}
	}
	for d, sug := range deny {
		_, denyLen, sugIdx := li.tree.Search(strings.TrimSuffix(d, "$"))
		if denyLen != len(d) {
			t.Errorf("expected %q to be a deny entry: deny length %d", d, denyLen)
			continue
		}
		if sugIdx < 0 || sugIdx >= len(li.suggestions) || li.suggestions[sugIdx] != sug {
			t.Errorf("expected %q to suggest %q: suggestion index %d of %v", d, sug, sugIdx, li.suggestions)
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
