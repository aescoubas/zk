package store

import (
	"database/sql"
	"encoding/json"
	"errors"
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

// Store manages the SQLite database.
type Store struct {
	db               *sql.DB
	stateDB          *sql.DB
	root             string
	indexPath        string
	statePath        string
	bibliographyPath string
	hasFTS           bool
}

// NewStore initializes the database connection and schema.
func NewStore(dbPath string) (*Store, error) {
	absDBPath, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve database path: %w", err)
	}

	root := filepath.Dir(filepath.Dir(absDBPath))
	statePath, err := stateDBPathForRoot(root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve state database path: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(absDBPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite3", absDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, &IndexRebuildRequiredError{Path: absDBPath, Reason: err.Error()}
	}

	s := &Store{
		db:               db,
		root:             root,
		indexPath:        absDBPath,
		statePath:        statePath,
		bibliographyPath: bibliographyPathForRoot(root),
	}

	if err := s.migrateLegacyPortableData(); err != nil {
		db.Close()
		return nil, err
	}

	features := indexFeatures{FTS5: probeFTS5Support()}
	if err := s.initIndexSchema(features); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func probeFTS5Support() bool {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return false
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return false
	}
	if _, err := db.Exec(`CREATE VIRTUAL TABLE fts_probe USING fts5(content)`); err != nil {
		return false
	}
	return true
}

func (s *Store) initIndexSchema(features indexFeatures) error {
	userTables, err := listUserTables(s.db)
	if err != nil {
		return fmt.Errorf("failed to inspect index schema: %w", err)
	}

	if len(userTables) == 0 {
		return s.bootstrapIndexSchema(features)
	}
	if !userTables[metaTableName] {
		return &IndexRebuildRequiredError{Path: s.indexPath, Reason: "legacy index schema has no version metadata"}
	}

	version, err := readMetaInt(s.db, "schema_version")
	if err != nil {
		return &IndexRebuildRequiredError{Path: s.indexPath, Reason: "index schema version metadata is unreadable"}
	}
	if version != indexSchemaVersion {
		return &IndexRebuildRequiredError{Path: s.indexPath, Reason: fmt.Sprintf("schema version %d does not match expected version %d", version, indexSchemaVersion)}
	}

	storedFeatures, err := readIndexFeatures(s.db)
	if err != nil {
		return &IndexRebuildRequiredError{Path: s.indexPath, Reason: "index feature metadata is unreadable"}
	}
	if storedFeatures != features {
		return &IndexRebuildRequiredError{Path: s.indexPath, Reason: fmt.Sprintf("stored feature flags %+v do not match current binary %+v", storedFeatures, features)}
	}

	for _, required := range []string{"notes", "links", "tags", "embeddings", "citations"} {
		if !userTables[required] {
			return &IndexRebuildRequiredError{Path: s.indexPath, Reason: fmt.Sprintf("required table %q is missing", required)}
		}
	}
	if userTables["srs_items"] {
		return &IndexRebuildRequiredError{Path: s.indexPath, Reason: "legacy SRS data still lives in index.db"}
	}
	if userTables["refs"] {
		return &IndexRebuildRequiredError{Path: s.indexPath, Reason: "legacy bibliography data still lives in index.db"}
	}
	if features.FTS5 {
		if !userTables["notes_fts"] {
			return &IndexRebuildRequiredError{Path: s.indexPath, Reason: "FTS5 index is missing"}
		}
		if _, err := s.db.Exec(`SELECT count(*) FROM notes_fts LIMIT 1`); err != nil {
			return &IndexRebuildRequiredError{Path: s.indexPath, Reason: fmt.Sprintf("FTS5 index is unreadable: %v", err)}
		}
	}

	s.hasFTS = features.FTS5
	return nil
}

func (s *Store) bootstrapIndexSchema(features indexFeatures) error {
	if _, err := s.db.Exec(indexSchema); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}
	if err := writeMetaValue(s.db, "schema_version", fmt.Sprintf("%d", indexSchemaVersion)); err != nil {
		return err
	}
	if err := writeIndexFeatures(s.db, features); err != nil {
		return err
	}
	if features.FTS5 {
		if _, err := s.db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(id UNINDEXED, title, content)`); err != nil {
			return fmt.Errorf("failed to create FTS5 table: %w", err)
		}
	}
	s.hasFTS = features.FTS5
	return nil
}

func (s *Store) migrateLegacyPortableData() error {
	userTables, err := listUserTables(s.db)
	if err != nil {
		return err
	}

	if userTables["srs_items"] {
		if err := s.migrateLegacySRS(); err != nil {
			return fmt.Errorf("failed to migrate legacy SRS state: %w", err)
		}
	}
	if userTables["refs"] {
		if err := s.migrateLegacyBibliography(); err != nil {
			return fmt.Errorf("failed to migrate legacy bibliography: %w", err)
		}
	}
	return nil
}

func (s *Store) migrateLegacySRS() error {
	rows, err := s.db.Query(`SELECT note_id, next_review, interval, ease_factor, repetitions FROM srs_items`)
	if err != nil {
		return err
	}
	defer rows.Close()

	stateDB, err := s.ensureStateDB()
	if err != nil {
		return err
	}

	stmt, err := stateDB.Prepare(`
		INSERT INTO srs_items (note_id, next_review, interval, ease_factor, repetitions)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(note_id) DO NOTHING
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for rows.Next() {
		var item model.SRSItem
		var nextReview int64
		if err := rows.Scan(&item.NoteID, &nextReview, &item.Interval, &item.EaseFactor, &item.Repetitions); err != nil {
			return err
		}
		if _, err := stmt.Exec(item.NoteID, nextReview, item.Interval, item.EaseFactor, item.Repetitions); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Store) migrateLegacyBibliography() error {
	rows, err := s.db.Query(`SELECT id, type, title, author, year, url, description FROM refs`)
	if err != nil {
		return err
	}
	defer rows.Close()

	file, err := s.loadBibliography()
	if err != nil {
		return err
	}

	existing := make(map[string]model.Ref, len(file.Refs))
	for _, ref := range file.Refs {
		existing[ref.ID] = ref
	}
	changed := false

	for rows.Next() {
		var ref model.Ref
		var refType, author, year, url, desc sql.NullString
		if err := rows.Scan(&ref.ID, &refType, &ref.Title, &author, &year, &url, &desc); err != nil {
			return err
		}
		if refType.Valid {
			ref.Type = refType.String
		}
		if author.Valid {
			ref.Author = author.String
		}
		if year.Valid {
			ref.Year = year.String
		}
		if url.Valid {
			ref.URL = url.String
		}
		if desc.Valid {
			ref.Description = desc.String
		}
		if _, ok := existing[ref.ID]; !ok {
			file.Refs = append(file.Refs, ref)
			changed = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if !changed {
		if _, err := os.Stat(s.bibliographyPath); errors.Is(err, os.ErrNotExist) {
			return nil
		}
	}

	return s.saveBibliography(file)
}

func (s *Store) ensureStateDB() (*sql.DB, error) {
	if s.stateDB != nil {
		return s.stateDB, nil
	}

	if err := os.MkdirAll(filepath.Dir(s.statePath), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", s.statePath)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(stateSchema); err != nil {
		db.Close()
		return nil, err
	}
	if err := writeMetaValue(db, "schema_version", fmt.Sprintf("%d", stateSchemaVersion)); err != nil {
		db.Close()
		return nil, err
	}

	s.stateDB = db
	return s.stateDB, nil
}

func listUserTables(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables[name] = true
	}
	return tables, rows.Err()
}

func readMetaInt(db *sql.DB, key string) (int, error) {
	var raw string
	if err := db.QueryRow(`SELECT value FROM zk_meta WHERE key = ?`, key).Scan(&raw); err != nil {
		return 0, err
	}
	var value int
	if _, err := fmt.Sscanf(raw, "%d", &value); err != nil {
		return 0, err
	}
	return value, nil
}

func writeMetaValue(db *sql.DB, key, value string) error {
	_, err := db.Exec(`
		INSERT INTO zk_meta (key, value)
		VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

func readIndexFeatures(db *sql.DB) (indexFeatures, error) {
	var raw string
	if err := db.QueryRow(`SELECT value FROM zk_meta WHERE key = 'features'`).Scan(&raw); err != nil {
		return indexFeatures{}, err
	}
	var features indexFeatures
	if err := json.Unmarshal([]byte(raw), &features); err != nil {
		return indexFeatures{}, err
	}
	return features, nil
}

func writeIndexFeatures(db *sql.DB, features indexFeatures) error {
	data, err := json.Marshal(features)
	if err != nil {
		return err
	}
	return writeMetaValue(db, "features", string(data))
}

func ResetIndex(dbPath string) error {
	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	var errs []error
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if s.stateDB != nil {
		if err := s.stateDB.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs[0]
}

func (s *Store) HasFTS() bool {
	return s.hasFTS
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

	// 2. Update FTS (if available)
	if s.hasFTS {
		_, err = tx.Exec(`DELETE FROM notes_fts WHERE id = ?`, n.ID)
		if err != nil {
			return fmt.Errorf("failed to delete fts: %w", err)
		}
		_, err = tx.Exec(`INSERT INTO notes_fts (id, title, content) VALUES (?, ?, ?)`, n.ID, n.Title, n.RawContent)
		if err != nil {
			return fmt.Errorf("failed to insert fts: %w", err)
		}
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

	seenTargets := make(map[string]bool)
	for _, l := range n.OutgoingLinks {
		if seenTargets[l.TargetID] {
			continue
		}
		seenTargets[l.TargetID] = true

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

	// 5. Update Citations
	_, err = tx.Exec(`DELETE FROM citations WHERE note_id = ?`, n.ID)
	if err != nil {
		return fmt.Errorf("failed to delete old citations: %w", err)
	}
	stmtCite, err := tx.Prepare(`INSERT OR IGNORE INTO citations (note_id, ref_id) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmtCite.Close()
	for _, c := range n.Citations {
		_, err = stmtCite.Exec(n.ID, c.RefID)
		if err != nil {
			log.Printf("check citation insertion error: %v", err)
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

	// Fetch Outgoing Links
	rows, err := s.db.Query(`SELECT target_id, display_text FROM links WHERE source_id = ?`, n.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch links: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var l model.Link
		l.SourceID = n.ID
		if err := rows.Scan(&l.TargetID, &l.DisplayText); err != nil {
			return nil, err
		}
		n.OutgoingLinks = append(n.OutgoingLinks, l)
	}

	// Fetch Tags
	tagRows, err := s.db.Query(`SELECT tag FROM tags WHERE note_id = ?`, n.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}
	defer tagRows.Close()
	for tagRows.Next() {
		var t string
		if err := tagRows.Scan(&t); err != nil {
			return nil, err
		}
		n.Metadata.Tags = append(n.Metadata.Tags, t)
	}

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

	// Delete from FTS (if available)
	if s.hasFTS {
		_, err = tx.Exec("DELETE FROM notes_fts WHERE id = ?", id)
		if err != nil {
			return fmt.Errorf("failed to delete from fts: %w", err)
		}
	}

	// Delete from notes
	_, err = tx.Exec("DELETE FROM notes WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}

	// Delete from citations
	_, err = tx.Exec("DELETE FROM citations WHERE note_id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete citations: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	stateDB, err := s.ensureStateDB()
	if err != nil {
		return err
	}
	if _, err := stateDB.Exec("DELETE FROM srs_items WHERE note_id = ?", id); err != nil {
		return fmt.Errorf("failed to delete srs item: %w", err)
	}

	return nil
}

// GetSRSItem retrieves the SRS state for a note.
func (s *Store) GetSRSItem(noteID string) (*model.SRSItem, error) {
	stateDB, err := s.ensureStateDB()
	if err != nil {
		return nil, err
	}

	row := stateDB.QueryRow(`SELECT note_id, next_review, interval, ease_factor, repetitions FROM srs_items WHERE note_id = ?`, noteID)
	var item model.SRSItem
	var nextReview int64
	err = row.Scan(&item.NoteID, &nextReview, &item.Interval, &item.EaseFactor, &item.Repetitions)
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
	stateDB, err := s.ensureStateDB()
	if err != nil {
		return err
	}

	_, err = stateDB.Exec(`
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
	stateDB, err := s.ensureStateDB()
	if err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	rows, err := stateDB.Query(`
		SELECT note_id
		FROM srs_items
		WHERE next_review <= ?
		ORDER BY next_review ASC
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*model.Note
	for rows.Next() {
		var noteID string
		if err := rows.Scan(&noteID); err != nil {
			return nil, err
		}

		n, err := s.GetNote(noteID)
		if errors.Is(err, sql.ErrNoRows) {
			if _, cleanupErr := stateDB.Exec(`DELETE FROM srs_items WHERE note_id = ?`, noteID); cleanupErr != nil {
				log.Printf("Failed to prune orphaned SRS item %s: %v", noteID, cleanupErr)
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		notes = append(notes, n)
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
// Falls back to LIKE-based search if FTS5 is not available.
func (s *Store) SearchNotes(query string) ([]*model.Note, error) {
	if !s.hasFTS {
		return s.searchNotesLike(query)
	}

	// Sanitize query: replace single quotes with spaces.
	// This ensures that "don't" is treated as "don" AND "t", matching the tokenizer's output.
	query = strings.ReplaceAll(query, "'", " ")

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

// searchNotesLike is a fallback search using LIKE when FTS5 is unavailable.
func (s *Store) searchNotesLike(query string) ([]*model.Note, error) {
	like := "%" + query + "%"
	rows, err := s.db.Query(`
		SELECT id, path, title, summary, hash, mod_time
		FROM notes
		WHERE title LIKE ? OR id LIKE ?
		ORDER BY mod_time DESC
	`, like, like)
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
				Note:  &model.Note{ID: e.NoteID}, // Temporary placeholder
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
				Note:  &model.Note{ID: e.NoteID},
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
	if !s.hasFTS {
		return 0, nil
	}
	var count int
	err := s.db.QueryRow(`SELECT count(*) FROM notes_fts WHERE length(content) < 100`).Scan(&count)
	return count, err
}

// --- Bibliography/Reference Methods ---

// UpsertRef inserts or updates a bibliographic reference.
func (s *Store) UpsertRef(r *model.Ref) error {
	file, err := s.loadBibliography()
	if err != nil {
		return err
	}

	replaced := false
	for i := range file.Refs {
		if file.Refs[i].ID == r.ID {
			file.Refs[i] = *r
			replaced = true
			break
		}
	}
	if !replaced {
		file.Refs = append(file.Refs, *r)
	}
	return s.saveBibliography(file)
}

// GetRef retrieves a reference by ID.
func (s *Store) GetRef(id string) (*model.Ref, error) {
	file, err := s.loadBibliography()
	if err != nil {
		return nil, err
	}
	for _, ref := range file.Refs {
		if ref.ID == id {
			copy := ref
			return &copy, nil
		}
	}
	return nil, sql.ErrNoRows
}

// DeleteRef removes a reference and its citations.
func (s *Store) DeleteRef(id string) error {
	file, err := s.loadBibliography()
	if err != nil {
		return err
	}

	filtered := file.Refs[:0]
	found := false
	for _, ref := range file.Refs {
		if ref.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, ref)
	}
	if !found {
		return sql.ErrNoRows
	}
	file.Refs = filtered
	if err := s.saveBibliography(file); err != nil {
		return err
	}

	if _, err := s.db.Exec("DELETE FROM citations WHERE ref_id = ?", id); err != nil {
		return fmt.Errorf("failed to delete citations for ref: %w", err)
	}
	return nil
}

// ListRefSummaries retrieves all references with citation counts.
func (s *Store) ListRefSummaries() ([]model.RefSummary, error) {
	file, err := s.loadBibliography()
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`SELECT ref_id, count(*) FROM citations GROUP BY ref_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var refID string
		var count int
		if err := rows.Scan(&refID, &count); err != nil {
			return nil, err
		}
		counts[refID] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	summaries := make([]model.RefSummary, 0, len(file.Refs))
	for _, ref := range file.Refs {
		summaries = append(summaries, model.RefSummary{
			Ref:       ref,
			Citations: counts[ref.ID],
		})
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Citations == summaries[j].Citations {
			if summaries[i].Ref.Title == summaries[j].Ref.Title {
				return summaries[i].Ref.ID < summaries[j].Ref.ID
			}
			return summaries[i].Ref.Title < summaries[j].Ref.Title
		}
		return summaries[i].Citations > summaries[j].Citations
	})
	return summaries, nil
}

// GetRefCount returns the total number of references.
func (s *Store) GetRefCount() (int, error) {
	file, err := s.loadBibliography()
	if err != nil {
		return 0, err
	}
	return len(file.Refs), nil
}

// GetCitingNotes returns notes that cite a given reference.
func (s *Store) GetCitingNotes(refID string) ([]*model.Note, error) {
	query := `
		SELECT n.id, n.path, n.title, n.summary, n.hash, n.mod_time
		FROM notes n
		JOIN citations c ON n.id = c.note_id
		WHERE c.ref_id = ?
		ORDER BY n.mod_time DESC
	`
	rows, err := s.db.Query(query, refID)
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
