package utils

import (
	"strings"
)

type CompressedPrefixTree struct {
	root *compressedPrefixTreeNode
}

func NewCompressedPrefixTree() *CompressedPrefixTree {
	return &CompressedPrefixTree{
		root: newNode("", 0, -1),
	}
}

func (cpt *CompressedPrefixTree) InsertAllow(str string) {
	s, f := entryFlags(str, allowPrefix, allowExact)
	cpt.insert(s, f, -1)
}

func (cpt *CompressedPrefixTree) InsertDeny(str string, sugIdx int) {
	s, f := entryFlags(str, denyPrefix, denyExact)
	cpt.insert(s, f, sugIdx)
}

func entryFlags(str string, prefix, exact nodeFlags) (string, nodeFlags) {
	if s, ok := strings.CutSuffix(str, "$"); ok {
		return s, exact
	}
	return str, prefix
}

func (cpt *CompressedPrefixTree) insert(str string, flags nodeFlags, sugIdx int) {
	node := cpt.root
	i := 0
	for i < len(str) {
		key := str[i]
		child, exists := node.child(key)
		// No child with key, create one
		if !exists {
			node.children = append(node.children, newNode(str[i:], flags, sugIdx))
			break
		}

		// We found a child node, update variables
		node = child
		prefixLength := commonPrefixLength(str[i:], node.prefix)
		i += prefixLength

		// - prefixLength can't be greater len(node.prefix)
		// - If prefixLength equals len(node.prefix) we know that either we have the
		//   same string, or that we have to create another child node past this node.
		//   To create a new child just continue iteration with updated i from above.
		//
		// - If prefixLength is less than len(node.prefix) we have to split the node.
		if prefixLength < len(node.prefix) {
			// Create new child with remaining of the word copied from existing node
			newChild := *node
			newChild.prefix = node.prefix[prefixLength:]
			// Update existing node as nothing has gone here
			*node = *newNode(node.prefix[:prefixLength], 0, -1)
			node.children = append(node.children, &newChild)
		}

		// This current string is matching this node update flags to current strings settings!
		if i == len(str) {
			updateNode(node, flags, sugIdx)
		}
	}
}

func (cpt *CompressedPrefixTree) Search(str string) (allowLen, denyLen, sugIdx int) {
	node := cpt.root
	i := 0
	allowLen = -1
	denyLen = -1
	sugIdx = -1
	for i < len(str) {
		key := str[i]
		child, exists := node.child(key)
		// No child with key, no where to go so not found
		if !exists {
			return
		}

		// We found a child node, update variables
		node = child
		prefixLength := commonPrefixLength(str[i:], node.prefix)
		i += prefixLength

		// No possible match beyond this point
		if prefixLength < len(node.prefix) {
			return
		}

		if node.flags&allowPrefix != 0 {
			allowLen = i
		}
		if node.flags&denyPrefix != 0 {
			denyLen = i
			sugIdx = node.prefixSugIdx
		}
	}
	if node.flags&allowExact != 0 {
		allowLen = i + 1 // So that exactness is greater than prefix
	}
	if node.flags&denyExact != 0 {
		denyLen = i + 1 // So that exactness is greater than prefix
		sugIdx = node.exactSugIdx
	}
	return
}

type nodeFlags uint8

const (
	allowPrefix nodeFlags = 1 << iota
	allowExact
	denyPrefix
	denyExact
)

type compressedPrefixTreeNode struct {
	prefix       string
	flags        nodeFlags
	prefixSugIdx int
	exactSugIdx  int
	children     []*compressedPrefixTreeNode
}

func newNode(prefix string, flags nodeFlags, sugIdx int) *compressedPrefixTreeNode {
	newNode := &compressedPrefixTreeNode{
		prefix:       prefix,
		flags:        0,
		prefixSugIdx: -1,
		exactSugIdx:  -1,
	}
	updateNode(newNode, flags, sugIdx)
	return newNode
}

func updateNode(node *compressedPrefixTreeNode, flags nodeFlags, sugIdx int) {
	node.flags |= flags
	if flags&denyPrefix != 0 {
		node.prefixSugIdx = sugIdx
	}
	if flags&denyExact != 0 {
		node.exactSugIdx = sugIdx
	}
}

func (cptn *compressedPrefixTreeNode) child(key byte) (*compressedPrefixTreeNode, bool) {
	for _, child := range cptn.children {
		if child.prefix[0] == key {
			return child, true
		}
	}
	return nil, false
}

func commonPrefixLength(a, b string) int {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return i
}
