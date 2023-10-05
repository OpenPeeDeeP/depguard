package depguard

import (
	"errors"
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

func toGlobList(patterns ...string) []glob.Glob {
	ret := make([]glob.Glob, len(patterns))
	for idx, pattern := range patterns {
		glob, err := inputPatternToGlob(pattern)
		if err != nil {
			panic(err)
		}
		ret[idx] = glob
	}
	return ret
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
				Allow: []string{"os", "example.com/*/pkg"},
				Deny: map[string]string{
					"reflect":             "Don't use Reflect",
					"example.com/**/test": "Don't use test code",
				},
			},
			exp: &list{
				allow:       toGlobList("example.com/*/pkg", "os"),
				deny:        toGlobList("example.com/**/test", "reflect"),
				suggestions: []string{"Don't use test code", "Don't use Reflect"},
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
				allow: toGlobList("os"),
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
				allow: toGlobList("os"),
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
				allow: toGlobList("os"),
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
				allow: toGlobList("FIND ME", "FIND ME TOO"),
			},
		},
		{
			name: "Expanded Deny",
			list: &List{
				Deny: map[string]string{"$gostd": "Don't use standard"},
			},
			exp: &list{
				deny:        toGlobList("FIND ME", "FIND ME TOO"),
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
				deny:        toGlobList("reflect"),
				suggestions: []string{"Don't use Reflect"},
			},
		},
		{
			name: "Only Allow",
			list: &List{
				Allow: []string{"os"},
			},
			exp: &list{
				allow: toGlobList("os"),
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
				allow:       toGlobList("os"),
				deny:        toGlobList("reflect"),
				suggestions: []string{"Don't use Reflect"},
			},
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
					allow: toGlobList("FIND ME", "FIND ME TOO"),
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
					allow: toGlobList("os"),
				},
				{
					name: "Test",
					files: []glob.Glob{
						glob.MustCompile("**/*_test.go", '/'),
					},
					allow: toGlobList("os"),
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

func testStrInGlobList(str string, expect bool) func(t *testing.T) {
	return func(t *testing.T) {
		if found, _ := strInGlobList(str, globList); found != expect {
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
	expected   bool
	suggestion string
}

var listImportAllowedScenarios = []*listImportAllowedScenario{
	{
		name: "Empty allow matches anything not in deny",
		setup: &list{
			deny:        toGlobList("some/pkg/a", "some/pkg/b$"),
			suggestions: []string{"because I said so", "please use newer version"},
		},
		tests: []*listImportAllowedScenarioInner{
			{
				name:       "in deny",
				input:      "some/pkg/a/bar",
				expected:   false,
				suggestion: "because I said so",
			},
			{
				name:     "not in deny suffixed by exact match",
				input:    "some/pkg/b/foo/bar",
				expected: true,
			},
			{
				name:       "in deny exact match",
				input:      "some/pkg/b",
				expected:   false,
				suggestion: "please use newer version",
			},
		},
	},
	{
		name: "Empty deny only matches what is in allow",
		setup: &list{
			allow: toGlobList("some/pkg/a", "some/pkg/b$"),
		},
		tests: []*listImportAllowedScenarioInner{
			{
				name:     "in allow",
				input:    "some/pkg/a/bar",
				expected: true,
			},
			{
				name:     "not in allow suffixed by exact match",
				input:    "some/pkg/b/foo/bar",
				expected: false,
			},
			{
				name:     "in allow exact match",
				input:    "some/pkg/b",
				expected: true,
			},
		},
	},
	{
		name: "Both only allows what is in allow and not in deny",
		setup: &list{
			allow:       toGlobList("some/pkg/a"),
			deny:        toGlobList("some/pkg/a/foo"),
			suggestions: []string{"because I said so"},
		},
		tests: []*listImportAllowedScenarioInner{
			{
				name:     "in allow but not in deny",
				input:    "some/pkg/a/bar",
				expected: true,
			},
			{
				name:       "in allow and in deny",
				input:      "some/pkg/a/foo/bar",
				expected:   false,
				suggestion: "because I said so",
			},
			{
				name:     "not in allow nor in deny",
				input:    "some/pkg/b/foo/bar",
				expected: false,
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
					if act != sc.expected {
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
