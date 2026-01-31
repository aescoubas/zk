package parser

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/escoubas/zk/internal/model"
	"github.com/yuin/goldmark"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/parser"
)

// Parser handles the parsing of note files.
type Parser struct {
	md goldmark.Markdown
}

// NewParser creates a new Parser instance.
func NewParser() *Parser {
	md := goldmark.New(
		goldmark.WithExtensions(
			meta.Meta,
		),
	)
	return &Parser{md: md}
}

// ParseFile reads a file and converts it into a Note model.
func (p *Parser) ParseFile(root, path string) (*model.Note, error) {
	fullPath := filepath.Join(root, path)
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Get file info for modification time
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	// Read content
	content, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	// Compute Hash
	hash := fmt.Sprintf("%x", sha256.Sum256(content))

	// Parse Markdown
	context := parser.NewContext()
	var buf bytes.Buffer
	if err := p.md.Convert(content, &buf, parser.WithContext(context)); err != nil {
		return nil, err
	}

	// Extract Metadata
	metaData := meta.Get(context)
	frontmatter := make(map[string]interface{})
	tags := []string{}

	for k, v := range metaData {
		frontmatter[k] = v
		if k == "tags" {
			if tList, ok := v.([]interface{}); ok {
				for _, t := range tList {
					if tStr, ok := t.(string); ok {
						tags = append(tags, tStr)
					}
				}
			}
		}
	}

	// Extract Title
	// Strategy: Use 'title' from frontmatter, or filename.
	title := ""
	if t, ok := frontmatter["title"].(string); ok {
		title = t
	} else {
		// Fallback to filename without extension
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	// Extract ID
	// Strategy: Filename stem (slug) is the ID.
	id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	// Extract Links (Regex for now)
	links := extractLinks(string(content), id)

	note := &model.Note{
		ID:         id,
		Path:       path,
		Title:      title,
		RawContent: string(content),
		Hash:       hash,
		ModTime:    fi.ModTime(),
		Metadata: model.Metadata{
			Tags:        tags,
			Frontmatter: frontmatter,
		},
		OutgoingLinks: links,
	}

	return note, nil
}

var linkRegex = regexp.MustCompile(`\[\[(.*?)\]\]`)

func extractLinks(content, sourceID string) []model.Link {
	matches := linkRegex.FindAllStringSubmatch(content, -1)
	var links []model.Link
	seen := make(map[string]bool)

	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		original := m[0]
		inner := m[1]
		
		// Handle aliased links [[Target|Alias]]
		parts := strings.Split(inner, "|")
		target := strings.TrimSpace(parts[0])
		display := target
		if len(parts) > 1 {
			display = strings.TrimSpace(parts[1])
		}

		// Avoid duplicates per note? Or keep all occurrences?
		// Usually distinct links are enough for graph.
		if seen[original] {
			continue
		}
		seen[original] = true

		links = append(links, model.Link{
			SourceID:     sourceID,
			TargetID:     target, // Simplistic ID inference: explicit target name
			OriginalText: original,
			DisplayText:  display,
		})
	}
	return links
}
