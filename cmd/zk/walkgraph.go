package main

import "time"

// edgeLabel describes how a navigation step was taken.
type edgeLabel string

const (
	edgeRoot     edgeLabel = "root"
	edgeOutgoing edgeLabel = "outgoing"
	edgeBacklink edgeLabel = "backlink"
	edgeSimilar  edgeLabel = "similar"
	edgeCitation edgeLabel = "citation"
)

// walkNode represents a single visit in the navigation walk graph.
// Each node corresponds to one "visit" — the same note may appear
// multiple times if revisited via different paths.
type walkNode struct {
	id        int
	noteID    string
	title     string
	parent    *walkNode
	children  []*walkNode
	timestamp time.Time
	edge      edgeLabel
	depth     int
}

// walkGraph is a session-scoped directed tree that records the full
// navigation history through the zettelkasten, including branches
// created by backtracking and following different links.
type walkGraph struct {
	root      *walkNode
	current   *walkNode
	nodeCount int
}

// newWalkGraph creates a new walk graph rooted at the given note.
func newWalkGraph(noteID, title string) *walkGraph {
	root := &walkNode{
		id:        0,
		noteID:    noteID,
		title:     title,
		timestamp: time.Now(),
		edge:      edgeRoot,
		depth:     0,
	}
	return &walkGraph{
		root:      root,
		current:   root,
		nodeCount: 1,
	}
}

// addChild appends a new child node under the current position
// and advances the cursor to it. This is "organic" navigation.
func (g *walkGraph) addChild(noteID, title string, edge edgeLabel) {
	child := &walkNode{
		id:        g.nodeCount,
		noteID:    noteID,
		title:     title,
		parent:    g.current,
		timestamp: time.Now(),
		edge:      edge,
		depth:     g.current.depth + 1,
	}
	g.current.children = append(g.current.children, child)
	g.current = child
	g.nodeCount++
}

// back moves the cursor to the parent node.
// Returns false if already at the root.
func (g *walkGraph) back() bool {
	if g.current.parent == nil {
		return false
	}
	g.current = g.current.parent
	return true
}

// jumpTo moves the cursor to an existing node by walk-node ID
// without creating new edges. Returns the noteID of the target
// node, or empty string if the node was not found.
func (g *walkGraph) jumpTo(nodeID int) string {
	node := g.findNode(g.root, nodeID)
	if node == nil {
		return ""
	}
	g.current = node
	return node.noteID
}

// findNode recursively searches the tree for a node by walk-node ID.
func (g *walkGraph) findNode(node *walkNode, id int) *walkNode {
	if node.id == id {
		return node
	}
	for _, child := range node.children {
		if found := g.findNode(child, id); found != nil {
			return found
		}
	}
	return nil
}

// nodeTotal returns the total number of nodes in the graph.
func (g *walkGraph) nodeTotal() int {
	return g.nodeCount
}

// maxDepth returns the maximum depth reached in the graph.
func (g *walkGraph) maxDepth() int {
	return g.computeMaxDepth(g.root)
}

func (g *walkGraph) computeMaxDepth(node *walkNode) int {
	max := node.depth
	for _, child := range node.children {
		if d := g.computeMaxDepth(child); d > max {
			max = d
		}
	}
	return max
}

// branchCount returns the number of fork points (nodes with >1 child).
func (g *walkGraph) branchCount() int {
	return g.countBranches(g.root)
}

func (g *walkGraph) countBranches(node *walkNode) int {
	count := 0
	if len(node.children) > 1 {
		count = 1
	}
	for _, child := range node.children {
		count += g.countBranches(child)
	}
	return count
}

// dfsOrder returns all nodes in depth-first pre-order traversal.
func (g *walkGraph) dfsOrder() []*walkNode {
	var result []*walkNode
	g.dfsCollect(g.root, &result)
	return result
}

func (g *walkGraph) dfsCollect(node *walkNode, result *[]*walkNode) {
	*result = append(*result, node)
	for _, child := range node.children {
		g.dfsCollect(child, result)
	}
}
