package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/escoubas/zk/internal/llm"
	"github.com/escoubas/zk/internal/model"
	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

var (
	embedModel   string
	ollamaURL    string
	forceReembed bool
)

var embedCmd = &cobra.Command{
	Use:   "embed",
	Short: "Generate embeddings for notes",
	Run: func(cmd *cobra.Command, args []string) {
		runEmbed()
	},
}

func init() {
	rootCmd.AddCommand(embedCmd)
	embedCmd.Flags().StringVar(&embedModel, "model", "nomic-embed-text", "Ollama embedding model")
	embedCmd.Flags().StringVar(&ollamaURL, "url", "http://localhost:11434", "Ollama URL")
	embedCmd.Flags().BoolVarP(&forceReembed, "force", "f", false, "Force re-embedding of all notes")
}

func runEmbed() {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	dbPath := filepath.Join(absRoot, ".zk", "index.db")

	st, err := store.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening DB: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	notes, err := st.ListNotes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing notes: %v\n", err)
		os.Exit(1)
	}

	client := llm.NewClient(ollamaURL, embedModel)
	fmt.Printf("Using Ollama at %s with model %s\n", ollamaURL, embedModel)

	count := 0
	for i, n := range notes {
		// Check if exists
		if !forceReembed {
			existing, _ := st.GetEmbedding(n.ID)
			if existing != nil && existing.Model == embedModel {
				continue
			}
		}

		// Need full content. ListNotes only gives minimal info?
		// Check internal/store/store.go ListNotes -> SELECT id, path, title, hash, mod_time
		// Content is NOT in ListNotes.
		// Need to read file or use GetNote (which doesn't return content either based on current store.go?)
		// store.go GetNote -> id, path, title, hash, mod_time
		// RawContent field exists in model.Note but is not populated by Store methods unless FTS?
		// Wait, parser populates it.
		// I need to read the file.
		
		fullPath := filepath.Join(absRoot, n.Path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", n.Path, err)
			continue
		}

		fmt.Printf("[%d/%d] Embedding %s...\n", i+1, len(notes), n.ID)
		
		text := string(content)
		// Truncate to avoid context limit (aggressive limit 2000 chars)
		if len(text) > 2000 {
			text = text[:2000]
		}
		// fmt.Printf("Embedding %d chars...\n", len(text))

		vec, err := client.Embed(text)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to embed %s: %v\n", n.ID, err)
			continue
		}

		emb := &model.Embedding{
			NoteID: n.ID,
			Vector: vec,
			Model:  embedModel,
		}
		
		if err := st.SaveEmbedding(emb); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save embedding for %s: %v\n", n.ID, err)
		}
		count++
	}

	fmt.Printf("Finished. Generated %d embeddings.\n", count)
}
