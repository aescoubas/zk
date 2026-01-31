package store

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/escoubas/zk/internal/model"
	_ "github.com/mattn/go-sqlite3" // Import for side effects
)

const schema = `
CREATE TABLE IF NOT EXISTS notes (
	id TEXT PRIMARY KEY,
	path TEXT NOT NULL,
	title TEXT,
	hash TEXT,
	mod_time DATETIME
);

CREATE TABLE IF NOT EXISTS links (
	source_id TEXT,
	target_id TEXT,
	display_text TEXT,
	PRIMARY KEY (source_id, target_id),
	FOREIGN KEY(source_id) REFERENCES notes(id)
);

CREATE TABLE IF NOT EXISTS tags (
	note_id TEXT,
	tag TEXT,
	PRIMARY KEY (note_id, tag),
	FOREIGN KEY(note_id) REFERENCES notes(id)
);

-- FTS5 Virtual Table for Full-Text Search
CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(id UNINDEXED, title, content);

-- Triggers to keep FTS index in sync with notes table
CREATE TRIGGER IF NOT EXISTS notes_ai AFTER INSERT ON notes BEGIN
  INSERT INTO notes_fts(id, title, content) VALUES (new.id, new.title, ''); -- Content updated separately? Or need to store content in DB?
END;

CREATE TRIGGER IF NOT EXISTS notes_ad AFTER DELETE ON notes BEGIN
  DELETE FROM notes_fts WHERE id = old.id;
END;

CREATE TRIGGER IF NOT EXISTS notes_au AFTER UPDATE ON notes BEGIN
  DELETE FROM notes_fts WHERE id = old.id;
  INSERT INTO notes_fts(id, title, content) VALUES (new.id, new.title, ''); -- Logic to update content needed
END;
`

// Note: FTS content update via triggers is tricky if content isn't in the main table.
// For a shadow index, we might just insert directly into both or use the 'notes' table to store content if we want it self-contained.
// Given "Shadow Index", it might be better to manage FTS inserts explicitly in Go code rather than complex triggers if 'content' isn't in 'notes'.
// I'll adjust the schema to simpler tables and manage FTS in code for clarity.

const schemaSimple = `
CREATE TABLE IF NOT EXISTS notes (
	id TEXT PRIMARY KEY,
	path TEXT NOT NULL,
	title TEXT,
	hash TEXT,
	mod_time INTEGER
);

CREATE TABLE IF NOT EXISTS links (
	source_id TEXT,
	target_id TEXT,
	display_text TEXT,
	PRIMARY KEY (source_id, target_id)
);

CREATE TABLE IF NOT EXISTS tags (
	note_id TEXT,
	tag TEXT,
	PRIMARY KEY (note_id, tag)
);

CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(id UNINDEXED, title, content);
`

// Store manages the SQLite database.
type Store struct {
	db *sql.DB
}

// NewStore initializes the database connection and schema.
func NewStore(dbPath string) (*Store, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) initSchema() error {
	_, err := s.db.Exec(schemaSimple)
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}
	return nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// IndexNote inserts or updates a note and its relations.
