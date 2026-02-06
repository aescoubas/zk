package main

import "testing"

func TestNewWalkGraph(t *testing.T) {
	g := newWalkGraph("note-1", "First Note")

	if g.root == nil {
		t.Fatal("root should not be nil")
	}
	if g.current != g.root {
		t.Fatal("current should point to root")
	}
	if g.root.noteID != "note-1" {
		t.Errorf("root noteID = %q, want %q", g.root.noteID, "note-1")
	}
	if g.root.title != "First Note" {
		t.Errorf("root title = %q, want %q", g.root.title, "First Note")
	}
	if g.root.edge != edgeRoot {
		t.Errorf("root edge = %q, want %q", g.root.edge, edgeRoot)
	}
	if g.root.depth != 0 {
		t.Errorf("root depth = %d, want 0", g.root.depth)
	}
	if g.nodeTotal() != 1 {
		t.Errorf("nodeTotal = %d, want 1", g.nodeTotal())
	}
}

func TestAddChildAndBack(t *testing.T) {
	g := newWalkGraph("root", "Root")

	// Follow a link: root -> A
	g.addChild("a", "Note A", edgeOutgoing)
	if g.current.noteID != "a" {
		t.Errorf("current = %q, want %q", g.current.noteID, "a")
	}
	if g.current.depth != 1 {
		t.Errorf("depth = %d, want 1", g.current.depth)
	}
	if g.current.edge != edgeOutgoing {
		t.Errorf("edge = %q, want %q", g.current.edge, edgeOutgoing)
	}
	if g.nodeTotal() != 2 {
		t.Errorf("nodeTotal = %d, want 2", g.nodeTotal())
	}

	// Follow another link: A -> B
	g.addChild("b", "Note B", edgeBacklink)
	if g.current.noteID != "b" {
		t.Errorf("current = %q, want %q", g.current.noteID, "b")
	}
	if g.current.depth != 2 {
		t.Errorf("depth = %d, want 2", g.current.depth)
	}

	// Back to A
	ok := g.back()
	if !ok {
		t.Fatal("back() should return true")
	}
	if g.current.noteID != "a" {
		t.Errorf("after back, current = %q, want %q", g.current.noteID, "a")
	}

	// Back to root
	ok = g.back()
	if !ok {
		t.Fatal("back() should return true")
	}
	if g.current.noteID != "root" {
		t.Errorf("after back, current = %q, want %q", g.current.noteID, "root")
	}

	// Can't go back further
	ok = g.back()
	if ok {
		t.Fatal("back() at root should return false")
	}
}

func TestBranching(t *testing.T) {
	// Simulate: root -> A -> B, then back to A, then A -> D
	g := newWalkGraph("root", "Root")

	g.addChild("a", "Note A", edgeOutgoing)
	g.addChild("b", "Note B", edgeOutgoing)

	// Back to A
	g.back()
	if g.current.noteID != "a" {
		t.Fatalf("current = %q, want %q", g.current.noteID, "a")
	}

	// Branch: follow a different link from A
	g.addChild("d", "Note D", edgeSimilar)
	if g.current.noteID != "d" {
		t.Errorf("current = %q, want %q", g.current.noteID, "d")
	}

	// Verify A has two children (B and D)
	nodeA := g.root.children[0]
	if len(nodeA.children) != 2 {
		t.Errorf("A children count = %d, want 2", len(nodeA.children))
	}
	if nodeA.children[0].noteID != "b" {
		t.Errorf("A child[0] = %q, want %q", nodeA.children[0].noteID, "b")
	}
	if nodeA.children[1].noteID != "d" {
		t.Errorf("A child[1] = %q, want %q", nodeA.children[1].noteID, "d")
	}

	// Stats
	if g.nodeTotal() != 4 {
		t.Errorf("nodeTotal = %d, want 4", g.nodeTotal())
	}
	if g.branchCount() != 1 {
		t.Errorf("branchCount = %d, want 1", g.branchCount())
	}
	if g.maxDepth() != 2 {
		t.Errorf("maxDepth = %d, want 2", g.maxDepth())
	}
}

func TestJumpTo(t *testing.T) {
	g := newWalkGraph("root", "Root")
	g.addChild("a", "Note A", edgeOutgoing) // id=1
	g.addChild("b", "Note B", edgeOutgoing) // id=2

	// Jump back to root (id=0) without creating edges
	noteID := g.jumpTo(0)
	if noteID != "root" {
		t.Errorf("jumpTo(0) = %q, want %q", noteID, "root")
	}
	if g.current.noteID != "root" {
		t.Errorf("current = %q, want %q", g.current.noteID, "root")
	}
	// Node count should not change (no new edges)
	if g.nodeTotal() != 3 {
		t.Errorf("nodeTotal = %d, want 3 (unchanged)", g.nodeTotal())
	}

	// Jump to B (id=2)
	noteID = g.jumpTo(2)
	if noteID != "b" {
		t.Errorf("jumpTo(2) = %q, want %q", noteID, "b")
	}

	// Jump to non-existent node
	noteID = g.jumpTo(99)
	if noteID != "" {
		t.Errorf("jumpTo(99) = %q, want empty string", noteID)
	}
}

func TestDfsOrder(t *testing.T) {
	// Build tree: root -> A -> B, root -> C
	g := newWalkGraph("root", "Root")
	g.addChild("a", "A", edgeOutgoing) // id=1
	g.addChild("b", "B", edgeOutgoing) // id=2
	g.back()                            // back to A
	g.back()                            // back to root
	g.addChild("c", "C", edgeBacklink) // id=3

	nodes := g.dfsOrder()
	if len(nodes) != 4 {
		t.Fatalf("dfsOrder len = %d, want 4", len(nodes))
	}

	expected := []string{"root", "a", "b", "c"}
	for i, n := range nodes {
		if n.noteID != expected[i] {
			t.Errorf("dfsOrder[%d] = %q, want %q", i, n.noteID, expected[i])
		}
	}
}

func TestMaxDepthDeep(t *testing.T) {
	g := newWalkGraph("root", "Root")
	g.addChild("a", "A", edgeOutgoing)
	g.addChild("b", "B", edgeOutgoing)
	g.addChild("c", "C", edgeOutgoing)
	g.addChild("d", "D", edgeOutgoing)

	if g.maxDepth() != 4 {
		t.Errorf("maxDepth = %d, want 4", g.maxDepth())
	}
}

func TestBranchCountMultiple(t *testing.T) {
	// root -> A, root -> B (root is a branch point)
	// A -> C, A -> D (A is a branch point)
	g := newWalkGraph("root", "Root")
	g.addChild("a", "A", edgeOutgoing)
	g.addChild("c", "C", edgeOutgoing)
	g.back() // back to A
	g.addChild("d", "D", edgeOutgoing)
	g.back() // back to A
	g.back() // back to root
	g.addChild("b", "B", edgeOutgoing)

	if g.branchCount() != 2 {
		t.Errorf("branchCount = %d, want 2", g.branchCount())
	}
}
