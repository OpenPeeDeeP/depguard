package depguard_test

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"
	"testing"

	"github.com/OpenPeeDeeP/depguard"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/loader"
)

// ========== AllowList ==========

func TestBasicAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType: depguard.LTWhitelist,
		Packages: []string{"allow"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file.go", "allow"))
	require.NoError(t, err)
	require.Len(t, issues, 0)
}

func TestPrefixAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType: depguard.LTWhitelist,
		Packages: []string{"allow"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file.go", "allow/a", "allow/b"))
	require.NoError(t, err)
	require.Len(t, issues, 0)
}

func TestGlobAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType: depguard.LTWhitelist,
		Packages: []string{"allow/**/pkg"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file.go", "allow/a/pkg", "allow/b/c/pkg"))
	require.NoError(t, err)
	require.Len(t, issues, 0)
}

func TestMixedAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType: depguard.LTWhitelist,
		Packages: []string{"allow"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file.go", "allow/a", "deny/a"))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "deny/a", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
}

func TestBasicTestFileAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:     depguard.LTWhitelist,
		TestPackages: []string{"allowtest"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file_test.go", "allowtest"))
	require.NoError(t, err)
	require.Len(t, issues, 0)
}

func TestPrefixTestFileAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:     depguard.LTWhitelist,
		TestPackages: []string{"allowtest"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file_test.go", "allowtest/a", "allowtest/b"))
	require.NoError(t, err)
	require.Len(t, issues, 0)
}

func TestGlobTestFileAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:     depguard.LTWhitelist,
		TestPackages: []string{"allowtest/**/pkg"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file_test.go", "allowtest/a/pkg", "allowtest/b/c/pkg"))
	require.NoError(t, err)
	require.Len(t, issues, 0)
}

func TestMixedTestFileAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:     depguard.LTWhitelist,
		TestPackages: []string{"allowtest"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file_test.go", "allowtest/a", "denytest/a"))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "denytest/a", issues[0].PackageName)
	require.Equal(t, "file_test.go", issues[0].Position.Filename)
}

func TestExcludeGoRootAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType: depguard.LTWhitelist,
		Packages: []string{"allow"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file.go", "go/ast"))
	require.NoError(t, err)
	require.Len(t, issues, 0)
}

func TestIncludeGoRootAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:      depguard.LTWhitelist,
		Packages:      []string{"allow"},
		IncludeGoRoot: true,
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file.go", "go/ast"))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "go/ast", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
}

// ========== DenyList ==========

func TestBasicDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType: depguard.LTBlacklist,
		Packages: []string{"deny"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file.go", "deny"))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "deny", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
}

func TestPrefixDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType: depguard.LTBlacklist,
		Packages: []string{"deny"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file.go", "deny/a", "deny/b"))
	require.NoError(t, err)
	require.Len(t, issues, 2)
	sortIssues(issues)
	require.Equal(t, "deny/a", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
	require.Equal(t, "deny/b", issues[1].PackageName)
	require.Equal(t, "file.go", issues[1].Position.Filename)
}

func TestGlobDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType: depguard.LTBlacklist,
		Packages: []string{"deny/**/pkg"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file.go", "deny/a/pkg", "deny/b/c/pkg"))
	require.NoError(t, err)
	require.Len(t, issues, 2)
	sortIssues(issues)
	require.Equal(t, "deny/a/pkg", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
	require.Equal(t, "deny/b/c/pkg", issues[1].PackageName)
	require.Equal(t, "file.go", issues[1].Position.Filename)
}

func TestMixedDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType: depguard.LTBlacklist,
		Packages: []string{"deny"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file.go", "allow/a", "deny/a"))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "deny/a", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
}

func TestBasicTestFileDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:     depguard.LTBlacklist,
		Packages:     []string{"deny"}, // NOTE: Linter will shortcut with no package deny list
		TestPackages: []string{"denytest"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file_test.go", "denytest"))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "denytest", issues[0].PackageName)
	require.Equal(t, "file_test.go", issues[0].Position.Filename)
}

func TestPrefixTestFileDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:     depguard.LTBlacklist,
		Packages:     []string{"deny"}, // NOTE: Linter will shortcut with no package deny list
		TestPackages: []string{"denytest"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file_test.go", "denytest/a", "denytest/b"))
	require.NoError(t, err)
	require.Len(t, issues, 2)
	sortIssues(issues)
	require.Equal(t, "denytest/a", issues[0].PackageName)
	require.Equal(t, "file_test.go", issues[0].Position.Filename)
	require.Equal(t, "denytest/b", issues[1].PackageName)
	require.Equal(t, "file_test.go", issues[1].Position.Filename)
}

func TestGlobTestFileDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:     depguard.LTBlacklist,
		Packages:     []string{"deny"}, // NOTE: Linter will shortcut with no package deny list
		TestPackages: []string{"denytest/**/pkg"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file_test.go", "denytest/a/pkg", "denytest/b/c/pkg"))
	require.NoError(t, err)
	require.Len(t, issues, 2)
	sortIssues(issues)
	require.Equal(t, "denytest/a/pkg", issues[0].PackageName)
	require.Equal(t, "file_test.go", issues[0].Position.Filename)
	require.Equal(t, "denytest/b/c/pkg", issues[1].PackageName)
	require.Equal(t, "file_test.go", issues[1].Position.Filename)
}

func TestMixedTestFileDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:     depguard.LTBlacklist,
		Packages:     []string{"deny"}, // NOTE: Linter will shortcut with no package deny list
		TestPackages: []string{"denytest"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file_test.go", "allowtest/a", "denytest/a"))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "denytest/a", issues[0].PackageName)
	require.Equal(t, "file_test.go", issues[0].Position.Filename)
}

func TestExcludeGoRootDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType: depguard.LTBlacklist,
		Packages: []string{"go/ast"},
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file.go", "go/ast"))
	require.NoError(t, err)
	require.Len(t, issues, 0)
}

func TestIncludeGoRootDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:      depguard.LTBlacklist,
		Packages:      []string{"go/ast"},
		IncludeGoRoot: true,
	}

	issues, err := dg.Run(newLoadConfig(), newProgram("file.go", "go/ast"))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "go/ast", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
}

func newLoadConfig() *loader.Config {
	return &loader.Config{
		Cwd:   "",
		Build: nil,
	}
}

func newProgram(fileName string, packagePaths ...string) *loader.Program {
	// Build up a mini AST of the information we need to run the linter
	var packageImports []*ast.ImportSpec
	for i, _ := range packagePaths {
		packagePath := packagePaths[i]
		packageImports = append(packageImports, &ast.ImportSpec{
			Path: &ast.BasicLit{
				ValuePos: token.Pos(1),
				Kind:     token.STRING,
				Value:    packagePath,
			},
		})
	}

	astFile := &ast.File{
		Imports: packageImports,
	}

	progFileSet := token.NewFileSet()
	progFileSet.AddFile(fileName, 1, 0)

	return &loader.Program{
		Created: []*loader.PackageInfo{
			{
				Pkg:                   nil,
				Importable:            true,
				TransitivelyErrorFree: true,

				Files:  []*ast.File{astFile},
				Errors: nil,
			},
		},
		Fset: progFileSet,
	}
}

func sortIssues(issues []*depguard.Issue) {
	sort.Slice(issues, func(i, j int) bool {
		return strings.Compare(issues[i].PackageName, issues[j].PackageName) < 0
	})
}
