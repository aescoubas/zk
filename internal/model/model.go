package model

import (
	"time"
)

// Note represents a single node in the Zettelkasten.
type Note struct {
	// ID is the unique identifier for the note, typically the filename stem (slug).
	ID string `json:"id"`
	// Path is the relative path to the note file.
	Path string `json:"path"`
	// Title is the display title of the note.
	Title string `json:"title"`
	// RawContent is the full text content of the note (for indexing/FTS).
	RawContent string `json:"raw_content"`
	// Hash is a checksum of the file content to detect changes.
	Hash string `json:"hash"`
	// ModTime is the last modification time of the file.
	ModTime time.Time `json:"mod_time"`
	// Metadata contains parsed frontmatter and tags.
	Metadata Metadata `json:"metadata"`
	// OutgoingLinks is a list of links found in this note.
	OutgoingLinks []Link `json:"outgoing_links"`
}

// SRSItem represents the state of a note in the Spaced Repetition System.
type SRSItem struct {
	NoteID      string    `json:"note_id"`
	NextReview  time.Time `json:"next_review"`
	Interval    float64   `json:"interval"`    // In days
	EaseFactor  float64   `json:"ease_factor"` // Default 2.5
	Repetitions int       `json:"repetitions"`
}

// Embedding represents a vector embedding for a note.
type Embedding struct {
	NoteID string    `json:"note_id"`
	Vector []float64 `json:"vector"`
	Model  string    `json:"model"`
}

// Link represents a connection between two notes.
type Link struct {
	// SourceID is the ID of the note containing the link.
	SourceID string `json:"source_id"`
	// TargetID is the inferred ID of the target note.
	TargetID string `json:"target_id"`
	// OriginalText is the raw text of the link (e.g., "[[Target Name]]").
	OriginalText string `json:"original_text"`
	// DisplayText is the text displayed for the link.
	DisplayText string `json:"display_text"`
}

// Metadata holds extracted frontmatter and other properties.
type Metadata struct {
	// Tags is a list of tags associated with the note.
	Tags []string `json:"tags"`
	// Frontmatter holds the raw key-value pairs from the YAML header.
	Frontmatter map[string]interface{} `json:"frontmatter"`
}

// NoteSummary represents a summary of a note for listing purposes.
type NoteSummary struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	ModTime       time.Time `json:"mod_time"`
	Tags          []string  `json:"tags"`
	// Backlinks is the number of incoming links.
	Backlinks int `json:"backlinks"`
	// OutgoingLinks is the number of outgoing links.
	OutgoingLinks int `json:"outgoing_links"`
}

// TagCount represents a tag and its usage count.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// SimilarNote represents a note with a similarity score.
type SimilarNote struct {
	Note  *Note
	Score float64
}
