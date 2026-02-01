package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

var (
	rootDir string
)

var rootCmd = &cobra.Command{
	Use:   "zk",
	Short: "Zettelkasten CLI tool",
	Long:  `A comprehensive tool for managing your Zettelkasten knowledge base.`,
	Run: func(cmd *cobra.Command, args []string) {
		runNavigator()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flag for root directory
	defaultDir := "."
	if envDir := os.Getenv("ZK_PATH"); envDir != "" {
		defaultDir = envDir
	}
	rootCmd.PersistentFlags().StringVar(&rootDir, "dir", defaultDir, "Root directory of the Zettelkasten")
}

func runNavigator() {
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

	fmt.Fprintf(os.Stderr, "Found %d notes.\n", len(notes))

	selected, err := RunSelector(notes, "Zettelkasten Navigator")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	if selected != nil {
		// Open in EDITOR
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim"
		}
		
		fullPath := filepath.Join(absRoot, selected.Path)
		cmd := exec.Command(editor, fullPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error opening editor: %v\n", err)
		}
	}
}