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
				listMode:    lmMixed,
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
				listMode: lmAllow,
				allow:    []string{"os"},
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
				listMode: lmAllow,
				allow:    []string{"os"},
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
				listMode: lmAllow,
				allow:    []string{"os"},
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
				listMode: lmAllow,
				allow:    []string{"FIND ME", "FIND ME TOO"},
			},
		},
		{
			name: "Expanded Deny",
			list: &List{
				Deny: map[string]string{"$gostd": "Don't use standard"},
			},
			exp: &list{
				listMode:    lmDeny,
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
				listMode:    lmDeny,
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
				listMode: lmAllow,
				allow:    []string{"os"},
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
				listMode:    lmMixed,
				allow:       []string{"os"},
				deny:        []string{"reflect"},
				suggestions: []string{"Don't use Reflect"},
			},
		},
	}
	settingsCompileScenarios = []*settingsCompileScenario{
		{
			name: "Zero State",
			exp: []*list{
				{
					name:     "Main",
					listMode: lmAllow,
					allow:    []string{"FIND ME", "FIND ME TOO"},
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
