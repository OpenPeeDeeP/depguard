package depguard

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
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

type originType uint8

const (
	originExplicit originType = iota
	originVariable
)

type list struct {
	listMode    listMode
	name        string
	files       []*glob.Pattern
	negFiles    []*glob.Pattern
	tree        *utils.CompressedPrefixTree
	suggestions []string
	anyAllows   bool // only for backwards compatible original lists
	rules       []rule
}

func (l *List) compile() (*list, error) {
	if l == nil {
		return nil, nil
	}
	li := &list{}
	var errs utils.MultiError

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
		entries, fromVar, err := utils.PathExpandable.Expand(f)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !fromVar {
			entries = []string{f}
		}
		for _, exp := range entries {
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

	type keyedRule struct {
		key    string
		origin originType
		rule   rule
	}

	var parsed []keyedRule

	// Parse the Allow list
	for _, allow := range l.Allow {
		entries, fromVar, err := utils.PackageExpandable.Expand(allow)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		origin := originExplicit
		if !fromVar {
			entries = []string{allow}
		} else {
			origin = originVariable
		}
		for _, e := range entries {
			k, r, err := parseRule(e, ruleAllow)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			parsed = append(parsed, keyedRule{k, origin, r})
			li.anyAllows = true
		}
	}

	// Parse the Deny list
	for deny, sug := range l.Deny {
		sugIdx := int32(len(li.suggestions))
		li.suggestions = append(li.suggestions, strings.TrimSpace(sug))
		entries, fromVar, err := utils.PackageExpandable.Expand(deny)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		origin := originExplicit
		if !fromVar {
			entries = []string{deny}
		} else {
			origin = originVariable
		}
		for _, e := range entries {
			k, r, err := parseRule(e, ruleDeny)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			r.sug = sugIdx
			parsed = append(parsed, keyedRule{k, origin, r})
		}
	}

	// Need to sort so rules are contiguous
	slices.SortFunc(parsed, func(left, right keyedRule) int {
		return cmp.Or(
			strings.Compare(left.key, right.key),
			cmp.Compare(left.rule.kind, right.rule.kind),
			cmp.Compare(left.rule.length, right.rule.length),
			cmp.Compare(left.origin, right.origin),
		)
	})

	// don't want duplicated rules (first wins)
	// Sort above is important to ensure explicit suggestions are used rather than inherited ones
	parsed = slices.CompactFunc(parsed, func(left, right keyedRule) bool {
		return left.key == right.key && left.rule.kind == right.rule.kind && left.rule.length == right.rule.length
	})

	li.rules = make([]rule, len(parsed))
	var start uint32 = 0
	for i, p := range parsed {
		li.rules[i] = p.rule
		// Grouping logic used when a node may have more than 1 rule
		if i+1 == len(parsed) || parsed[i+1].key != p.key {
			li.tree.Insert(p.key, start, uint32(i+1))
			start = uint32(i + 1)
		}
	}

	if len(li.rules) == 0 {
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
	aLen, dLen, dSug := int32(-1), int32(-1), int32(-1)
	l.tree.Walk(imp, func(start, end uint32, atEnd bool) {
		for _, r := range l.rules[start:end] {
			if r.exact && !atEnd {
				continue
			}
			switch r.kind {
			case ruleAllow:
				aLen = max(aLen, r.length)
			case ruleDeny:
				if r.length > dLen {
					dLen = r.length
					dSug = r.sug
				}
			}
		}
	})
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
	if !allowed && inDenied {
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

type ruleKind uint8

func (rk ruleKind) String() string {
	switch rk {
	case ruleAllow:
		return "allow"
	case ruleDeny:
		return "deny"
	}
	return ""
}

const (
	ruleAllow ruleKind = iota
	ruleDeny
)

type rule struct {
	kind   ruleKind
	exact  bool
	length int32
	sug    int32
}

func parseRule(str string, kind ruleKind) (string, rule, error) {
	if str == "" {
		return "", rule{}, fmt.Errorf("%s contains an empty package", kind)
	}
	s, isExact := strings.CutSuffix(str, "$")
	l := len(s)
	if isExact {
		if l == 0 {
			return "", rule{}, fmt.Errorf("%s contains a package with only $", kind)
		}
		l += 1
	}
	return s, rule{
		kind:   kind,
		exact:  isExact,
		length: int32(l),
		sug:    -1,
	}, nil
}
