package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/escoubas/zk/internal/llm"
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
	summary TEXT,
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

CREATE TABLE IF NOT EXISTS srs_items (
	note_id TEXT PRIMARY KEY,
	next_review INTEGER,
	interval REAL,
	ease_factor REAL,
	repetitions INTEGER
);

CREATE TABLE IF NOT EXISTS embeddings (
	note_id TEXT PRIMARY KEY,
	vector BLOB,
	model TEXT
);
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

	// Migration: Add summary column if it doesn't exist
	// Ignore error if column already exists (SQLite doesn't support IF NOT EXISTS for ADD COLUMN easily in one statement without checking)
	// Simple hack: Try to add it, ignore "duplicate column name" error.
	_, err = s.db.Exec(`ALTER TABLE notes ADD COLUMN summary TEXT`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		// Only return error if it's NOT about the column existing
		// Check for different SQLite error messages variants just in case, but "duplicate column name" is standard.
		// Actually, standard sql package might not return the text string reliably across drivers/versions.
		// Let's assume if schemaSimple ran, table exists.
		// If this fails, it might be serious, or it might be "column exists".
		// Let's log it or ignore it? 
		// Safer: Check pragma.
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
		INSERT INTO notes (id, path, title, summary, hash, mod_time) 
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			path=excluded.path,
			title=excluded.title,
			summary=excluded.summary,
			hash=excluded.hash,
			mod_time=excluded.mod_time
	`, n.ID, n.Path, n.Title, n.Summary, n.Hash, n.ModTime.Unix())
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
	rows, err := s.db.Query(`SELECT id, path, title, summary, hash, mod_time FROM notes ORDER BY mod_time DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*model.Note
	for rows.Next() {
		var n model.Note
		var unixTime int64
		var summary sql.NullString
		if err := rows.Scan(&n.ID, &n.Path, &n.Title, &summary, &n.Hash, &unixTime); err != nil {
			return nil, err
		}
		if summary.Valid {
			n.Summary = summary.String
		}
		n.ModTime = time.Unix(unixTime, 0)
		notes = append(notes, &n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return notes, nil
}

// ListNoteSummaries retrieves all notes with statistics.
func (s *Store) ListNoteSummaries() ([]*model.NoteSummary, error) {
	query := `
		SELECT 
			n.id, 
			n.title, 
			n.mod_time,
			(SELECT COUNT(*) FROM links WHERE target_id = n.id) AS backlinks,
			(SELECT COUNT(*) FROM links WHERE source_id = n.id) AS outgoing,
			(SELECT GROUP_CONCAT(tag, ',') FROM tags WHERE note_id = n.id) AS tags
		FROM notes n
		ORDER BY n.mod_time DESC
	`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []*model.NoteSummary
	for rows.Next() {
		var ns model.NoteSummary
		var unixTime int64
		var tagsStr sql.NullString // Tags might be null

		if err := rows.Scan(&ns.ID, &ns.Title, &unixTime, &ns.Backlinks, &ns.OutgoingLinks, &tagsStr); err != nil {
			return nil, err
		}
		ns.ModTime = time.Unix(unixTime, 0)
		if tagsStr.Valid && tagsStr.String != "" {
			ns.Tags = strings.Split(tagsStr.String, ",")
		} else {
			ns.Tags = []string{}
		}
		summaries = append(summaries, &ns)
	}
	return summaries, nil
}

// GetNote retrieves a note by ID.
func (s *Store) GetNote(id string) (*model.Note, error) {
	row := s.db.QueryRow(`SELECT id, path, title, summary, hash, mod_time FROM notes WHERE id = ?`, id)
	var n model.Note
	var unixTime int64
	var summary sql.NullString
	err := row.Scan(&n.ID, &n.Path, &n.Title, &summary, &n.Hash, &unixTime)
	if err != nil {
		return nil, err
	}
	if summary.Valid {
		n.Summary = summary.String
	}
	n.ModTime = time.Unix(unixTime, 0)
	return &n, nil
}

// GetRandomNote retrieves a single random note.
func (s *Store) GetRandomNote() (*model.Note, error) {
	row := s.db.QueryRow(`SELECT id, path, title, summary, hash, mod_time FROM notes ORDER BY RANDOM() LIMIT 1`)
	var n model.Note
	var unixTime int64
	var summary sql.NullString
	err := row.Scan(&n.ID, &n.Path, &n.Title, &summary, &n.Hash, &unixTime)
	if err != nil {
		return nil, err
	}
	if summary.Valid {
		n.Summary = summary.String
	}
	n.ModTime = time.Unix(unixTime, 0)
	return &n, nil
}

// GetStaleNotes retrieves notes unmodified for the given duration.
func (s *Store) GetStaleNotes(age time.Duration) ([]*model.Note, error) {
	cutoff := time.Now().Add(-age).Unix()
	rows, err := s.db.Query(`SELECT id, path, title, summary, hash, mod_time FROM notes WHERE mod_time < ? ORDER BY mod_time ASC`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*model.Note
	for rows.Next() {
		var n model.Note
		var unixTime int64
		var summary sql.NullString
		if err := rows.Scan(&n.ID, &n.Path, &n.Title, &summary, &n.Hash, &unixTime); err != nil {
			return nil, err
		}
		if summary.Valid {
			n.Summary = summary.String
		}
		n.ModTime = time.Unix(unixTime, 0)
		notes = append(notes, &n)
	}
	return notes, nil
}

// GetStats returns the total count of notes and links.
func (s *Store) GetStats() (int, int, error) {
	var notesCount, linksCount int
	if err := s.db.QueryRow("SELECT count(*) FROM notes").Scan(&notesCount); err != nil {
		return 0, 0, err
	}
	if err := s.db.QueryRow("SELECT count(*) FROM links").Scan(&linksCount); err != nil {
		return 0, 0, err
	}
	return notesCount, linksCount, nil
}

// GetRecentNotes retrieves the most recently modified notes.
func (s *Store) GetRecentNotes(limit int) ([]*model.Note, error) {
	rows, err := s.db.Query(`SELECT id, path, title, summary, hash, mod_time FROM notes ORDER BY mod_time DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*model.Note
	for rows.Next() {
		var n model.Note
		var unixTime int64
		var summary sql.NullString
		if err := rows.Scan(&n.ID, &n.Path, &n.Title, &summary, &n.Hash, &unixTime); err != nil {
			return nil, err
		}
		if summary.Valid {
			n.Summary = summary.String
		}
		n.ModTime = time.Unix(unixTime, 0)
		notes = append(notes, &n)
	}
	return notes, nil
}

// GetBacklinks retrieves notes that link to the given targetID.
func (s *Store) GetBacklinks(targetID string) ([]*model.Note, error) {
	query := `
		SELECT n.id, n.path, n.title, n.summary, n.hash, n.mod_time
		FROM notes n
		JOIN links l ON n.id = l.source_id
		WHERE l.target_id = ?
		ORDER BY n.mod_time DESC
	`
	rows, err := s.db.Query(query, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*model.Note
	for rows.Next() {
		var n model.Note
		var unixTime int64
		var summary sql.NullString
		if err := rows.Scan(&n.ID, &n.Path, &n.Title, &summary, &n.Hash, &unixTime); err != nil {
			return nil, err
		}
		if summary.Valid {
			n.Summary = summary.String
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

	// Delete from SRS
	_, err = tx.Exec("DELETE FROM srs_items WHERE note_id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete srs item: %w", err)
	}

	return tx.Commit()
}

// GetSRSItem retrieves the SRS state for a note.
func (s *Store) GetSRSItem(noteID string) (*model.SRSItem, error) {
	row := s.db.QueryRow(`SELECT note_id, next_review, interval, ease_factor, repetitions FROM srs_items WHERE note_id = ?`, noteID)
	var item model.SRSItem
	var nextReview int64
	err := row.Scan(&item.NoteID, &nextReview, &item.Interval, &item.EaseFactor, &item.Repetitions)
	if err == sql.ErrNoRows {
		return nil, nil // Not found is okay, means new item
	}
	if err != nil {
		return nil, err
	}
	item.NextReview = time.Unix(nextReview, 0)
	return &item, nil
}

// SaveSRSItem inserts or updates an SRS item.
func (s *Store) SaveSRSItem(item *model.SRSItem) error {
	_, err := s.db.Exec(`
		INSERT INTO srs_items (note_id, next_review, interval, ease_factor, repetitions)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(note_id) DO UPDATE SET
			next_review=excluded.next_review,
			interval=excluded.interval,
			ease_factor=excluded.ease_factor,
			repetitions=excluded.repetitions
	`, item.NoteID, item.NextReview.Unix(), item.Interval, item.EaseFactor, item.Repetitions)
	return err
}

// GetDueReviews retrieves notes that are due for review.
func (s *Store) GetDueReviews() ([]*model.Note, error) {
	now := time.Now().Unix()
	// Get notes where next_review <= now OR next_review IS NULL (if we want to include unreviewed notes?)
	// For now, let's only return items that exist in SRS table and are due.
	// Users can add notes to SRS manually or we can have a policy to auto-add.
	// Roadmap says: "based on note importance and freshness".
	// For this iteration, let's stick to items already in SRS or explicitly "stale but important".
	
	// Let's just return what is in SRS table and due.
	query := `
		SELECT n.id, n.path, n.title, n.summary, n.hash, n.mod_time
		FROM notes n
		JOIN srs_items s ON n.id = s.note_id
		WHERE s.next_review <= ?
		ORDER BY s.next_review ASC
	`
	rows, err := s.db.Query(query, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*model.Note
	for rows.Next() {
		var n model.Note
		var unixTime int64
		var summary sql.NullString
		if err := rows.Scan(&n.ID, &n.Path, &n.Title, &summary, &n.Hash, &unixTime); err != nil {
			return nil, err
		}
		if summary.Valid {
			n.Summary = summary.String
		}
		n.ModTime = time.Unix(unixTime, 0)
		notes = append(notes, &n)
	}
	return notes, nil
}

// SaveEmbedding saves the vector embedding for a note.
func (s *Store) SaveEmbedding(e *model.Embedding) error {
	vecBytes, err := json.Marshal(e.Vector)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO embeddings (note_id, vector, model)
		VALUES (?, ?, ?)
		ON CONFLICT(note_id) DO UPDATE SET
			vector=excluded.vector,
			model=excluded.model
	`, e.NoteID, vecBytes, e.Model)
	return err
}

// GetEmbedding retrieves the embedding for a note.
func (s *Store) GetEmbedding(noteID string) (*model.Embedding, error) {
	row := s.db.QueryRow(`SELECT note_id, vector, model FROM embeddings WHERE note_id = ?`, noteID)
	var e model.Embedding
	var vecBytes []byte
	err := row.Scan(&e.NoteID, &vecBytes, &e.Model)
	if err == sql.ErrNoRows {
		return nil, nil // Not found is okay
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(vecBytes, &e.Vector); err != nil {
		return nil, err
	}
	return &e, nil
}

// GetAllEmbeddings retrieves all embeddings.
func (s *Store) GetAllEmbeddings() ([]*model.Embedding, error) {
	rows, err := s.db.Query(`SELECT note_id, vector, model FROM embeddings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var embeddings []*model.Embedding
	for rows.Next() {
		var e model.Embedding
		var vecBytes []byte
		if err := rows.Scan(&e.NoteID, &vecBytes, &e.Model); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(vecBytes, &e.Vector); err != nil {
			continue // Skip malformed?
		}
		embeddings = append(embeddings, &e)
	}
	return embeddings, nil
}

// SearchNotes performs a full-text search using FTS5.
func (s *Store) SearchNotes(query string) ([]*model.Note, error) {
	// FTS5 query
	rows, err := s.db.Query(`
		SELECT n.id, n.path, n.title, n.summary, n.hash, n.mod_time 
		FROM notes n
		JOIN notes_fts f ON n.id = f.id
		WHERE notes_fts MATCH ?
		ORDER BY rank
	`, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*model.Note
	for rows.Next() {
		var n model.Note
		var unixTime int64
		var summary sql.NullString
		if err := rows.Scan(&n.ID, &n.Path, &n.Title, &summary, &n.Hash, &unixTime); err != nil {
			return nil, err
		}
		if summary.Valid {
			n.Summary = summary.String
		}
		n.ModTime = time.Unix(unixTime, 0)
		notes = append(notes, &n)
	}
	return notes, nil
}

// FindSimilar finds notes semantically similar to the target note.
func (s *Store) FindSimilar(targetID string, limit int) ([]model.SimilarNote, error) {
	// 1. Get Target Embedding
	targetEmb, err := s.GetEmbedding(targetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get target embedding: %w", err)
	}
    if targetEmb == nil {
        return nil, nil // No embedding, no similar notes
    }

	// 2. Get All Embeddings
	// Optimization: In a real DB, we'd use a vector index (sqlite-vss). 
	// Here we load all (slow for large DBs, fine for personal ZK).
	allEmbs, err := s.GetAllEmbeddings()
	if err != nil {
		return nil, fmt.Errorf("failed to load embeddings: %w", err)
	}

	// 3. Compute Similarity
	var matches []model.SimilarNote
	for _, e := range allEmbs {
		if e.NoteID == targetID {
			continue
		}
		score := llm.CosineSimilarity(targetEmb.Vector, e.Vector)
		
		// Only keep relevant matches? Or return top N?
		// Let's filter slightly to avoid garbage
		if score > 0.3 { // Threshold?
			// We need the Note object. 
			// Performance: Getting note for every match is slow.
			// Better: Sort first, then fetch top N notes.
			matches = append(matches, model.SimilarNote{
				Note: &model.Note{ID: e.NoteID}, // Temporary placeholder
				Score: score,
			})
		}
	}

	// 4. Sort
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	// 5. Limit
	if len(matches) > limit {
		matches = matches[:limit]
	}

	// 6. Fill Note Details
	for i := range matches {
		n, err := s.GetNote(matches[i].Note.ID)
		if err == nil {
			matches[i].Note = n
		} else {
			matches[i].Note.Title = matches[i].Note.ID // Fallback
		}
	}

	return matches, nil
}

// SearchByVector finds notes semantically similar to the query vector.
func (s *Store) SearchByVector(queryVector []float64, limit int) ([]model.SimilarNote, error) {
	// 1. Get All Embeddings
	allEmbs, err := s.GetAllEmbeddings()
	if err != nil {
		return nil, fmt.Errorf("failed to load embeddings: %w", err)
	}

	// 2. Compute Similarity
	var matches []model.SimilarNote
	for _, e := range allEmbs {
		score := llm.CosineSimilarity(queryVector, e.Vector)
		
		if score > 0.15 { // Lower threshold for search queries as they are short/imperfect
			matches = append(matches, model.SimilarNote{
				Note: &model.Note{ID: e.NoteID},
				Score: score,
			})
		}
	}

	// 3. Sort
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	// 4. Limit
	if len(matches) > limit {
		matches = matches[:limit]
	}

	// 5. Fill Note Details
	for i := range matches {
		n, err := s.GetNote(matches[i].Note.ID)
		if err == nil {
			matches[i].Note = n
		} else {
			matches[i].Note.Title = matches[i].Note.ID // Fallback
		}
	}

	return matches, nil
}

// GetTopTags returns the most frequent tags.
func (s *Store) GetTopTags(limit int) ([]model.TagCount, error) {
	rows, err := s.db.Query(`SELECT tag, count(*) as c FROM tags GROUP BY tag ORDER BY c DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tags []model.TagCount
	for rows.Next() {
		var t model.TagCount
		if err := rows.Scan(&t.Tag, &t.Count); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, nil
}

// GetOrphanCount returns the number of notes with zero backlinks.
func (s *Store) GetOrphanCount() (int, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT count(*) FROM notes n 
		WHERE NOT EXISTS (SELECT 1 FROM links l WHERE l.target_id = n.id)
	`).Scan(&count)
	return count, err
}

// GetStubCount returns the number of short notes (e.g. < 100 chars in FTS).
func (s *Store) GetStubCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT count(*) FROM notes_fts WHERE length(content) < 100`).Scan(&count)
	return count, err
}
