package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type deadLink struct {
	source string
	target string
}

var lintCmd = &cobra.Command{
	Use:           "lint",
	Short:         "Check zettels for dead links and orphan notes",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runLint()
	},
}

func init() {
	rootCmd.AddCommand(lintCmd)
}

func runLint() error {
	notes, err := loadZettels(rootDir)
	if err != nil {
		return err
	}

	noteIDs := make(map[string]struct{}, len(notes))
	incoming := make(map[string]int, len(notes))
	for _, note := range notes {
		noteIDs[note.ID] = struct{}{}
		incoming[note.ID] = 0
	}

	var deadLinks []deadLink
	for _, note := range notes {
		for _, link := range note.OutgoingLinks {
			target := strings.TrimSpace(link.TargetID)
			if target == "" {
				continue
			}
			if _, ok := noteIDs[target]; ok {
				incoming[target]++
				continue
			}
			deadLinks = append(deadLinks, deadLink{
				source: filepath.Base(note.Path),
				target: target,
			})
		}
	}

	fmt.Printf("Linting %d notes...\n\n", len(notes))

	if len(deadLinks) == 0 {
		fmt.Println("No dead links found.")
	} else {
		fmt.Println("--- Dead Links (Target not found) ---")
		for _, link := range deadLinks {
			fmt.Printf("In '%s': [[%s]]\n", link.source, link.target)
		}
	}

	var orphans []string
	for id, count := range incoming {
		if count == 0 {
			orphans = append(orphans, id)
		}
	}
	sort.Strings(orphans)

	if len(orphans) > 0 {
		fmt.Printf("\n--- Orphans (%d notes with 0 incoming links) ---\n", len(orphans))
		limit := len(orphans)
		if limit > 10 {
			limit = 10
		}
		for _, orphan := range orphans[:limit] {
			fmt.Printf("- %s\n", orphan)
		}
		if len(orphans) > limit {
			fmt.Printf("... and %d more.\n", len(orphans)-limit)
		}
	}

	if len(deadLinks) > 0 {
		return exitError{code: 1}
	}

	return nil
}
