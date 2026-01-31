package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

var linkCmd = &cobra.Command{
	Use:   "link",
	Short: "Search and link notes",
	Run: func(cmd *cobra.Command, args []string) {
		runLink()
	},
}

func init() {
	rootCmd.AddCommand(linkCmd)
}

func runLink() {
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
	defer st.Close()

	notes, err := st.ListNotes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing notes: %v\n", err)
		os.Exit(1)
	}

	selected, err := RunSelector(notes, "Select Note to Link")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	if selected != nil {
		fmt.Printf("[[%s]]", selected.ID)
	}
}