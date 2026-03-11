package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/escoubas/zk/internal/model"
	"github.com/escoubas/zk/internal/parser"
)

func loadZettels(root string) ([]*model.Note, error) {
	zettelsDir := filepath.Join(root, "zettels")
	entries, err := os.ReadDir(zettelsDir)
	if err != nil {
		return nil, fmt.Errorf("read zettels directory: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	p := parser.NewParser()
	notes := make([]*model.Note, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		relPath := filepath.Join("zettels", entry.Name())
		note, err := p.ParseFile(root, relPath)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", relPath, err)
		}
		notes = append(notes, note)
	}

	return notes, nil
}
