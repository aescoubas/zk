package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var dumpCmd = &cobra.Command{
	Use:   "dump [text...]",
	Short: "Quickly dump a thought",
	Long:  `Creates a new timestamped note in permanent_notes/. If text is provided, it is saved directly. Otherwise, opens the default editor.`, 
	Run: func(cmd *cobra.Command, args []string) {
		runDump(args)
	},
}

func init() {
	rootCmd.AddCommand(dumpCmd)
}

func runDump(args []string) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving root: %v\n", err)
		os.Exit(1)
	}

	// Dump directly to permanent_notes
	// The concept is: Everything is a note. Structure comes from linking.
	targetDir := filepath.Join(absRoot, "permanent_notes")
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		// Fallback to root if permanent_notes doesn't exist
		targetDir = absRoot
	}

	timestamp := time.Now().Format("20060102150405")
	filename := fmt.Sprintf("%s.md", timestamp)
	path := filepath.Join(targetDir, filename)

	// Case 1: Arguments provided -> Write directly
	if len(args) > 0 {
		content := strings.Join(args, " ") + "\n"
		
		// Add ID to content for tracking? 
		// "Flatter structure" usually implies frontmatter, but for a "dump" 
		// we might want just raw text. 
		// However, to be a "good citizen" in the ZK, let's add minimal header.
		fileContent := fmt.Sprintf("---\nId: %s\nDate: %s\n---\n\n%s", 
			timestamp, time.Now().Format("2006-01-02"), content)

		if err := os.WriteFile(path, []byte(fileContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write dump: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Dump saved to %s\n", path)
		return
	}

	// Case 2: No arguments -> Open editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	// Pre-fill with minimal template
	initialContent := fmt.Sprintf("---\nId: %s\nDate: %s\n---\n\n", 
		timestamp, time.Now().Format("2006-01-02"))

	if err := os.WriteFile(path, []byte(initialContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init dump file: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Move cursor to end of file (for vim)
	if strings.Contains(editor, "vim") {
		cmd.Args = append(cmd.Args, "+") 
	}

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error opening editor: %v\n", err)
		// Cleanup if editor failed
		os.Remove(path)
		os.Exit(1)
	}

	// Check if file is effectively empty (just the header)
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}
	
	if strings.TrimSpace(string(content)) == strings.TrimSpace(initialContent) {
		os.Remove(path)
		fmt.Println("Empty dump discarded.")
	} else {
		fmt.Printf("Dump saved to %s\n", path)
	}
}