package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

var editCmd = &cobra.Command{
	Use:   "edit [query]",
	Short: "Find and edit a note",
	Long:  `Fuzzy search for a note by title or ID and open it in the editor.`, 
	Run: func(cmd *cobra.Command, args []string) {
		runEdit(args)
	},
}

func init() {
	rootCmd.AddCommand(editCmd)
}

func runEdit(args []string) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving root: %v\n", err)
		os.Exit(1)
	}
	dbPath := filepath.Join(absRoot, ".zk", "index.db")

	st, err := store.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening DB (run 'zk index' first): %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	// 1. Get all notes
	notes, err := st.ListNotes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing notes: %v\n", err)
		os.Exit(1)
	}

	if len(notes) == 0 {
		fmt.Println("No notes found.")
		return
	}

	// 2. Select note
	// If query is provided in args, try to find exact match first? 
	// Or just pre-filter? For now, just use the fuzzy selector which is powerful.
	// Users can just type their query in the TUI.
	
	// If args provided, we might want to use them to pre-filter or auto-select.
	// But RunSelector doesn't support pre-filtering yet.
	// Let's stick to the TUI selector.
	
	selected, err := RunSelector(notes, "Select Note to Edit")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	if selected == nil {
		fmt.Println("No note selected.")
		return
	}

	// 3. Open in Editor
	fullPath := filepath.Join(absRoot, selected.Path)
	
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	cmd := exec.Command(editor, fullPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Move cursor to end of file (for vim)
	if strings.Contains(editor, "vim") {
		cmd.Args = append(cmd.Args, "+") 
	}

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error opening editor: %v\n", err)
		os.Exit(1)
	}
}
