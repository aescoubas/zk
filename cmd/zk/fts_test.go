package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/escoubas/zk/internal/parser"
	"github.com/escoubas/zk/internal/store"
)

func TestFTSSearch(t *testing.T) {
	// Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "zk_fts_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Init DB
	dbPath := filepath.Join(tmpDir, ".zk", "index.db")
	st, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// 1. Create a note manually
	content := `---
title: Test FTS
---
You don't know what you are doing`

	filename := "test-note.md"
	fullPath := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Parse and Index (simulate IndexAndEmbedNote but without embedding/llm dep)
	p := parser.NewParser()
	note, err := p.ParseFile(tmpDir, filename)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	
	// Verify RawContent
	if !strings.Contains(note.RawContent, "You don't know what") {
		t.Errorf("RawContent missing text. Got: %s", note.RawContent)
	}

	if err := st.IndexNote(note); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	// 3. Search - Exact phrase match (quoted)
	// Try "you don't know what"
	query := `"you don't know what"`
	results, err := st.SearchNotes(query)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("Search quoted failed for \"you don't know what\"")
	}

	// 4. Search - Unquoted
	queryRaw := "you don't know what"
	resultsRaw, err := st.SearchNotes(queryRaw)
	if err != nil {
		t.Logf("Search unquoted error: %v", err)
	}
	if len(resultsRaw) == 0 {
		t.Errorf("Search unquoted failed for '%s'", queryRaw)
	}

	// 5. Debug - Simple words
	q2 := "you know what"
	r2, _ := st.SearchNotes(q2)
	if len(r2) == 0 {
		t.Error("Search failed for 'you know what'")
	}

	// 6. Debug - word with apostrophe only
	q3 := "don't"
	r3, _ := st.SearchNotes(q3)
	if len(r3) == 0 {
		t.Error("Search failed for 'don't'")
	}

	// 7. Debug - Parts
	q4 := "don"
	r4, _ := st.SearchNotes(q4)
	if len(r4) == 0 {
		t.Error("Search failed for 'don'")
	}

	q5 := "know"
	r5, _ := st.SearchNotes(q5)
	if len(r5) == 0 {
		t.Error("Search failed for 'know'")
	}

	q6 := "t"
	r6, _ := st.SearchNotes(q6)
	if len(r6) == 0 {
		t.Error("Search failed for 't'")
	}

	q7 := "you don t know what"
	r7, _ := st.SearchNotes(q7)
	if len(r7) == 0 {
		t.Error("Search failed for 'you don t know what'")
	}
}
