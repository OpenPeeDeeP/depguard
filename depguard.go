package v2

import (
	"fmt"
	"go/ast"
	"go/build"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/gobwas/glob"
	"golang.org/x/tools/go/analysis"
)

// ListType states what kind of list is passed in.
type ListType int

const (
	// LTDenyList states the list given is a list of packages to deny. (default)
	LTDenyList ListType = iota
	// LTAllowList states the list given is a list of package to allow.
	LTAllowList
)

type negatableGlob struct {
	glob   glob.Glob
	negate bool
}

// TODO define if certain slices should AND results or OR results accordingly. Is it configurable?
// TODO Maybe having both prefix and glob is too much to consider the above... Any alternatives that may work better
// List defines the packages to either allow or deny within certain files.
// All Globs are compiled with https://pkg.go.dev/github.com/gobwas/glob#Compile.
// We do add a special case to all globs for negating the match. Prefix the string with "!".
// EX. *_test.go matches test files where !*_test.go matches anything but test files.
// Ordering of the different slices does matter as they are treated as an OR operation.
// AKA first truthy value resolves to a match.
type List struct {
	// Globs matching files to use this list for (allow list)
	// If this list is empty, it is assumes to be applied to every file.
	// Order matters so the first matching entry assumes the file should be processed by this list.
	// EX. *_test.go only applies this list to test files
	// EX. ["packageA/**", "!packageA/foo.go"] will match all files in packageA (because of ordering)
	// EX. ["!packageA/foo.go", "packageA/**"] will match all files in packageA except for foo.go
	Files []string

	// The kind of list this is
	ListType ListType

	// TODO would like a way to make suggestions!
	// The list of packages this List is concerned about.
	// Assumed to be a list of package prefixes.
	// If a glob character is detected, would use a glob match instead.
	// All non-globs are checked first, then goes through each glob in the order they were defined to try and match.
	List []string

	// Whether or not this list should try and match against packages found from GOROOT.
	// EX. os, path, strings, etc.
	IgnoreGoRootPackages bool
}

// LinterSettings define how Depguard behaves.
type LinterSettings struct {
	// The different lists that Depguard uses for import matching.
	// The order in which the lists are defined is important.
	// The first list to match on a file is used.
	Lists []*List
}

// V1Settings is used for backwards compatibility only and should move to the new LinterSettings if possible.
// Deprecated: Use LinterSettings instead.
type V1Settings struct {
	ListType        ListType
	IncludeGoRoot   bool
	Packages        []string
	TestPackages    []string
	IgnoreFileRules []string
}

// NewAnalyzerFromV2Settings returns an Analyzer from V1's settings.
// Deprecated: Use NewAnalyzer instead.
func NewAnalyzerFromV1Settings(settings *V1Settings) (*analysis.Analyzer, error) {
	ls := &LinterSettings{}
	// TODO find a way to handle the old IgnoreFileRules (mix of glob and prefix)
	hasTestList := false
	if len(settings.TestPackages) > 0 {
		hasTestList = true
		ls.Lists = append(ls.Lists, &List{
			Files:                []string{"*_test.go"},
			ListType:             settings.ListType,
			List:                 settings.TestPackages,
			IgnoreGoRootPackages: !settings.IncludeGoRoot,
		})
	}
	if len(settings.Packages) > 0 {
		files := []string{}
		if hasTestList {
			files = append(files, "!*_test.go")
		}
		ls.Lists = append(ls.Lists, &List{
			Files:                files,
			ListType:             settings.ListType,
			List:                 settings.Packages,
			IgnoreGoRootPackages: !settings.IncludeGoRoot,
		})
	}
	return NewAnalyzer(ls)
}

// NewAnalyzer creates a new analyzer from the settings passed in
func NewAnalyzer(settings *LinterSettings) (*analysis.Analyzer, error) {
	analyzer := newAnalyzer(settings)
	return analyzer, nil
}

func newAnalyzer(settings *LinterSettings) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:             "Debguard",
		Doc:              "Go linter that checks if package imports are in a list of acceptable packages",
		Run:              settings.run,
		RunDespiteErrors: false,
	}
}

func (s *LinterSettings) run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		for _, imp := range file.Imports {
			pass.ReportRangef(imp, "%s is an import", rawBasicLit(imp.Path))
		}
	}

	return nil, nil
}

func strInPrefixList(str string, prefixList []string) bool {
	// Idx represents where in the prefix slice the passed in string would go
	// when sorted. -1 Just means that it would be at the very front of the slice.
	idx := sort.Search(len(prefixList), func(i int) bool {
		return prefixList[i] > str
	}) - 1
	// This means that the string passed in has no way to be prefixed by anything
	// in the prefix list as it is already smaller then everything
	if idx == -1 {
		return false
	}
	return strings.HasPrefix(str, prefixList[idx])
}

func strInGlobList(str string, globList []glob.Glob) bool {
	for _, g := range globList {
		if g.Match(str) {
			return true
		}
	}
	return false
}

// We can do this as all imports that are not root are either prefixed with a domain
// or prefixed with `./` or `/` to dictate it is a local file reference
func listGoRootPrefixes() ([]string, error) {
	root := path.Join(build.Default.GOROOT, "src")
	fs, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("could not read GOROOT directory: %w", err)
	}
	var pkgPrefix []string
	for _, f := range fs {
		if !f.IsDir() {
			continue
		}
		pkgPrefix = append(pkgPrefix, f.Name())
	}
	return pkgPrefix, nil
}

func rawBasicLit(lit *ast.BasicLit) string {
	return strings.Trim(lit.Value, "\"")
}
