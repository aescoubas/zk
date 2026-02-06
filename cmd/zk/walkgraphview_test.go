package main

import "testing"

func TestBuildTreeLinesSingle(t *testing.T) {
	g := newWalkGraph("root", "Root Note")
	lines := buildTreeLines(g)

	if len(lines) != 1 {
		t.Fatalf("len = %d, want 1", len(lines))
	}
	if lines[0].prefix != "" {
		t.Errorf("root prefix = %q, want empty", lines[0].prefix)
	}
	if lines[0].node.noteID != "root" {
		t.Errorf("root noteID = %q, want %q", lines[0].node.noteID, "root")
	}
}

func TestBuildTreeLinesLinear(t *testing.T) {
	// root -> A -> B
	g := newWalkGraph("root", "Root")
	g.addChild("a", "A", edgeOutgoing)
	g.addChild("b", "B", edgeBacklink)

	lines := buildTreeLines(g)
	if len(lines) != 3 {
		t.Fatalf("len = %d, want 3", len(lines))
	}

	// Root has no prefix
	if lines[0].prefix != "" {
		t.Errorf("lines[0] prefix = %q, want empty", lines[0].prefix)
	}
	// A is last child of root → └──
	if lines[1].prefix != "└── " {
		t.Errorf("lines[1] prefix = %q, want %q", lines[1].prefix, "└── ")
	}
	// B is last child of A → "    └── "
	if lines[2].prefix != "    └── " {
		t.Errorf("lines[2] prefix = %q, want %q", lines[2].prefix, "    └── ")
	}
}

func TestBuildTreeLinesBranching(t *testing.T) {
	// root -> A, root -> B
	g := newWalkGraph("root", "Root")
	g.addChild("a", "A", edgeOutgoing)
	g.back()
	g.addChild("b", "B", edgeSimilar)

	lines := buildTreeLines(g)
	if len(lines) != 3 {
		t.Fatalf("len = %d, want 3", len(lines))
	}

	// A is NOT last child → ├──
	if lines[1].prefix != "├── " {
		t.Errorf("lines[1] prefix = %q, want %q", lines[1].prefix, "├── ")
	}
	// B is last child → └──
	if lines[2].prefix != "└── " {
		t.Errorf("lines[2] prefix = %q, want %q", lines[2].prefix, "└── ")
	}
}

func TestBuildTreeLinesDeep(t *testing.T) {
	// root -> A -> C, root -> B
	g := newWalkGraph("root", "Root")
	g.addChild("a", "A", edgeOutgoing) // child 0
	g.addChild("c", "C", edgeOutgoing)
	g.back() // back to A
	g.back() // back to root
	g.addChild("b", "B", edgeSimilar) // child 1

	lines := buildTreeLines(g)
	if len(lines) != 4 {
		t.Fatalf("len = %d, want 4", len(lines))
	}

	// root
	if lines[0].node.noteID != "root" {
		t.Errorf("lines[0] = %q", lines[0].node.noteID)
	}
	// A (not last child of root)
	if lines[1].prefix != "├── " {
		t.Errorf("lines[1] prefix = %q, want %q", lines[1].prefix, "├── ")
	}
	// C (last child of A, under non-last branch)
	if lines[2].prefix != "│   └── " {
		t.Errorf("lines[2] prefix = %q, want %q", lines[2].prefix, "│   └── ")
	}
	// B (last child of root)
	if lines[3].prefix != "└── " {
		t.Errorf("lines[3] prefix = %q, want %q", lines[3].prefix, "└── ")
	}
}

func TestTruncateTitle(t *testing.T) {
	tests := []struct {
		input  string
		max    int
		expect string
	}{
		{"Short", 30, "Short"},
		{"Exactly thirty characters long!", 30, "Exactly thirty characters l..."},
		{"A very long title that goes way beyond the limit", 30, "A very long title that goes..."},
		{"AB", 2, "AB"},
		{"ABC", 3, "ABC"},
		{"ABCD", 3, "ABC"},
	}

	for _, tt := range tests {
		got := truncateTitle(tt.input, tt.max)
		if got != tt.expect {
			t.Errorf("truncateTitle(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.expect)
		}
	}
}

func TestEdgeIndicator(t *testing.T) {
	tests := []struct {
		edge   edgeLabel
		expect string
	}{
		{edgeRoot, "●"},
		{edgeOutgoing, "→"},
		{edgeBacklink, "←"},
		{edgeSimilar, "~"},
		{edgeCitation, "@"},
		{edgeLabel("unknown"), "·"},
	}

	for _, tt := range tests {
		got := edgeIndicator(tt.edge)
		if got != tt.expect {
			t.Errorf("edgeIndicator(%q) = %q, want %q", tt.edge, got, tt.expect)
		}
	}
}

func TestNewWalkGraphModelCursorOnCurrent(t *testing.T) {
	// root -> A -> B (current is B)
	g := newWalkGraph("root", "Root")
	g.addChild("a", "A", edgeOutgoing)
	g.addChild("b", "B", edgeOutgoing)
	// current is now B (index 2 in DFS order)

	m := newWalkGraphModel(g, nil, "")
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (current node B)", m.cursor)
	}
	if len(m.lines) != 3 {
		t.Errorf("lines = %d, want 3", len(m.lines))
	}
}

func TestWalkGraphModelCursorAfterBack(t *testing.T) {
	// root -> A -> B, then back to A
	g := newWalkGraph("root", "Root")
	g.addChild("a", "A", edgeOutgoing)
	g.addChild("b", "B", edgeOutgoing)
	g.back() // current is A

	m := newWalkGraphModel(g, nil, "")
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (current node A)", m.cursor)
	}
}
