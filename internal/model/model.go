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
