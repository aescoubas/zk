package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new [title]",
	Short: "Create a new note",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		title := strings.Join(args, " ")
		runNew(title)
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}

func runNew(title string) {
	path, err := createNoteFile(title)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating note: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(path)

	// Open in EDITOR
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error opening editor: %v\n", err)
	}

	// Index and Embed immediately
	fmt.Printf("Indexing and embedding %s...\n", path)
	
	// Re-resolve root since createNoteFile handled it internally but we need it here
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving root: %v\n", err)
		return
	}
	
	relPath, _ := filepath.Rel(absRoot, path)
	if err := IndexAndEmbedNote(absRoot, relPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	} else {
		fmt.Println("Done.")
	}
}

func createNoteFile(title string) (string, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return "", fmt.Errorf("error resolving root: %v", err)
	}

	slug := slugify(title)
	timestamp := time.Now().Format("200601021504")
	filename := fmt.Sprintf("%s-%s.md", timestamp, slug)
	
	// Create in zettels by default, or root if zettels doesn't exist
	targetDir := filepath.Join(absRoot, "zettels")
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		targetDir = absRoot
	}

	path := filepath.Join(targetDir, filename)

	content := fmt.Sprintf(`---
title: %s
date: %s
tags: []
---

# %s

Summary...

## Context / Details

## References
`, title, time.Now().Format("2006-01-02"), title)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to create note: %v", err)
	}

	return path, nil
}

func slugify(s string) string {
	s = strings.ToLower(s)
	reg := regexp.MustCompile("[^a-z0-9]+")
	s = reg.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}