package utils

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/tools/go/buildutil"
	"golang.org/x/tools/go/packages"
)

type Expander interface {
	Expand() ([]string, error)
}

type ExpanderMap map[string]Expander

func (em ExpanderMap) Expand(str string) ([]string, bool, error) {
	e, found := em[str]
	if !found {
		return nil, false, nil
	}
	expanded, err := e.Expand()
	if err != nil {
		return nil, true, fmt.Errorf("couldn't expand %s: %w", str, err)
	}
	return expanded, true, nil
}

var (
	PathExpandable = ExpanderMap{
		"$all":  &allExpander{},
		"$test": &testExpander{},
	}
	PackageExpandable = ExpanderMap{
		"$gostd": &gostdExpander{},
	}
	goStdPkgs = sync.OnceValues(findStdPkgs)
)

type allExpander struct{}

func (*allExpander) Expand() ([]string, error) {
	return []string{"**/*.go"}, nil
}

type testExpander struct{}

func (*testExpander) Expand() ([]string, error) {
	return []string{"**/*_test.go"}, nil
}

type gostdExpander struct{}

func (e *gostdExpander) Expand() ([]string, error) {
	return goStdPkgs()
}

func findStdPkgs() ([]string, error) {
	var stdPkgs []string
	if goroot := os.Getenv("GOROOT"); goroot != "" {
		// User explicitly set GOROOT, we should use that
		buildCtx := build.Default
		buildCtx.GOROOT = goroot
		buildCtx.GOPATH = "" // I don't want GOPATH packages only GOROOT
		for _, p := range buildutil.AllPackages(&buildCtx) {
			if !isStdPkg(p) {
				continue
			}
			files, err := os.ReadDir(filepath.Join(goroot, "src", p))
			if err != nil {
				continue
			}
			hasGoFile := false
			for _, f := range files {
				if f.IsDir() {
					continue
				}
				if strings.HasSuffix(f.Name(), "_test.go") {
					continue
				}
				if !strings.HasSuffix(f.Name(), ".go") {
					continue
				}
				hasGoFile = true
				break
			}
			if hasGoFile {
				stdPkgs = append(stdPkgs, p+"$")
			}
		}
		if len(stdPkgs) == 0 {
			return nil, fmt.Errorf("GOROOT=%q contains no standard library packages", goroot)
		}
		return stdPkgs, nil
	}
	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedName}, "std")
	if err != nil {
		return nil, fmt.Errorf("could not list standard library: %w", err)
	}
	for _, p := range pkgs {
		if !isStdPkg(p.PkgPath) {
			continue
		}
		stdPkgs = append(stdPkgs, p.PkgPath+"$")
	}
	return stdPkgs, nil
}

func isStdPkg(str string) bool {
	return str != "cmd" &&
		!strings.HasPrefix(str, "cmd/") &&
		str != "vendor" &&
		!strings.HasPrefix(str, "vendor/") &&
		str != "internal" &&
		!strings.HasPrefix(str, "internal/") &&
		!strings.Contains(str, "/internal/") &&
		!strings.HasSuffix(str, "/internal")
}
