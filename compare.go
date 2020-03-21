package depguard

import (
	"go/build"
	"go/token"
	"io/ioutil"
	"path"
	"sort"
	"strings"

	"github.com/gobwas/glob"
	"golang.org/x/tools/go/loader"
)

var globMatching = "!?*[]{}"

// ListType states what kind of list is passed in.
type ListType int

const (
	// LTBlacklist states the list given is a blacklist. (default)
	LTBlacklist ListType = iota
	// LTWhitelist states the list given is a whitelist.
	LTWhitelist
)

// StringToListType makes it easier to turn a string into a ListType.
// It assumes that the string representation is lower case.
var StringToListType = map[string]ListType{
	"whitelist": LTWhitelist,
	"blacklist": LTBlacklist,
}

// Issue with the package with PackageName at the Position.
type Issue struct {
	PackageName string
	Position    token.Position
	ListType    ListType
}

type importOptions struct {
	rootPrefixes []string
}

// ImportOption is optional configuration when importing
type ImportOption func(*importOptions)

// ImportWithRootPrefixes adds GoRoot prefixes to use with the lists
func ImportWithRootPrefixes(rootPrefixes []string) ImportOption {
	return func(opt *importOptions) {
		opt.rootPrefixes = rootPrefixes
	}
}

func createImportMap(prog *loader.Program, opts ...ImportOption) (map[string][]token.Position, error) {
	opt := &importOptions{}
	for _, o := range opts {
		o(opt)
	}
	importMap := make(map[string][]token.Position)
	// For the directly imported packages
	for _, imported := range prog.InitialPackages() {
		// Go through their files
		for _, file := range imported.Files {
			// And populate a map of all direct imports and their positions
			// This will filter out GoRoot depending on the Depguard.IncludeGoRoot
			for _, fileImport := range file.Imports {
				fileImportPath := cleanBasicLitString(fileImport.Path.Value)
				if len(opt.rootPrefixes) > 0 && isRoot(fileImportPath, opt.rootPrefixes) {
					continue
				}
				position := prog.Fset.Position(fileImport.Pos())
				positions, found := importMap[fileImportPath]
				if !found {
					importMap[fileImportPath] = []token.Position{
						position,
					}
					continue
				}
				importMap[fileImportPath] = append(positions, position)
			}
		}
	}
	return importMap, nil
}

func pkgInList(pkg string, prefixList []string, globList []glob.Glob) bool {
	if pkgInPrefixList(pkg, prefixList) {
		return true
	}
	return pkgInGlobList(pkg, globList)
}

func pkgInPrefixList(pkg string, prefixList []string) bool {
	// Idx represents where in the package slice the passed in package would go
	// when sorted. -1 Just means that it would be at the very front of the slice.
	idx := sort.Search(len(prefixList), func(i int) bool {
		return prefixList[i] > pkg
	}) - 1
	// This means that the package passed in has no way to be prefixed by anything
	// in the package list as it is already smaller then everything
	if idx == -1 {
		return false
	}
	return strings.HasPrefix(pkg, prefixList[idx])
}

func pkgInGlobList(pkg string, globList []glob.Glob) bool {
	for _, g := range globList {
		if g.Match(pkg) {
			return true
		}
	}
	return false
}

// InList | WhiteList | BlackList
//   y   |           |     x
//   n   |     x     |
func flagIt(pkg string, listType ListType, prefixList []string, globList []glob.Glob) bool {
	return pkgInList(pkg, prefixList, globList) == (listType == LTBlacklist)
}

func cleanBasicLitString(value string) string {
	return strings.Trim(value, "\"\\")
}

// We can do this as all imports that are not root are either prefixed with a domain
// or prefixed with `./` or `/` to dictate it is a local file reference
func listRootPrefixs(buildCtx *build.Context) ([]string, error) {
	if buildCtx == nil {
		buildCtx = &build.Default
	}
	root := path.Join(buildCtx.GOROOT, "src")
	fs, err := ioutil.ReadDir(root)
	if err != nil {
		return nil, err
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

func isRoot(importPath string, rootPrefixes []string) bool {
	// Idx represents where in the package slice the passed in package would go
	// when sorted. -1 Just means that it would be at the very front of the slice.
	idx := sort.Search(len(rootPrefixes), func(i int) bool {
		return rootPrefixes[i] > importPath
	}) - 1
	// This means that the package passed in has no way to be prefixed by anything
	// in the package list as it is already smaller then everything
	if idx == -1 {
		return false
	}
	// if it is prefixed by a root prefix we need to check if it is an exact match
	// or prefix with `/` as this could return false positive if the domain was
	// `archive.com` for example as `archive` is a go root package.
	if strings.HasPrefix(importPath, rootPrefixes[idx]) {
		return strings.HasPrefix(importPath, rootPrefixes[idx]+"/") || importPath == rootPrefixes[idx]
	}
	return false
}
