package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/escoubas/zk/internal/lsp"
	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

var lspCmd = &cobra.Command{
	Use:   "lsp",
	Short: "Start the Zettelkasten Language Server",
	Run: func(cmd *cobra.Command, args []string) {
		runLSP()
	},
}

func init() {
	rootCmd.AddCommand(lspCmd)
}

func runLSP() {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	dbPath := filepath.Join(absRoot, ".zk", "index.db")

	// Ensure DB exists or warn? LSP usually needs the index.
	// We'll open it in ReadOnly mode mostly, but store.NewStore opens it normally.
	st, err := store.NewStore(dbPath)
	if err != nil {
		// Log to stderr, as stdout is used for LSP communication
		fmt.Fprintf(os.Stderr, "Error opening DB: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	server := lsp.NewServer(st, absRoot)
	if err := server.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "LSP Server error: %v\n", err)
		os.Exit(1)
	}
}
