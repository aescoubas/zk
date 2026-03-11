package store

import (
	"errors"
	"fmt"
)

const (
	indexSchemaVersion = 1
	stateSchemaVersion = 1
	metaTableName      = "zk_meta"
)

const indexSchema = `
CREATE TABLE IF NOT EXISTS zk_meta (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

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

CREATE TABLE IF NOT EXISTS embeddings (
	note_id TEXT PRIMARY KEY,
	vector BLOB,
	model TEXT
);

CREATE TABLE IF NOT EXISTS citations (
	note_id TEXT,
	ref_id TEXT,
	PRIMARY KEY (note_id, ref_id)
);
`

const stateSchema = `
CREATE TABLE IF NOT EXISTS zk_meta (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS srs_items (
	note_id TEXT PRIMARY KEY,
	next_review INTEGER,
	interval REAL,
	ease_factor REAL,
	repetitions INTEGER
);
`

type indexFeatures struct {
	FTS5 bool `json:"fts5"`
}

type IndexRebuildRequiredError struct {
	Path   string
	Reason string
}

func (e *IndexRebuildRequiredError) Error() string {
	return fmt.Sprintf("index incompatible at %s: %s. Run 'zk index' to rebuild.", e.Path, e.Reason)
}

func IsIndexRebuildRequired(err error) bool {
	var rebuildErr *IndexRebuildRequiredError
	return errors.As(err, &rebuildErr)
}
