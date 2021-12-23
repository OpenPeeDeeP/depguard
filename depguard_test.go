package depguard_test

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/OpenPeeDeeP/depguard"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/loader"
)

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

	issues, err := dg.Run(newLoadConfig(), newProgram("file.go", "allow/a/pkg", "allow/a/b/pkg"))
	require.NoError(t, err)
	require.Len(t, issues, 0)
}

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

	issues, err := dg.Run(newLoadConfig(), newProgram("file.go", "deny/a/pkg", "deny/a/b/pkg"))
	require.NoError(t, err)
	require.Len(t, issues, 2)
	require.Equal(t, "deny/a/pkg", issues[0].PackageName)
	require.Equal(t, "file.go", issues[0].Position.Filename)
	require.Equal(t, "deny/a/b/pkg", issues[1].PackageName)
	require.Equal(t, "file.go", issues[1].Position.Filename)
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
