package depguard

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/OpenPeeDeeP/depguard/v2/internal/utils"
	"github.com/gobwas/glob"
)

type List struct {
	Files []string          `json:"files" yaml:"files" toml:"files" mapstructure:"files"`
	Allow []string          `json:"allow" yaml:"allow" toml:"allow" mapstructure:"allow"`
	Deny  map[string]string `json:"deny" yaml:"deny" toml:"deny" mapstructure:"deny"`
}

type list struct {
	name        string
	files       []glob.Glob
	negFiles    []glob.Glob
	allow       []glob.Glob
	deny        []glob.Glob
	suggestions []string
}

func (l *List) compile() (*list, error) {
	if l == nil {
		return nil, nil
	}
	li := &list{}
	var errs utils.MultiError
	var err error

	// Compile Files
	for _, f := range l.Files {
		var negate bool
		if len(f) > 0 && f[0] == '!' {
			negate = true
			f = f[1:]
		}
		// Expand File if needed
		fs, err := utils.ExpandSlice([]string{f}, utils.PathExpandable)
		if err != nil {
			errs = append(errs, err)
		}
		for _, exp := range fs {
			g, err := glob.Compile(exp, '/')
			if err != nil {
				errs = append(errs, fmt.Errorf("%s could not be compiled: %w", exp, err))
				continue
			}
			if negate {
				li.negFiles = append(li.negFiles, g)
				continue
			}
			li.files = append(li.files, g)
		}
	}

	if len(l.Allow) > 0 {
		// Expand Allow
		l.Allow, err = utils.ExpandSlice(l.Allow, utils.PackageExpandable)
		if err != nil {
			errs = append(errs, err)
		}
		sort.Strings(l.Allow)

		// Sort Allow
		li.allow = make([]glob.Glob, 0, len(l.Allow))
		for _, pkg := range l.Allow {
			glob, err := inputPatternToGlob(pkg)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			li.allow = append(li.allow, glob)
		}
	}

	if l.Deny != nil {
		// Expand Deny Map (to keep suggestions)
		err = utils.ExpandMap(l.Deny, utils.PackageExpandable)
		if err != nil {
			errs = append(errs, err)
		}

		// Split Deny Into Package Slice

		// sort before compiling as globs are opaque
		pkgs := make([]string, 0, len(l.Deny))
		for k := range l.Deny {
			pkgs = append(pkgs, k)
		}
		sort.Strings(pkgs)

		li.deny = make([]glob.Glob, 0, len(pkgs))
		li.suggestions = make([]string, 0, len(pkgs))
		for _, pkg := range pkgs {
			glob, err := inputPatternToGlob(pkg)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			li.deny = append(li.deny, glob)
			li.suggestions = append(li.suggestions, strings.TrimSpace(l.Deny[pkg]))
		}
	}

	// Populate the type of this list
	if len(li.allow) == 0 && len(li.deny) == 0 {
		errs = append(errs, errors.New("must have an Allow and/or Deny package list"))
	}

	if len(errs) > 0 {
		return nil, errs
	}
	return li, nil
}

func (l *list) fileMatch(fileName string) bool {
	inDenied, _ := strInGlobList(fileName, l.negFiles)
	if inDenied {
		return false
	}
	if len(l.files) == 0 {
		// No allow list matches all
		return true
	}
	inAllowed, _ := strInGlobList(fileName, l.files)
	return inAllowed
}

func (l *list) importAllowed(imp string) (bool, string) {
	inAllowed := len(l.allow) == 0
	if !inAllowed {
		inAllowed, _ = strInGlobList(imp, l.allow)
	}
	inDenied, suggIdx := strInGlobList(imp, l.deny)
	sugg := ""
	if inDenied && suggIdx != -1 {
		sugg = l.suggestions[suggIdx]
	}
	return inAllowed && !inDenied, sugg
}

type LinterSettings map[string]*List

type linterSettings []*list

func (l LinterSettings) compile() (linterSettings, error) {
	if len(l) == 0 {
		// Only allow $gostd in all files
		set := &List{
			Files: []string{"$all"},
			Allow: []string{"$gostd"},
		}
		li, err := set.compile()
		if err != nil {
			return nil, err
		}
		li.name = "Main"
		return linterSettings{li}, nil
	}
	names := make([]string, 0, len(l))
	for name := range l {
		names = append(names, name)
	}
	sort.Strings(names)
	li := make(linterSettings, 0, len(l))
	var errs utils.MultiError
	for _, name := range names {
		c, err := l[name].compile()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if c == nil {
			continue
		}
		c.name = name
		li = append(li, c)
	}
	if len(errs) > 0 {
		return nil, errs
	}

	return li, nil
}

func (ls linterSettings) whichLists(fileName string) []*list {
	var matches []*list
	for _, l := range ls {
		if l.fileMatch(fileName) {
			matches = append(matches, l)
		}
	}
	return matches
}

func strInGlobList(str string, globList []glob.Glob) (bool, int) {
	for idx, g := range globList {
		if g.Match(str) {
			return true, idx
		}
	}
	return false, 0
}

func inputPatternToGlob(pattern string) (glob.Glob, error) {
	lastIdx := len(pattern) - 1
	if pattern[lastIdx] == '$' {
		pattern = pattern[:lastIdx]
	} else {
		pattern += "**"
	}
	return glob.Compile(pattern, '/')

}
