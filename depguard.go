package depguard

import (
	"sort"
	"strings"

	"github.com/gobwas/glob"
	"golang.org/x/tools/go/loader"
)

// Depguard checks imports to make sure they follow the given list and constraints.
type Depguard struct {
	ListType      ListType
	IncludeGoRoot bool

	Packages       []string
	prefixPackages []string
	globPackages   []glob.Glob

	TestPackages       []string
	prefixTestPackages []string
	globTestPackages   []glob.Glob

	prefixRoot []string
}

// Run checks for dependencies given the program and validates them against
// Packages.
func (dg *Depguard) Run(config *loader.Config, prog *loader.Program) ([]*Issue, error) {
	// Shortcut execution on an empty blacklist as that means every package is allowed
	if dg.ListType == LTBlacklist && len(dg.Packages) == 0 {
		return nil, nil
	}

	if err := dg.initialize(config); err != nil {
		return nil, err
	}
	directImports, err := createImportMap(prog, ImportWithRootPrefixes(dg.prefixRoot))
	if err != nil {
		return nil, err
	}
	var issues []*Issue
	for pkg, positions := range directImports {
		for _, pos := range positions {

			prefixList, globList := dg.prefixPackages, dg.globPackages
			if len(dg.TestPackages) > 0 && strings.Index(pos.Filename, "_test.go") != -1 {
				prefixList, globList = dg.prefixTestPackages, dg.globTestPackages
			}

			if flagIt(pkg, dg.ListType, prefixList, globList) {
				issues = append(issues, &Issue{
					PackageName: pkg,
					Position:    pos,
				})
			}
		}
	}
	return issues, nil
}

func (dg *Depguard) initialize(config *loader.Config) error {
	// parse ordinary guarded packages
	for _, pkg := range dg.Packages {
		if strings.ContainsAny(pkg, globMatching) {
			g, err := glob.Compile(pkg, '/')
			if err != nil {
				return err
			}
			dg.globPackages = append(dg.globPackages, g)
		} else {
			dg.prefixPackages = append(dg.prefixPackages, pkg)
		}
	}

	// Sort the packages so we can have a faster search in the array
	sort.Strings(dg.prefixPackages)

	// parse guarded tests packages
	for _, pkg := range dg.TestPackages {
		if strings.ContainsAny(pkg, globMatching) {
			g, err := glob.Compile(pkg, '/')
			if err != nil {
				return err
			}
			dg.globTestPackages = append(dg.globTestPackages, g)
		} else {
			dg.prefixTestPackages = append(dg.prefixTestPackages, pkg)
		}
	}

	// Sort the test packages so we can have a faster search in the array
	sort.Strings(dg.prefixTestPackages)

	if !dg.IncludeGoRoot {
		var err error
		dg.prefixRoot, err = listRootPrefixs(config.Build)
		if err != nil {
			return err
		}
	}

	return nil
}
