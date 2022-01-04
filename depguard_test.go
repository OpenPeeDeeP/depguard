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

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file.go", "allow"))
	require.NoError(t, err)
	require.Len(t, issues, 0)
}

func TestPrefixAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType: depguard.LTWhitelist,
		Packages: []string{"allow"},
	}

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file.go", "allow/a", "allow/b"))
	require.NoError(t, err)
	require.Len(t, issues, 0)
}

func TestGlobAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType: depguard.LTWhitelist,
		Packages: []string{"allow/**/pkg"},
	}

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file.go", "allow/a/pkg", "allow/b/c/pkg"))
	require.NoError(t, err)
	require.Len(t, issues, 0)
}

func TestMixedAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType: depguard.LTWhitelist,
		Packages: []string{"allow"},
	}

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file.go", "allow/a", "deny/a"))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "deny/a", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
}

func TestBasicTestPackagesAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:     depguard.LTWhitelist,
		TestPackages: []string{"allowtest"},
	}

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file_test.go", "allowtest"))
	require.NoError(t, err)
	require.Len(t, issues, 0)
}

func TestPrefixTestPackagesAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:     depguard.LTWhitelist,
		TestPackages: []string{"allowtest"},
	}

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file_test.go", "allowtest/a", "allowtest/b"))
	require.NoError(t, err)
	require.Len(t, issues, 0)
}

func TestGlobTestPackagesAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:     depguard.LTWhitelist,
		TestPackages: []string{"allowtest/**/pkg"},
	}

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file_test.go", "allowtest/a/pkg", "allowtest/b/c/pkg"))
	require.NoError(t, err)
	require.Len(t, issues, 0)
}

func TestMixedTestPackagesAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:     depguard.LTWhitelist,
		TestPackages: []string{"allowtest"},
	}

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file_test.go", "allowtest/a", "denytest/a"))
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

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file.go", "go/ast"))
	require.NoError(t, err)
	require.Len(t, issues, 0)
}

func TestIncludeGoRootAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:      depguard.LTWhitelist,
		Packages:      []string{"allow"},
		IncludeGoRoot: true,
	}

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file.go", "go/ast"))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "go/ast", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
}

func TestBasicIgnoreFilesRuleAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:        depguard.LTWhitelist,
		Packages:        []string{"allow"},
		IgnoreFileRules: []string{"ignore.go"},
	}

	filesAndPackagePaths := make(map[string][]string, 0)
	filesAndPackagePaths["ignore.go"] = []string{"allow", "deny"}
	filesAndPackagePaths["file.go"] = []string{"allow", "deny"}
	issues, err := dg.Run(newLoadConfig(), newProgram(filesAndPackagePaths))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "deny", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
}

func TestPrefixIgnoreFilesRuleAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:        depguard.LTWhitelist,
		Packages:        []string{"allow"},
		IgnoreFileRules: []string{"ignore"},
	}

	filesAndPackagePaths := make(map[string][]string, 0)
	filesAndPackagePaths["ignore/file.go"] = []string{"allow", "deny"}
	filesAndPackagePaths["file.go"] = []string{"allow", "deny"}
	issues, err := dg.Run(newLoadConfig(), newProgram(filesAndPackagePaths))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "deny", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
}

func TestGlobIgnoreFilesRuleAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:        depguard.LTWhitelist,
		Packages:        []string{"allow"},
		IgnoreFileRules: []string{"ignore/**/*.go"},
	}

	filesAndPackagePaths := make(map[string][]string, 0)
	filesAndPackagePaths["ignore/a/file.go"] = []string{"allow", "deny"}
	filesAndPackagePaths["file.go"] = []string{"allow", "deny"}
	issues, err := dg.Run(newLoadConfig(), newProgram(filesAndPackagePaths))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "deny", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
}

func TestNegateGlobIgnoreFilesRuleAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:        depguard.LTWhitelist,
		Packages:        []string{"allow"},
		IgnoreFileRules: []string{"!**/keep/*.go"},
	}

	filesAndPackagePaths := make(map[string][]string, 0)
	filesAndPackagePaths["pkg/ignore/file.go"] = []string{"allow", "deny"}
	filesAndPackagePaths["pkg/keep/file.go"] = []string{"allow", "deny"}
	issues, err := dg.Run(newLoadConfig(), newProgram(filesAndPackagePaths))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "deny", issues[0].PackageName)
	require.Equal(t, "pkg/keep/file.go", issues[0].Position.Filename)
}

