package main

import (
	"fmt"
	"path/filepath"

	"github.com/escoubas/zk/internal/llm"
	"github.com/escoubas/zk/internal/model"
	"github.com/escoubas/zk/internal/parser"
	"github.com/escoubas/zk/internal/store"
)

// IndexAndEmbedNote parses a single note, updates the index, and generates its embedding.
func IndexAndEmbedNote(root, relPath string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("failed to resolve root: %w", err)
	}

	dbPath := filepath.Join(absRoot, ".zk", "index.db")
	st, err := store.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer st.Close()

	// 1. Parse
	p := parser.NewParser()
	note, err := p.ParseFile(absRoot, relPath)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// 2. Index (Metadata + FTS)
	if err := st.IndexNote(note); err != nil {
		return fmt.Errorf("failed to index note: %w", err)
	}

	// 3. Embed (Vector)
	// TODO: Load config for model/url? Using defaults for now.
	client := llm.NewClient("http://localhost:11434", "nomic-embed-text")
	
	// We use the raw content for embedding. 
	// Truncate if necessary (naive truncation).
	text := note.RawContent
	if len(text) > 4000 {
		text = text[:4000]
	}

	vec, err := client.Embed(text)
	if err != nil {
		// If LLM fails (e.g. offline), we shouldn't fail the whole operation, 
		// but we should probably report it.
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	emb := &model.Embedding{
		NoteID: note.ID,
		Vector: vec,
		Model:  client.Model,
	}

	if err := st.SaveEmbedding(emb); err != nil {
		return fmt.Errorf("failed to save embedding: %w", err)
	}

	return nil
}
