package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/escoubas/zk/internal/llm"
	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

var debugSemanticCmd = &cobra.Command{
	Use:   "debug-semantic [query]",
	Short: "Debug semantic search for a query",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runDebugSemantic(args[0])
	},
}

func init() {
	rootCmd.AddCommand(debugSemanticCmd)
}

func runDebugSemantic(query string) {
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

	client := llm.NewClient("http://localhost:11434", "nomic-embed-text")
	
	fmt.Printf("Generating embedding for query: '%s'\n", query)
	queryVec, err := client.Embed(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error embedding query: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Searching for similar notes (threshold 0.15)...")
	results, err := st.SearchByVector(queryVec, 20)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error searching: %v\n", err)
		os.Exit(1)
	}

	if len(results) == 0 {
		fmt.Println("No matches found.")
	} else {
		for _, r := range results {
			fmt.Printf("[%.4f] %s (%s)\n", r.Score, r.Note.Title, r.Note.ID)
		}
	}

	// Also check specifically against the test note if we can find it
	// We scan all embeddings to find the one for the test note
	fmt.Println("\nChecking manual similarity against all notes...")
	allEmbs, _ := st.GetAllEmbeddings()
	for _, e := range allEmbs {
		score := llm.CosineSimilarity(queryVec, e.Vector)
		// Print if score is decent or if ID looks like the test note
		if score > 0.1 || strings.Contains(e.NoteID, "test") {
			n, _ := st.GetNote(e.NoteID)
			title := e.NoteID
			if n != nil {
				title = n.Title
			}
			fmt.Printf(" - %s (%s): %.4f\n", title, e.NoteID, score)
		}
	}
}
