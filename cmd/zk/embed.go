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
	embedBatch   int
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
	embedCmd.Flags().IntVarP(&embedBatch, "batch", "b", 50, "Number of notes per batch request")
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

	batchSize := embedBatch
	if batchSize < 1 {
		batchSize = 1
	}

	count := 0
	for i := 0; i < len(toEmbed); i += batchSize {
		end := i + batchSize
		if end > len(toEmbed) {
			end = len(toEmbed)
		}
		batch := toEmbed[i:end]

		// Read and prepare texts for this batch.
		texts := make([]string, 0, len(batch))
		valid := make([]*model.Note, 0, len(batch))
		for _, n := range batch {
			fullPath := filepath.Join(absRoot, n.Path)
			content, err := os.ReadFile(fullPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", n.Path, err)
				continue
			}
			text := string(content)
			if len(text) > 2000 {
				text = text[:2000]
			}
			texts = append(texts, text)
			valid = append(valid, n)
		}

		if len(texts) == 0 {
			continue
		}

		fmt.Printf("[%d-%d/%d] Embedding batch of %d notes...\n", i+1, i+len(valid), len(toEmbed), len(valid))

		vectors, err := client.EmbedBatch(texts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to embed batch %d-%d: %v\n", i+1, i+len(valid), err)
			continue
		}

		for j, n := range valid {
			emb := &model.Embedding{
				NoteID: n.ID,
				Vector: vectors[j],
				Model:  embedModel,
				Hash:   n.Hash,
			}
			if err := st.SaveEmbedding(emb); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to save embedding for %s: %v\n", n.ID, err)
				continue
			}
			count++
		}
	}

	fmt.Printf("Finished. Generated %d new embeddings.\n", count)
}
