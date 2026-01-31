package store

import (
	"os"
	"testing"
	"time"

	"github.com/escoubas/zk/internal/model"
)

func TestStore(t *testing.T) {
	// Temp DB
	f, err := os.CreateTemp("", "zk_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	dbPath := f.Name()
	f.Close()
	defer os.Remove(dbPath)

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

	// 5. Delete Note
	if err := st.DeleteNote("note-1"); err != nil {
		t.Fatalf("DeleteNote failed: %v", err)
	}
	
	_, err = st.GetNote("note-1")
	if err == nil {
		t.Error("Expected error getting deleted note, got nil")
	}
}
