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
	ListMode string            `json:"listMode" yaml:"listMode" toml:"listMode" mapstructure:"listMode"`
	Files    []string          `json:"files" yaml:"files" toml:"files" mapstructure:"files"`
	Allow    []string          `json:"allow" yaml:"allow" toml:"allow" mapstructure:"allow"`
	Deny     map[string]string `json:"deny" yaml:"deny" toml:"deny" mapstructure:"deny"`
}

type listMode int

const (
	listModeOriginal listMode = iota
	listModeStrict
	listModeLax
)

type list struct {
	listMode    listMode
	name        string
	files       []*glob.Pattern
	negFiles    []*glob.Pattern
	tree        *utils.CompressedPrefixTree
	suggestions []string
	anyAllows   bool // only for backwards compatible original lists
}

func (l *List) compile() (*list, error) {
	if l == nil {
		return nil, nil
	}
	li := &list{}
	var errs utils.MultiError
	var err error

	// Determine List Mode
	switch strings.ToLower(l.ListMode) {
	case "":
		li.listMode = listModeOriginal
	case "original":
		li.listMode = listModeOriginal
	case "strict":
		li.listMode = listModeStrict
	case "lax":
		li.listMode = listModeLax
	default:
		errs = append(errs, fmt.Errorf("%s is not a known list mode", l.ListMode))
	}

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

	li.tree = utils.NewCompressedPrefixTree()

	if len(l.Allow) > 0 {
		// Expand Allow
		l.Allow, err = utils.ExpandSlice(l.Allow, utils.PackageExpandable)
		if err != nil {
			errs = append(errs, err)
		}
		for _, allow := range l.Allow {
			if allow == "" {
				errs = append(errs, errors.New("allow contains an empty package"))
				continue
			}
			li.anyAllows = true
			li.tree.InsertAllow(allow)
		}
	}

	if l.Deny != nil {
		// Expand Deny Map (to keep suggestions)
		err = utils.ExpandMap(l.Deny, utils.PackageExpandable)
		if err != nil {
			errs = append(errs, err)
		}

		li.suggestions = make([]string, 0, len(l.Deny))
		for deny, sug := range l.Deny {
			if deny == "" {
				errs = append(errs, errors.New("deny contains an empty package"))
				continue
			}
			li.tree.InsertDeny(deny, len(li.suggestions))
			li.suggestions = append(li.suggestions, strings.TrimSpace(sug))
		}
	}

	// Populate the type of this list
	if !li.anyAllows && len(li.suggestions) == 0 {
		errs = append(errs, errors.New("must have an Allow and/or Deny package list"))
	}

	if len(errs) > 0 {
		return nil, errs
	}
	return li, nil
}

func (l *list) fileMatch(fileName string) bool {
	inAllowed := len(l.files) == 0 || strInGlobList(fileName, l.files)
	inDenied := strInGlobList(fileName, l.negFiles)
	return inAllowed && !inDenied
}

func (l *list) importAllowed(imp string) (bool, string) {
	aLen, dLen, dSug := l.tree.Search(imp)
	inAllowed := aLen != -1
	inDenied := dLen != -1
	var allowed bool
	switch l.listMode {
	case listModeOriginal:
		inAllowed = !l.anyAllows || inAllowed
		allowed = inAllowed && !inDenied
	case listModeStrict:
		allowed = inAllowed && (!inDenied || aLen > dLen)
	case listModeLax:
		allowed = !inDenied || (inAllowed && aLen > dLen)
	default:
		allowed = false
	}
	sugg := ""
	if !allowed && inDenied && dSug != -1 {
		sugg = l.suggestions[dSug]
	}
	return allowed, sugg
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
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
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

func (s linterSettings) whichLists(fileName string) []*list {
	var matches []*list
	for _, l := range s {
		if l.fileMatch(fileName) {
			matches = append(matches, l)
		}
	}
	return matches
}

func strInGlobList(str string, globList []*glob.Pattern) bool {
	for _, g := range globList {
		if g.Match(str) {
			return true
		}
	}
	return false
}
