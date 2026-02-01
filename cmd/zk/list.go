package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

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

	summaries, err := st.ListNoteSummaries()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing notes: %v\n", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "CREATED\tMODIFIED\tIN\tOUT\tTOPIC\tTITLE")

	for _, s := range summaries {
		created := parseCreationDate(s.ID)
		mod := s.ModTime.Format("2006-01-02")
		topic := strings.Join(s.Tags, ", ")
		
		// Truncate topic if too long
		if len(topic) > 30 {
			topic = topic[:27] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\t%s\n", created, mod, s.Backlinks, s.OutgoingLinks, topic, s.Title)
	}
	w.Flush()
}

func parseCreationDate(id string) string {
	// ID format: YYYYMMDDHHMM-slug
	if len(id) >= 8 {
		// Check if it starts with digit
		if id[0] >= '0' && id[0] <= '9' {
			// Basic heuristic: try to format YYYY-MM-DD
			return fmt.Sprintf("%s-%s-%s", id[0:4], id[4:6], id[6:8])
		}
	}
	return "-"
}