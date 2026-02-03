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
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving root: %v\n", err)
		os.Exit(1)
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

`, title, time.Now().Format("2006-01-02"))

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create note: %v\n", err)
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
}

func slugify(s string) string {
	s = strings.ToLower(s)
	reg := regexp.MustCompile("[^a-z0-9]+")
	s = reg.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}