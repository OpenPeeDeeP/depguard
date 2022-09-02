package utils

import (
	"fmt"
	"go/build"
	"os"
	"path"
)

type Expander interface {
	Expand() ([]string, error)
}

var (
	PathExpandable = map[string]Expander{
		"$all":  &allExpander{},
		"$test": &testExpander{},
	}
	PackageExpandable = map[string]Expander{
		"$gostd": &gostdExpander{},
	}
)

type allExpander struct{}

func (*allExpander) Expand() ([]string, error) {
	return []string{"**/*.go"}, nil
}

type testExpander struct{}

func (*testExpander) Expand() ([]string, error) {
	return []string{"**/*_test.go"}, nil
}

type gostdExpander struct {
	cache []string
}

// We can do this as all imports that are not root are either prefixed with a domain
// or prefixed with `./` or `/` to dictate it is a local file reference
func (e *gostdExpander) Expand() ([]string, error) {
	if len(e.cache) != 0 {
		return e.cache, nil
	}
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
	e.cache = pkgPrefix
	return pkgPrefix, nil
}
