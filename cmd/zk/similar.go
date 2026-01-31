package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/escoubas/zk/internal/llm"
	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

var similarCmd = &cobra.Command{
	Use:   "similar [note_id]",
	Short: "Find semantically similar notes",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runSimilar(args[0])
	},
}

func init() {
	rootCmd.AddCommand(similarCmd)
}

type match struct {
	ID    string
	Score float64
	Title string
}

func runSimilar(targetID string) {
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

	// 1. Get Target Embedding
	targetEmb, err := st.GetEmbedding(targetID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting embedding for %s: %v\n", targetID, err)
		fmt.Println("Try running 'zk embed' first.")
		os.Exit(1)
	}

	// 2. Get All Embeddings
	allEmbs, err := st.GetAllEmbeddings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading embeddings: %v\n", err)
		os.Exit(1)
	}

	// 3. Compute Similarity
	var matches []match
	for _, e := range allEmbs {
		if e.NoteID == targetID {
			continue
		}
		score := llm.CosineSimilarity(targetEmb.Vector, e.Vector)
		matches = append(matches, match{ID: e.NoteID, Score: score})
	}

	// 4. Sort
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	// 5. Print Top 10
	fmt.Printf("Notes similar to '%s':\n", targetID)
	limit := 10
	if len(matches) < limit {
		limit = len(matches)
	}

	for i := 0; i < limit; i++ {
		m := matches[i]
		// Fetch title for nice display
		n, err := st.GetNote(m.ID)
		title := m.ID
		if err == nil {
			title = n.Title
		}
		fmt.Printf("%.4f  %s (%s)\n", m.Score, title, m.ID)
	}
}
