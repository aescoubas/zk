package main

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

var (
	listOrphans bool
	listSort    string
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
	listCmd.Flags().BoolVar(&listOrphans, "orphans", false, "Show only notes with no backlinks")
	listCmd.Flags().StringVar(&listSort, "sort", "modified", "Sort by: modified, created, title")
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
	fmt.Fprintln(w, "CREATED\tMODIFIED\tIN\tOUT\tTITLE")

	count := 0
	for _, s := range summaries {
		// Filter orphans
		if listOrphans && s.Backlinks > 0 {
			continue
		}

		created := parseCreationDate(s.ID)
		mod := s.ModTime.Format("2006-01-02")
		
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\n", created, mod, s.Backlinks, s.OutgoingLinks, s.Title)
		count++
	}
	w.Flush()
	
	if count == 0 {
		if listOrphans {
			fmt.Println("No orphan notes found.")
		} else {
			fmt.Println("No notes found.")
		}
	}
}

func parseCreationDate(id string) string {
	// ID format: YYYYMMDDHHMM...
	if len(id) >= 8 {
		if id[0] >= '0' && id[0] <= '9' {
			return fmt.Sprintf("%s-%s-%s", id[0:4], id[4:6], id[6:8])
		}
	}
	return "-"
}
