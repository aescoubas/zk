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

	// Load all existing embedding metadata in one query for fast cache checks.
	existingMeta, err := st.GetAllEmbeddingMeta()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading embedding metadata: %v\n", err)
		os.Exit(1)
	}

	client := llm.NewClient(ollamaURL, embedModel)
	fmt.Printf("Using Ollama at %s with model %s\n", ollamaURL, embedModel)

	// Build list of notes that need embedding.
	var toEmbed []*model.Note
	skipped := 0
	for _, n := range notes {
		if !forceReembed {
			if existing, ok := existingMeta[n.ID]; ok && existing.Model == embedModel && existing.Hash == n.Hash {
				skipped++
				continue
			}
		}
		toEmbed = append(toEmbed, n)
	}

	fmt.Printf("Notes: %d total, %d cached, %d to embed\n", len(notes), skipped, len(toEmbed))

	count := 0
	for i, n := range toEmbed {
		fullPath := filepath.Join(absRoot, n.Path)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", n.Path, err)
			continue
		}

		fmt.Printf("[%d/%d] Embedding %s...\n", i+1, len(toEmbed), n.ID)

		text := string(content)
		// Truncate to avoid context limit (aggressive limit 2000 chars)
		if len(text) > 2000 {
			text = text[:2000]
		}

		vec, err := client.Embed(text)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to embed %s: %v\n", n.ID, err)
			continue
		}

		emb := &model.Embedding{
			NoteID: n.ID,
			Vector: vec,
			Model:  embedModel,
			Hash:   n.Hash,
		}

		if err := st.SaveEmbedding(emb); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save embedding for %s: %v\n", n.ID, err)
		}
		count++
	}

	fmt.Printf("Finished. Generated %d new embeddings.\n", count)
}
