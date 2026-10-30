package utils

type CompressedPrefixTree struct {
	root *compressedPrefixTreeNode
}

func NewCompressedPrefixTree() *CompressedPrefixTree {
	return &CompressedPrefixTree{
		root: &compressedPrefixTreeNode{},
	}
}

func (cpt *CompressedPrefixTree) Insert(str string, start, end uint32) {
	// this is the root
	if len(str) == 0 {
		if cpt.root.spanSet() {
			panic("duplicate was inserted!")
		}
		cpt.root.start = start
		cpt.root.end = end
		return
	}
	node := cpt.root
	i := 0
	for i < len(str) {
		key := str[i]
		child, exists := node.child(key)
		// No child with key, create one
		if !exists {
			node.children = append(node.children, &compressedPrefixTreeNode{prefix: str[i:], start: start, end: end})
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
			*node = compressedPrefixTreeNode{
				prefix:   node.prefix[:prefixLength],
				children: []*compressedPrefixTreeNode{&newChild},
			}
		}

		// This current string is matching this node update flags to current strings settings!
		if i == len(str) {
			if node.spanSet() {
				panic("duplicate was inserted!")
			}
			node.start = start
			node.end = end
		}
	}
}

func (cpt *CompressedPrefixTree) Walk(str string, fn func(start, end uint32, atEnd bool)) {
	node := cpt.root
	if node.spanSet() {
		fn(node.start, node.end, len(str) == 0)
	}
	for i := 0; i < len(str); {
		child, ok := node.child(str[i])
		if !ok {
			// No children to go to we are done
			return
		}

		prefixLength := commonPrefixLength(str[i:], child.prefix)
		if prefixLength < len(child.prefix) {
			// The child that was found differs from the str we are searching so we end.
			return
		}
		i += prefixLength
		node = child
		if node.spanSet() {
			// This node has a span so we need to check it
			fn(node.start, node.end, i == len(str))
		}
	}
}

type compressedPrefixTreeNode struct {
	prefix     string
	start, end uint32
	children   []*compressedPrefixTreeNode
}

func (cptn *compressedPrefixTreeNode) child(key byte) (*compressedPrefixTreeNode, bool) {
	for _, child := range cptn.children {
		if child.prefix[0] == key {
			return child, true
		}
	}
	return nil, false
}

func (cptn *compressedPrefixTreeNode) spanSet() bool {
	return cptn.start != cptn.end
}

func commonPrefixLength(a, b string) int {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return i
}
