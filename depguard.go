package depguard

import (
	"go/ast"
	"sort"
	"strings"

	"github.com/gobwas/glob"
	"golang.org/x/tools/go/analysis"
)

// NewAnalyzer creates a new analyzer from the settings passed in
func NewAnalyzer(settings *LinterSettings) (*analysis.Analyzer, error) {
	s, err := settings.compile()
	if err != nil {
		return nil, err
	}
	analyzer := newAnalyzer(s)
	return analyzer, nil
}

func newAnalyzer(settings linterSettings) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:             "Debguard",
		Doc:              "Go linter that checks if package imports are in a list of acceptable packages",
		Run:              settings.run,
		RunDespiteErrors: false,
	}
}

func (s linterSettings) run(pass *analysis.Pass) (interface{}, error) {
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

func rawBasicLit(lit *ast.BasicLit) string {
	return strings.Trim(lit.Value, "\"")
}
