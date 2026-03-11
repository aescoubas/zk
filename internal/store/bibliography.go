package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/escoubas/zk/internal/model"
)

const bibliographyFileVersion = 1

type bibliographyFile struct {
	Version int         `json:"version"`
	Refs    []model.Ref `json:"refs"`
}

func (s *Store) loadBibliography() (bibliographyFile, error) {
	data, err := os.ReadFile(s.bibliographyPath)
	if errors.Is(err, os.ErrNotExist) {
		return bibliographyFile{Version: bibliographyFileVersion, Refs: []model.Ref{}}, nil
	}
	if err != nil {
		return bibliographyFile{}, err
	}

	var file bibliographyFile
	if err := json.Unmarshal(data, &file); err != nil {
		return bibliographyFile{}, err
	}
	if file.Version == 0 {
		file.Version = bibliographyFileVersion
	}
	file.Refs = normalizeRefs(file.Refs)
	return file, nil
}

func (s *Store) saveBibliography(file bibliographyFile) error {
	file.Version = bibliographyFileVersion
	file.Refs = normalizeRefs(file.Refs)

	if err := os.MkdirAll(filepath.Dir(s.bibliographyPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmpPath := s.bibliographyPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.bibliographyPath)
}

func normalizeRefs(refs []model.Ref) []model.Ref {
	byID := make(map[string]model.Ref, len(refs))
	for _, ref := range refs {
		if ref.ID == "" {
			continue
		}
		byID[ref.ID] = ref
	}

	out := make([]model.Ref, 0, len(byID))
	for _, ref := range byID {
		out = append(out, ref)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Title == out[j].Title {
			return out[i].ID < out[j].ID
		}
		return out[i].Title < out[j].Title
	})
	return out
}
