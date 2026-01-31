package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "zk_parser_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file
	content := `---
title: Test Note
tags: [tag1, tag2]
---

# Header

This is a [[linked-note]] and an [[alias|Aliased Link]].
`
	filename := "202301010000-test-note.md"
	filePath := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Parse
	p := NewParser()
	note, err := p.ParseFile(tmpDir, filename)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Verify
	if note.ID != "202301010000-test-note" {
		t.Errorf("expected ID '202301010000-test-note', got '%s'", note.ID)
	}
	if note.Title != "Test Note" {
		t.Errorf("expected Title 'Test Note', got '%s'", note.Title)
	}
	if len(note.Metadata.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(note.Metadata.Tags))
	}
	if note.Metadata.Tags[0] != "tag1" {
		t.Errorf("expected tag1, got %s", note.Metadata.Tags[0])
	}
	
	// Check Links
	if len(note.OutgoingLinks) != 2 {
		t.Errorf("expected 2 links, got %d", len(note.OutgoingLinks))
	}
	// Note: ExtractLinks logic might return them in order found.
	// [[linked-note]]
	if note.OutgoingLinks[0].TargetID != "linked-note" {
		t.Errorf("expected first link target 'linked-note', got '%s'", note.OutgoingLinks[0].TargetID)
	}
	// [[alias|Aliased Link]]
	if note.OutgoingLinks[1].TargetID != "alias" {
		t.Errorf("expected second link target 'alias', got '%s'", note.OutgoingLinks[1].TargetID)
	}
	if note.OutgoingLinks[1].DisplayText != "Aliased Link" {
		t.Errorf("expected second link display 'Aliased Link', got '%s'", note.OutgoingLinks[1].DisplayText)
	}
}
