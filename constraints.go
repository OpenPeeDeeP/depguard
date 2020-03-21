package depguard

import (
	"go/token"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gobwas/glob"
	"golang.org/x/tools/go/loader"
)

// Constraints checks imports for each constraing file/folder.
type Constraints struct {
	Constraints  map[string]*Constraint
	AllowMissing bool

	constraintsPrefix constraintsPrefixes
	constraintsGlob   []*constraintsGlob
	rootPrefixes      []string
	anyIncludeGoRoot  bool

	fileCache map[string]*Constraint
}

type constraintsGlob struct {
	glob.Glob
	orig string
}

type constraintsPrefixes []*constraintsPrefix

type constraintsPrefix struct {
	comp string
	orig string
}

func (cp constraintsPrefixes) Len() int {
	return len(cp)
}

func (cp constraintsPrefixes) Less(i, j int) bool {
	return cp[i].comp < cp[j].comp
}

func (cp constraintsPrefixes) Swap(i, j int) {
	cp[i], cp[j] = cp[j], cp[i]
}

// Run checks for dependencies given the program and validates them against
// the appropriate constraint.
func (c *Constraints) Run(config *loader.Config, prog *loader.Program) ([]*Issue, error) {
	err := c.initialize(config)
	if err != nil {
		return nil, err
	}
	directImports, err := createImportMap(prog, ImportWithRootPrefixes(c.rootPrefixes))
	if err != nil {
		return nil, err
	}
	var issues []*Issue
	for pkg, positions := range directImports {
		for _, pos := range positions {
			con := c.findConstraint(pos)
			if con == nil && !c.AllowMissing {
				issues = append(issues, &Issue{Position: pos})
				continue
			}
			if !con.IncludeGoRoot && isRoot(pkg, c.rootPrefixes) {
				continue
			}
			if flagIt(pkg, con.ListType, con.prefixPackages, con.globPackages) {
				issues = append(issues, &Issue{
					PackageName: pkg,
					Position:    pos,
					ListType:    con.ListType,
				})
			}
		}
	}
	return issues, nil
}

func (c *Constraints) initialize(config *loader.Config) error {
	c.anyIncludeGoRoot = c.anyIncludeGoRootCheck()
	c.fileCache = make(map[string]*Constraint)
	for conPrefix, con := range c.Constraints {
		abs, err := filepath.Abs(conPrefix)
		if err != nil {
			return err
		}
		// parse the constraint prefix to the appropriate slice
		if strings.ContainsAny(conPrefix, globMatching) {
			g, err := glob.Compile(abs, '/')
			if err != nil {
				return err
			}
			c.constraintsGlob = append(c.constraintsGlob, &constraintsGlob{g, conPrefix})
		} else {
			c.constraintsPrefix = append(c.constraintsPrefix, &constraintsPrefix{abs, conPrefix})
		}

		// initialize each constraint
		err = con.initialize()
		if err != nil {
			return err
		}
	}

	// Sort so we can have a faster search in the array
	sort.Sort(c.constraintsPrefix)

	// Only find GoRoot prefixes if we need to
	if !c.anyIncludeGoRoot {
		var err error
		c.rootPrefixes, err = listRootPrefixs(config.Build)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Constraints) findConstraint(pos token.Position) *Constraint {
	if con, found := c.fileCache[pos.Filename]; found {
		return con
	}

	var con *Constraint
	// Idx represents where in the package slice the passed in package would go
	// when sorted. -1 Just means that it would be at the very front of the slice.
	idx := sort.Search(len(c.constraintsPrefix), func(i int) bool {
		return c.constraintsPrefix[i].comp > pos.Filename
	}) - 1
	// This means that the package passed in has no way to be prefixed by anything
	// in the package list as it is already smaller then everything
	if idx != -1 && strings.HasPrefix(pos.Filename, c.constraintsPrefix[idx].comp) {
		con = c.Constraints[c.constraintsPrefix[idx].orig]
	}

	for _, g := range c.constraintsGlob {
		if g.Match(pos.Filename) {
			tmpCon := c.Constraints[g.orig]
			if con == nil {
				con = tmpCon
				continue
			}
			if tmpCon.Priority < con.Priority {
				con = tmpCon
			}
		}
	}

	c.fileCache[pos.Filename] = con
	return con
}

func (c *Constraints) anyIncludeGoRootCheck() bool {
	for _, con := range c.Constraints {
		if con.IncludeGoRoot {
			return true
		}
	}
	return false
}

// Constraint constrains the files/folders to the listed packages.
type Constraint struct {
	ListType      ListType
	IncludeGoRoot bool
	Priority      int

	Packages       []string
	prefixPackages []string
	globPackages   []glob.Glob
}

func (c *Constraint) initialize() error {
	for _, p := range c.Packages {
		if strings.ContainsAny(p, globMatching) {
			g, err := glob.Compile(p, '/')
			if err != nil {
				return err
			}
			c.globPackages = append(c.globPackages, g)
		} else {
			c.prefixPackages = append(c.prefixPackages, p)
		}
	}

	// Sort so we can have a faster search in the array
	sort.Strings(c.prefixPackages)
	return nil
}
