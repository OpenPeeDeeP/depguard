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

func ExpandSlice(sl []string, exp ExpanderMap) ([]string, error) {
	for i, s := range sl {
		f, found := exp[s]
		if !found {
			continue
		}
		e, err := f.Expand()
		if err != nil {
			return nil, fmt.Errorf("couldn't expand %s: %w", s, err)
		}
		sl = insertSlice(sl, i, e...)
	}
	return sl, nil
}

func ExpandMap(m map[string]string, exp ExpanderMap) error {
	for k, v := range m {
		f, found := exp[k]
		if !found {
			continue
		}
		e, err := f.Expand()
		if err != nil {
			return fmt.Errorf("couldn't expand %s: %w", k, err)
		}
		for _, ex := range e {
			m[ex] = v
		}
		delete(m, k)
	}
	return nil
}

func insertSlice(a []string, k int, b ...string) []string {
	n := len(a) + len(b) - 1
	if n <= cap(a) {
		a2 := a[:n]
		copy(a2[k+len(b):], a[k+1:])
		copy(a2[k:], b)
		return a2
	}
	a2 := make([]string, n)
	copy(a2, a[:k])
	copy(a2[k:], b)
	copy(a2[k+len(b):], a[k+1:])
	return a2
}
