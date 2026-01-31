package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all notes",
	Run: func(cmd *cobra.Command, args []string) {
		runList()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList() {
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

	for _, n := range notes {
		fmt.Printf("%s\t%s\n", n.ID, n.Title)
	}
}
