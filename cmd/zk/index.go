package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/escoubas/zk/internal/parser"
	"github.com/escoubas/zk/internal/store"
	"github.com/escoubas/zk/internal/watcher"
	"github.com/spf13/cobra"
)

var watchMode bool

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Index the Zettelkasten",
	Long:  `Scans the Zettelkasten directory, parses Markdown files, and updates the SQLite index.`, 
	Run: func(cmd *cobra.Command, args []string) {
		runIndex(rootDir, watchMode)
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
	indexCmd.Flags().BoolVarP(&watchMode, "watch", "w", false, "Watch for changes")
}

func runIndex(dir string, watch bool) {
	absRoot, err := filepath.Abs(dir)
	if err != nil {
		log.Fatalf("Failed to resolve root dir: %v", err)
	}

	dbPath := filepath.Join(absRoot, ".zk", "index.db")
	fmt.Printf("Zettelkasten Root: %s\n", absRoot)
	fmt.Printf("Database Path:   %s\n", dbPath)

	st, err := store.NewStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to open store: %v", err)
	}
	defer st.Close()

	p := parser.NewParser()

	count := 0
	start := time.Now()

	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip .git, .zk, and other hidden dirs
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}

		if filepath.Ext(path) != ".md" {
			return nil
		}

		// Calculate relative path
		relPath, err := filepath.Rel(absRoot, path)
		if err != nil {
			return err
		}

		// Parse
		note, err := p.ParseFile(absRoot, relPath)
		if err != nil {
			log.Printf("Failed to parse %s: %v", relPath, err)
			return nil // Continue
		}

		// Index
		if err := st.IndexNote(note); err != nil {
			log.Printf("Failed to index %s: %v", relPath, err)
			return nil
		}

		count++
		if count%100 == 0 {
			fmt.Printf("Indexed %d notes...\r", count)
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Error walking directory: %v", err)
	}

	duration := time.Since(start)
	fmt.Printf("\nDone. Indexed %d notes in %v.\n", count, duration)

	// Prune Stale Notes
	fmt.Println("Pruning stale notes...")
	allNotes, _ := st.ListNotes()
	pruned := 0
	for _, n := range allNotes {
		fullPath := filepath.Join(absRoot, n.Path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			if err := st.DeleteNote(n.ID); err == nil {
				fmt.Printf("Deleted stale note: %s\n", n.ID)
				pruned++
			}
		} else if err != nil {
            fmt.Printf("Error checking %s: %v\n", fullPath, err)
        }
	}
	if pruned > 0 {
		fmt.Printf("Pruned %d stale notes.\n", pruned)
	}

	if watch {
		fmt.Println("Watching for changes...")
		w, err := watcher.NewWatcher(absRoot, func(path string) {
			relPath, _ := filepath.Rel(absRoot, path)
			fmt.Printf("Update: %s\n", relPath)
			note, err := p.ParseFile(absRoot, relPath)
			if err != nil {
				log.Printf("Error parsing %s: %v", relPath, err)
				return
			}
			if err := st.IndexNote(note); err != nil {
				log.Printf("Error indexing %s: %v", relPath, err)
			}
		}, func(path string) {
			relPath, _ := filepath.Rel(absRoot, path)
			// ID is filename stem
			id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			fmt.Printf("Delete: %s (ID: %s)\n", relPath, id)
			if err := st.DeleteNote(id); err != nil {
				log.Printf("Error deleting %s: %v", relPath, err)
			}
		})
		if err != nil {
			log.Fatalf("Failed to create watcher: %v", err)
		}
		defer w.Close()

		if err := w.Start(); err != nil {
			log.Fatalf("Failed to start watcher: %v", err)
		}

		// Block forever
		select {}
	}
}