// NOTE: This is semantically equivalent to using the TestPackages configuration
func TestNonTestIgnoreFilesRuleAllowList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:        depguard.LTWhitelist,
		Packages:        []string{"allow"},
		IgnoreFileRules: []string{"!**/*_test.go"},
	}

	filesAndPackagePaths := make(map[string][]string, 0)
	filesAndPackagePaths["pkg/file.go"] = []string{"allow", "deny"}
	filesAndPackagePaths["pkg/file_test.go"] = []string{"allow", "deny"}
	filesAndPackagePaths["pkg/file_unit_test.go"] = []string{"allow", "deny"}
	issues, err := dg.Run(newLoadConfig(), newProgram(filesAndPackagePaths))
	require.NoError(t, err)
	require.Len(t, issues, 2)
	sortIssues(issues)
	require.Equal(t, "deny", issues[0].PackageName)
	require.Equal(t, "pkg/file_test.go", issues[0].Position.Filename)
	require.Equal(t, "deny", issues[1].PackageName)
	require.Equal(t, "pkg/file_unit_test.go", issues[1].Position.Filename)
}

// ========== DenyList ==========

func TestBasicDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType: depguard.LTBlacklist,
		Packages: []string{"deny"},
	}

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file.go", "deny"))
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

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file.go", "deny/a", "deny/b"))
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

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file.go", "deny/a/pkg", "deny/b/c/pkg"))
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

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file.go", "allow/a", "deny/a"))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "deny/a", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
}

func TestBasicTestPackagesDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:     depguard.LTBlacklist,
		Packages:     []string{"deny"}, // NOTE: Linter will shortcut with no package deny list
		TestPackages: []string{"denytest"},
	}

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file_test.go", "denytest"))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "denytest", issues[0].PackageName)
	require.Equal(t, "file_test.go", issues[0].Position.Filename)
}

func TestPrefixTestPackagesDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:     depguard.LTBlacklist,
		Packages:     []string{"deny"}, // NOTE: Linter will shortcut with no package deny list
		TestPackages: []string{"denytest"},
	}

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file_test.go", "denytest/a", "denytest/b"))
	require.NoError(t, err)
	require.Len(t, issues, 2)
	sortIssues(issues)
	require.Equal(t, "denytest/a", issues[0].PackageName)
	require.Equal(t, "file_test.go", issues[0].Position.Filename)
	require.Equal(t, "denytest/b", issues[1].PackageName)
	require.Equal(t, "file_test.go", issues[1].Position.Filename)
}

func TestGlobTestPackagesDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:     depguard.LTBlacklist,
		Packages:     []string{"deny"}, // NOTE: Linter will shortcut with no package deny list
		TestPackages: []string{"denytest/**/pkg"},
	}

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file_test.go", "denytest/a/pkg", "denytest/b/c/pkg"))
	require.NoError(t, err)
	require.Len(t, issues, 2)
	sortIssues(issues)
	require.Equal(t, "denytest/a/pkg", issues[0].PackageName)
	require.Equal(t, "file_test.go", issues[0].Position.Filename)
	require.Equal(t, "denytest/b/c/pkg", issues[1].PackageName)
	require.Equal(t, "file_test.go", issues[1].Position.Filename)
}

func TestMixedTestPackagesDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:     depguard.LTBlacklist,
		Packages:     []string{"deny"}, // NOTE: Linter will shortcut with no package deny list
		TestPackages: []string{"denytest"},
	}

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file_test.go", "allowtest/a", "denytest/a"))
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

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file.go", "go/ast"))
	require.NoError(t, err)
	require.Len(t, issues, 0)
}

func TestIncludeGoRootDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:      depguard.LTBlacklist,
		Packages:      []string{"go/ast"},
		IncludeGoRoot: true,
	}

	issues, err := dg.Run(newLoadConfig(), newSimpleProgram("file.go", "go/ast"))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "go/ast", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
}

func TestBasicIgnoreFilesRuleDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:        depguard.LTBlacklist,
		Packages:        []string{"deny"},
		IgnoreFileRules: []string{"ignore.go"},
	}

	filesAndPackagePaths := make(map[string][]string, 0)
	filesAndPackagePaths["ignore.go"] = []string{"deny"}
	filesAndPackagePaths["file.go"] = []string{"deny"}
	issues, err := dg.Run(newLoadConfig(), newProgram(filesAndPackagePaths))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "deny", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
}

func TestPrefixIgnoreFilesRuleDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:        depguard.LTBlacklist,
		Packages:        []string{"deny"},
		IgnoreFileRules: []string{"ignore"},
	}

	filesAndPackagePaths := make(map[string][]string, 0)
	filesAndPackagePaths["ignore/file.go"] = []string{"deny"}
	filesAndPackagePaths["file.go"] = []string{"deny"}
	issues, err := dg.Run(newLoadConfig(), newProgram(filesAndPackagePaths))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "deny", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
}

func TestGlobIgnoreFilesRuleDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:        depguard.LTBlacklist,
		Packages:        []string{"deny"},
		IgnoreFileRules: []string{"ignore/**/*.go"},
	}

	filesAndPackagePaths := make(map[string][]string, 0)
	filesAndPackagePaths["ignore/a/file.go"] = []string{"deny"}
	filesAndPackagePaths["file.go"] = []string{"deny"}
	issues, err := dg.Run(newLoadConfig(), newProgram(filesAndPackagePaths))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "deny", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
}

func TestNegateGlobIgnoreFilesRuleDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:        depguard.LTBlacklist,
		Packages:        []string{"deny"},
		IgnoreFileRules: []string{"!**/keep/*.go"},
	}

	filesAndPackagePaths := make(map[string][]string, 0)
	filesAndPackagePaths["pkg/ignore/file.go"] = []string{"deny"}
	filesAndPackagePaths["pkg/keep/file.go"] = []string{"deny"}
	issues, err := dg.Run(newLoadConfig(), newProgram(filesAndPackagePaths))
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Equal(t, "deny", issues[0].PackageName)
	require.Equal(t, "pkg/keep/file.go", issues[0].Position.Filename)
}

// NOTE: This is semantically equivalent to using the TestPackages configuration
func TestNonTestIgnoreFilesRuleDenyList(t *testing.T) {
	dg := depguard.Depguard{
		ListType:        depguard.LTBlacklist,
		Packages:        []string{"deny"},
		IgnoreFileRules: []string{"!**/*_test.go"},
	}

	filesAndPackagePaths := make(map[string][]string, 0)
	filesAndPackagePaths["pkg/file.go"] = []string{"deny"}
	filesAndPackagePaths["pkg/file_test.go"] = []string{"deny"}
	filesAndPackagePaths["pkg/file_unit_test.go"] = []string{"deny"}
	issues, err := dg.Run(newLoadConfig(), newProgram(filesAndPackagePaths))
	require.NoError(t, err)
	require.Len(t, issues, 2)
	sortIssues(issues)
	require.Equal(t, "deny", issues[0].PackageName)
	require.Equal(t, "pkg/file_test.go", issues[0].Position.Filename)
	require.Equal(t, "deny", issues[1].PackageName)
	require.Equal(t, "pkg/file_unit_test.go", issues[1].Position.Filename)
}

func newLoadConfig() *loader.Config {
	return &loader.Config{
		Cwd:   "",
		Build: nil,
	}
}

func newSimpleProgram(fileName string, packagePaths ...string) *loader.Program {
	filesAndPackagePaths := make(map[string][]string, 1)
	filesAndPackagePaths[fileName] = packagePaths
	return newProgram(filesAndPackagePaths)
}

func newProgram(filesAndPackagePaths map[string][]string) *loader.Program {
	var astFiles []*ast.File
	progFileSet := token.NewFileSet()

	programCounter := 1
	for fileName, packagePaths := range filesAndPackagePaths {
		// Build up a mini AST of the information we need to run the linter
		var packageImports []*ast.ImportSpec
		for i, _ := range packagePaths {
			packagePath := packagePaths[i]
			packageImports = append(packageImports, &ast.ImportSpec{
				Path: &ast.BasicLit{
					ValuePos: token.Pos(programCounter + i),
					Kind:     token.STRING,
					Value:    packagePath,
				},
			})
		}

		astFiles = append(astFiles, &ast.File{
			Imports: packageImports,
		})

		progFileSet.AddFile(fileName, programCounter, len(packageImports))
		programCounter += len(packageImports) + 1
	}

	return &loader.Program{
		Created: []*loader.PackageInfo{
			{
				Pkg:                   nil,
				Importable:            true,
				TransitivelyErrorFree: true,

				Files:  astFiles,
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
