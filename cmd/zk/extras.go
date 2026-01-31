package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

var randomCmd = &cobra.Command{
	Use:   "random",
	Short: "Open a random note",
	Run: func(cmd *cobra.Command, args []string) {
		runRandom()
	},
}

var staleCmd = &cobra.Command{
	Use:   "stale",
	Short: "List stale notes (> 1 year)",
	Run: func(cmd *cobra.Command, args []string) {
		runStale()
	},
}

func init() {
	rootCmd.AddCommand(randomCmd)
	rootCmd.AddCommand(staleCmd)
}

func getStore() *store.Store {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	dbPath := filepath.Join(absRoot, ".zk", "index.db")

	st, err := store.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening DB (run 'zk index' first): %v\n", err)
		os.Exit(1)
	}
	return st
}

func runRandom() {
	st := getStore()
	defer st.Close()

	note, err := st.GetRandomNote()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting random note: %v\n", err)
		os.Exit(1)
	}

	// Just print path? Or open?
	// Existing behavior: "Opens a random note"
	// Scripts usually print the path and let the caller open it, OR execute $EDITOR.
	// Since `zk new` prints path, let's print path here too.
	fmt.Println(note.Path)
}

func runStale() {
	st := getStore()
	defer st.Close()

	notes, err := st.GetStaleNotes(365 * 24 * time.Hour)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting stale notes: %v\n", err)
		os.Exit(1)
	}

	for _, n := range notes {
		fmt.Printf("%s\t%s\n", n.ModTime.Format("2006-01-02"), n.Title)
	}
}
