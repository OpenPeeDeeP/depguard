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
	Files []string          `json:"files" yaml:"files" toml:"files"`
	Allow []string          `json:"allow" yaml:"allow" toml:"allow"`
	Deny  map[string]string `json:"deny" yaml:"deny" toml:"deny"`
}

type listMode int

const (
	lmAllow listMode = iota // Only packages in allow are allowed
	lmDeny                  // Any package in deny is blocked
	lmMixed                 // Package must exist in allow and not be blocked in deny
)

type list struct {
	name        string
	files       []glob.Glob
	listMode    listMode
	allow       []string
	deny        []string
	suggestions []string
}

func (l *List) compile() (*list, error) {
	li := &list{}
	var errs utils.MultiError
	// TODO Expand Files
	// TODO Split files to negatable globs

	// Compile Files
	li.files = make([]glob.Glob, 0, len(l.Files))
	for _, f := range l.Files {
		g, err := glob.Compile(f, '/')
		if err != nil {
			errs = append(errs, fmt.Errorf("%s could not be compiled: %w", f, err))
			continue
		}
		li.files = append(li.files, g)
	}

	// TODO Expand Allow

	// Sort Allow
	li.allow = make([]string, len(l.Allow))
	copy(li.allow, l.Allow)
	sort.Strings(li.allow)

	// TODO Expand Deny Map (to keep suggestions)

	// Split Deny Into Package Slice
	li.deny = make([]string, 0, len(l.Deny))
	for pkg := range l.Deny {
		li.deny = append(li.deny, pkg)
	}

	// Sort Deny
	sort.Strings(li.deny)

	// Populate Suggestions to match the Deny order
	li.suggestions = make([]string, 0, len(li.deny))
	for _, dp := range li.deny {
		li.suggestions = append(li.suggestions, strings.TrimSpace(l.Deny[dp]))
	}

	if len(li.allow) > 0 && len(li.deny) > 0 {
		li.listMode = lmMixed
	} else if len(li.allow) > 0 {
		li.listMode = lmAllow
	} else if len(li.deny) > 0 {
		li.listMode = lmDeny
	} else {
		return nil, errors.New("must have an Allow and/or Deny package list")
	}

	if len(errs) > 0 {
		return nil, errs
	}
	return li, nil
}

type LinterSettings map[string]*List

type linterSettings []*list

func (l LinterSettings) compile() (linterSettings, error) {
	if len(l) == 0 {
		// Only allow $gostd in all files
		set := &List{
			Allow: []string{"$gostd"},
		}
		li, err := set.compile()
		if err != nil {
			return nil, err
		}
		return linterSettings{li}, nil
	}

	li := make(linterSettings, 0, len(l))
	var errs utils.MultiError
	for name, set := range l {
		c, err := set.compile()
		if err != nil {
			errs = append(errs, err)
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
