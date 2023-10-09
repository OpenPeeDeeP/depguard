package depguard

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/OpenPeeDeeP/depguard/v2/internal/utils"
	"github.com/gobwas/glob"
	"github.com/google/go-cmp/cmp"
)

type listCompileScenario struct {
	name   string
	list   *List
	exp    *list
	expErr error
}

type settingsCompileScenario struct {
	name     string
	settings LinterSettings
	exp      linterSettings
	expErr   error
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
				allow:       []string{"os"},
				deny:        []string{"reflect"},
				suggestions: []string{"Don't use Reflect"},
			},
		},
		{
			name: "Expanded Files",
			list: &List{
				Files: []string{"$all"},
				Allow: []string{"os"},
			},
			exp: &list{
				files: []glob.Glob{
					glob.MustCompile("**/*.go", '/'),
				},
				allow: []string{"os"},
			},
		},
		{
			name: "Expanded Negate Files",
			list: &List{
				Files: []string{"!$test"},
				Allow: []string{"os"},
			},
			exp: &list{
				negFiles: []glob.Glob{
					glob.MustCompile("**/*_test.go", '/'),
				},
				allow: []string{"os"},
			},
		},
		{
			name: "Normal and Negatable Files",
			list: &List{
				Files: []string{"**/foo.go", "!**/bar.go"},
				Allow: []string{"os"},
			},
			exp: &list{
				files: []glob.Glob{
					glob.MustCompile("**/foo.go", '/'),
				},
				negFiles: []glob.Glob{
					glob.MustCompile("**/bar.go", '/'),
				},
				allow: []string{"os"},
			},
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
				allow: []string{"FIND ME", "FIND ME TOO"},
			},
		},
		{
			name: "Expanded Deny",
			list: &List{
				Deny: map[string]string{"$gostd": "Don't use standard"},
			},
			exp: &list{
				deny:        []string{"FIND ME", "FIND ME TOO"},
				suggestions: []string{"Don't use standard", "Don't use standard"},
			},
		},
		{
			name: "Only Deny",
			list: &List{
				Deny: map[string]string{
					"reflect": "Don't use Reflect",
				},
			},
			exp: &list{
				deny:        []string{"reflect"},
				suggestions: []string{"Don't use Reflect"},
			},
		},
		{
			name: "Only Allow",
			list: &List{
				Allow: []string{"os"},
			},
			exp: &list{
				allow: []string{"os"},
			},
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
				files: []glob.Glob{
					glob.MustCompile("**/*.go", '/'),
				},
				negFiles: []glob.Glob{
					glob.MustCompile("**/*_test.go", '/'),
				},
				allow:       []string{"os"},
				deny:        []string{"reflect"},
				suggestions: []string{"Don't use Reflect"},
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
				listMode:    listModeOriginal,
				allow:       []string{"os"},
				deny:        []string{"reflect"},
				suggestions: []string{"Don't use Reflect"},
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
				listMode:    listModeOriginal,
				allow:       []string{"os"},
				deny:        []string{"reflect"},
				suggestions: []string{"Don't use Reflect"},
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
				listMode:    listModeStrict,
				allow:       []string{"os"},
				deny:        []string{"reflect"},
				suggestions: []string{"Don't use Reflect"},
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
				listMode:    listModeLax,
				allow:       []string{"os"},
				deny:        []string{"reflect"},
				suggestions: []string{"Don't use Reflect"},
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
	}
	settingsCompileScenarios = []*settingsCompileScenario{
		{
			name: "Zero State",
			exp: []*list{
				{
					name: "Main",
					files: []glob.Glob{
						glob.MustCompile("**/*.go", '/'),
					},
					allow: []string{"FIND ME", "FIND ME TOO"},
				},
			},
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
					files: []glob.Glob{
						glob.MustCompile("**/*.go", '/'),
					},
					allow: []string{"os"},
				},
				{
					name: "Test",
					files: []glob.Glob{
						glob.MustCompile("**/*_test.go", '/'),
					},
					allow: []string{"os"},
				},
			},
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
		diff := cmp.Diff(s.exp, act, cmp.AllowUnexported(list{}))
		if diff != "" {
			t.Errorf("compiled list is not what was expected\n%s", diff)
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
		diff := cmp.Diff(s.exp, act, cmp.AllowUnexported(list{}))
		if diff != "" {
			t.Errorf("compiled settings is not what was expected\n%s", diff)
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
	prefixList = []string{
		"some/package/a",
		"some/package/b",
		"some/package/c/",
		"some/package/d$",
		"some/pkg/c",
		"some/pkg/d",
		"some/pkg/e",
	}

	globList = []glob.Glob{
		glob.MustCompile("some/*/a", '/'),
		glob.MustCompile("some/**/a", '/'),
	}
)

func testStrInPrefixList(str string, expect bool, expectedIdx int) func(t *testing.T) {
	return func(t *testing.T) {
		act, idx := strInPrefixList(str, prefixList)
		if act != expect {
			t.Errorf("string prefix mismatch: expected %s - got %s", strconv.FormatBool(expect), strconv.FormatBool(act))
		}
		if idx != expectedIdx {
			t.Errorf("string prefix index: expected %d - got %d", expectedIdx, idx)
		}
	}
}

func TestStrInPrefixList(t *testing.T) {
	sort.Strings(prefixList)
	t.Run("full_match_start", testStrInPrefixList("some/package/a", true, 0))
	t.Run("full_match", testStrInPrefixList("some/package/b", true, 1))
	t.Run("full_match_end", testStrInPrefixList("some/pkg/e", true, 6))
	t.Run("no_match_end", testStrInPrefixList("zome/pkg/e", false, 6))
	t.Run("no_match_start", testStrInPrefixList("aome/pkg/e", false, -1))
	t.Run("match_start", testStrInPrefixList("some/package/a/files", true, 0))
	t.Run("match_middle", testStrInPrefixList("some/pkg/c/files", true, 4))
	t.Run("match_end", testStrInPrefixList("some/pkg/e/files", true, 6))
	t.Run("no_match_trailing", testStrInPrefixList("some/package/c", false, 1))
	t.Run("match_exact", testStrInPrefixList("some/package/d", true, 3))
	t.Run("no_prefix_match_exact", testStrInPrefixList("some/package/d/something", false, 3))
}

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
			negFiles: []glob.Glob{
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
			files: []glob.Glob{
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
			files: []glob.Glob{
				glob.MustCompile("**/*.go", '/'),
			},
			negFiles: []glob.Glob{
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
	setup *list
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
		setup: &list{
			deny:        []string{"some/pkg/a", "some/pkg/b$"},
			suggestions: []string{"because I said so", "please use newer version"},
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
		setup: &list{
			allow: []string{"some/pkg/a", "some/pkg/b$"},
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
		setup: &list{
			listMode:    listModeOriginal,
			allow:       []string{"some/pkg/a/foo", "some/pkg/b", "some/pkg/c"},
			deny:        []string{"some/pkg/a", "some/pkg/b/foo", "some/pkg/d"},
			suggestions: []string{"because I said so", "really don't use", "common"},
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
		setup: &list{
			listMode:    listModeStrict,
			deny:        []string{"some/pkg/a", "some/pkg/b$"},
			suggestions: []string{"because I said so", "please use newer version"},
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
		setup: &list{
			listMode: listModeStrict,
			allow:    []string{"some/pkg/a", "some/pkg/b$"},
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
		setup: &list{
			listMode:    listModeStrict,
			allow:       []string{"some/pkg/a/foo", "some/pkg/b", "some/pkg/c"},
			deny:        []string{"some/pkg/a", "some/pkg/b/foo", "some/pkg/d"},
			suggestions: []string{"because I said so", "really don't use", "common"},
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
		setup: &list{
			listMode:    listModeLax,
			deny:        []string{"some/pkg/a", "some/pkg/b$"},
			suggestions: []string{"because I said so", "please use newer version"},
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
		setup: &list{
			listMode: listModeLax,
			allow:    []string{"some/pkg/a", "some/pkg/b$"},
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
		setup: &list{
			listMode:    listModeLax,
			allow:       []string{"some/pkg/a/foo", "some/pkg/b", "some/pkg/c"},
			deny:        []string{"some/pkg/a", "some/pkg/b/foo", "some/pkg/d"},
			suggestions: []string{"because I said so", "really don't use", "common"},
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
}

func TestListImportAllowed(t *testing.T) {
	for _, s := range listImportAllowedScenarios {
		t.Run(s.name, func(ts *testing.T) {
			for _, sc := range s.tests {
				ts.Run(sc.name, func(tst *testing.T) {
					act, sugg := s.setup.importAllowed(sc.input)
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
		files: []glob.Glob{
			glob.MustCompile("**/*.go", '/'),
		},
	},
	{
		name: "Test",
		files: []glob.Glob{
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
