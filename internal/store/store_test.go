package store

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/escoubas/zk/internal/model"
	_ "github.com/mattn/go-sqlite3"
)

func TestStore(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	dbPath := filepath.Join(root, ".zk", "index.db")

	st, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer st.Close()

	// 1. Index Note
	note := &model.Note{
		ID:         "note-1",
		Path:       "note-1.md",
		Title:      "Note 1",
		RawContent: "Content of note 1",
		Hash:       "hash1",
		ModTime:    time.Now(),
		Metadata: model.Metadata{
			Tags: []string{"tagA", "tagB"},
		},
		OutgoingLinks: []model.Link{
			{SourceID: "note-1", TargetID: "note-2", DisplayText: "Note 2"},
		},
	}

	if err := st.IndexNote(note); err != nil {
		t.Fatalf("IndexNote failed: %v", err)
	}

	ref := &model.Ref{
		ID:     "book-1",
		Type:   "book",
		Title:  "Book 1",
		Author: "Author 1",
	}
	if err := st.UpsertRef(ref); err != nil {
		t.Fatalf("UpsertRef failed: %v", err)
	}

	// 2. Get Note
	retrieved, err := st.GetNote("note-1")
	if err != nil {
		t.Fatalf("GetNote failed: %v", err)
	}
	if retrieved.Title != "Note 1" {
		t.Errorf("Expected title 'Note 1', got '%s'", retrieved.Title)
	}

	// 3. List Notes
	notes, err := st.ListNotes()
	if err != nil {
		t.Fatalf("ListNotes failed: %v", err)
	}
	if len(notes) != 1 {
		t.Errorf("Expected 1 note, got %d", len(notes))
	}

	// 4. Check SRS
	srsItem := &model.SRSItem{
		NoteID:      "note-1",
		NextReview:  time.Now().Add(24 * time.Hour),
		Interval:    1,
		EaseFactor:  2.5,
		Repetitions: 1,
	}
	if err := st.SaveSRSItem(srsItem); err != nil {
		t.Fatalf("SaveSRSItem failed: %v", err)
	}

	retrievedSRS, err := st.GetSRSItem("note-1")
	if err != nil {
		t.Fatalf("GetSRSItem failed: %v", err)
	}
	if retrievedSRS.Repetitions != 1 {
		t.Errorf("Expected 1 rep, got %d", retrievedSRS.Repetitions)
	}

	if err := st.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	indexDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer indexDB.Close()

	assertTableMissing(t, indexDB, "srs_items")
	assertTableMissing(t, indexDB, "refs")

	statePath, err := stateDBPathForRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	stateDB, err := sql.Open("sqlite3", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer stateDB.Close()
	assertTablePresent(t, stateDB, "srs_items")

	var srsCount int
	if err := stateDB.QueryRow(`SELECT count(*) FROM srs_items`).Scan(&srsCount); err != nil {
		t.Fatal(err)
	}
	if srsCount != 1 {
		t.Fatalf("expected 1 local SRS row, got %d", srsCount)
	}

	data, err := os.ReadFile(filepath.Join(root, bibliographyFileName))
	if err != nil {
		t.Fatalf("reading bibliography file: %v", err)
	}
	var bib bibliographyFile
	if err := json.Unmarshal(data, &bib); err != nil {
		t.Fatalf("unmarshal bibliography file: %v", err)
	}
	if len(bib.Refs) != 1 || bib.Refs[0].ID != ref.ID {
		t.Fatalf("expected bibliography file to contain %q, got %+v", ref.ID, bib.Refs)
	}

	st, err = NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore reopen failed: %v", err)
	}
	defer st.Close()

	// 5. Delete Note
	if err := st.DeleteNote("note-1"); err != nil {
		t.Fatalf("DeleteNote failed: %v", err)
	}

	_, err = st.GetNote("note-1")
	if err == nil {
		t.Error("Expected error getting deleted note, got nil")
	}
}

func TestNewStoreMigratesLegacyStateAndSignalsRebuild(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	dbPath := filepath.Join(root, ".zk", "index.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TABLE notes (id TEXT PRIMARY KEY, path TEXT NOT NULL, title TEXT, summary TEXT, hash TEXT, mod_time INTEGER);
		CREATE TABLE srs_items (note_id TEXT PRIMARY KEY, next_review INTEGER, interval REAL, ease_factor REAL, repetitions INTEGER);
		CREATE TABLE refs (id TEXT PRIMARY KEY, type TEXT, title TEXT, author TEXT, year TEXT, url TEXT, description TEXT);
		INSERT INTO srs_items (note_id, next_review, interval, ease_factor, repetitions) VALUES ('note-1', 123, 2, 2.5, 3);
		INSERT INTO refs (id, type, title, author) VALUES ('book-1', 'book', 'Book 1', 'Author 1');
	`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	st, err := NewStore(dbPath)
	if st != nil {
		st.Close()
	}
	if !IsIndexRebuildRequired(err) {
		t.Fatalf("expected rebuild-required error, got %v", err)
	}

	statePath, err := stateDBPathForRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	stateDB, err := sql.Open("sqlite3", statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer stateDB.Close()

	var reps int
	if err := stateDB.QueryRow(`SELECT repetitions FROM srs_items WHERE note_id = 'note-1'`).Scan(&reps); err != nil {
		t.Fatalf("expected migrated SRS row: %v", err)
	}
	if reps != 3 {
		t.Fatalf("expected migrated repetitions 3, got %d", reps)
	}

	data, err := os.ReadFile(filepath.Join(root, bibliographyFileName))
	if err != nil {
		t.Fatalf("expected migrated bibliography file: %v", err)
	}
	var bib bibliographyFile
	if err := json.Unmarshal(data, &bib); err != nil {
		t.Fatalf("unmarshal migrated bibliography: %v", err)
	}
	if len(bib.Refs) != 1 || bib.Refs[0].ID != "book-1" {
		t.Fatalf("expected migrated ref book-1, got %+v", bib.Refs)
	}
}

func assertTablePresent(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name = ?`, name).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected table %q to exist", name)
	}
}

func assertTableMissing(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name = ?`, name).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected table %q to be absent", name)
	}
}