func (s *Store) IndexNote(n *model.Note) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Upsert Note
	_, err = tx.Exec(`
		INSERT INTO notes (id, path, title, hash, mod_time) 
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			path=excluded.path,
			title=excluded.title,
			hash=excluded.hash,
			mod_time=excluded.mod_time
	`, n.ID, n.Path, n.Title, n.Hash, n.ModTime.Unix())
	if err != nil {
		return fmt.Errorf("failed to upsert note: %w", err)
	}

	// 2. Update FTS
	// We delete first to ensure no duplicates in FTS if ID exists (FTS doesn't support ON CONFLICT REPLACE standardly like tables)
	_, err = tx.Exec(`DELETE FROM notes_fts WHERE id = ?`, n.ID)
	if err != nil {
		return fmt.Errorf("failed to delete fts: %w", err)
	}
	_, err = tx.Exec(`INSERT INTO notes_fts (id, title, content) VALUES (?, ?, ?)`, n.ID, n.Title, n.RawContent)
	if err != nil {
		return fmt.Errorf("failed to insert fts: %w", err)
	}

	// 3. Update Links (Naive: Delete all for source, re-insert)
	_, err = tx.Exec(`DELETE FROM links WHERE source_id = ?`, n.ID)
	if err != nil {
		return fmt.Errorf("failed to delete old links: %w", err)
	}
	stmtLinks, err := tx.Prepare(`INSERT INTO links (source_id, target_id, display_text) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmtLinks.Close()
	for _, l := range n.OutgoingLinks {
		_, err = stmtLinks.Exec(n.ID, l.TargetID, l.DisplayText)
		if err != nil {
			// Log error or ignore duplicates? Links pk is (source, target)
			log.Printf("check link insertion error: %v", err)
		}
	}

	// 4. Update Tags
	_, err = tx.Exec(`DELETE FROM tags WHERE note_id = ?`, n.ID)
	if err != nil {
		return fmt.Errorf("failed to delete old tags: %w", err)
	}
	stmtTags, err := tx.Prepare(`INSERT INTO tags (note_id, tag) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmtTags.Close()
	for _, t := range n.Metadata.Tags {
		_, err = stmtTags.Exec(n.ID, t)
		if err != nil {
			log.Printf("check tag insertion error: %v", err)
		}
	}

	return tx.Commit()
}

// ListNotes retrieves all notes from the database.
func (s *Store) ListNotes() ([]*model.Note, error) {
	rows, err := s.db.Query(`SELECT id, path, title, hash, mod_time FROM notes ORDER BY mod_time DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*model.Note
	for rows.Next() {
		var n model.Note
		var unixTime int64
		if err := rows.Scan(&n.ID, &n.Path, &n.Title, &n.Hash, &unixTime); err != nil {
			return nil, err
		}
		n.ModTime = time.Unix(unixTime, 0)
		notes = append(notes, &n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return notes, nil
}

// GetNote retrieves a note by ID.
func (s *Store) GetNote(id string) (*model.Note, error) {
	row := s.db.QueryRow(`SELECT id, path, title, hash, mod_time FROM notes WHERE id = ?`, id)
	var n model.Note
	var unixTime int64
	err := row.Scan(&n.ID, &n.Path, &n.Title, &n.Hash, &unixTime)
	if err != nil {
		return nil, err
	}
	n.ModTime = time.Unix(unixTime, 0)
	return &n, nil
}

// GetRandomNote retrieves a single random note.
func (s *Store) GetRandomNote() (*model.Note, error) {
	row := s.db.QueryRow(`SELECT id, path, title, hash, mod_time FROM notes ORDER BY RANDOM() LIMIT 1`)
	var n model.Note
	var unixTime int64
	err := row.Scan(&n.ID, &n.Path, &n.Title, &n.Hash, &unixTime)
	if err != nil {
		return nil, err
	}
	n.ModTime = time.Unix(unixTime, 0)
	return &n, nil
}

// GetStaleNotes retrieves notes unmodified for the given duration.
func (s *Store) GetStaleNotes(age time.Duration) ([]*model.Note, error) {
	cutoff := time.Now().Add(-age).Unix()
	rows, err := s.db.Query(`SELECT id, path, title, hash, mod_time FROM notes WHERE mod_time < ? ORDER BY mod_time ASC`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*model.Note
	for rows.Next() {
		var n model.Note
		var unixTime int64
		if err := rows.Scan(&n.ID, &n.Path, &n.Title, &n.Hash, &unixTime); err != nil {
			return nil, err
		}
		n.ModTime = time.Unix(unixTime, 0)
		notes = append(notes, &n)
	}
	return notes, nil
}

// DeleteNote removes a note from the index.
func (s *Store) DeleteNote(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete from tags
	_, err = tx.Exec("DELETE FROM tags WHERE note_id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete tags: %w", err)
	}

	// Delete from links (outgoing)
	_, err = tx.Exec("DELETE FROM links WHERE source_id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete outgoing links: %w", err)
	}
	
	// Delete from FTS
	_, err = tx.Exec("DELETE FROM notes_fts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete from fts: %w", err)
	}

	// Delete from notes
	_, err = tx.Exec("DELETE FROM notes WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}

	return tx.Commit()
}
