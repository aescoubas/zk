package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/escoubas/zk/internal/store"
)

type Server struct {
	store     *store.Store
	rootDir   string
	documents map[string]string // URI -> Content
}

func NewServer(st *store.Store, root string) *Server {
	return &Server{
		store:     st,
		rootDir:   root,
		documents: make(map[string]string),
	}
}

func (s *Server) Serve() error {
	scanner := bufio.NewReader(os.Stdin)
	for {
		body, err := ReadMessage(scanner)
		if err != nil {
			return err
		}

		// Decode minimal to get method
		var base struct {
			Method string      `json:"method"`
			ID     interface{} `json:"id"`
		}
		if err := json.Unmarshal(body, &base); err != nil {
			log.Printf("Error decoding base message: %v", err)
			continue
		}

		// Handle Requests
		if base.ID != nil {
			resp, err := s.handleRequest(base.Method, body)
			response := Response{
				JSONRPC: "2.0",
				ID:      base.ID,
			}
			if err != nil {
				response.Error = &RPCError{Code: -32603, Message: err.Error()}
			} else {
				response.Result = resp
			}
			WriteMessage(os.Stdout, response)
		} else {
			// Handle Notifications
			s.handleNotification(base.Method, body)
		}
	}
}

func (s *Server) handleRequest(method string, body []byte) (interface{}, error) {
	switch method {
	case "initialize":
		var params InitializeParams
		if err := json.Unmarshal(body, &struct{ Params *InitializeParams `json:"params"` }{&params}); err != nil {
			return nil, err
		}
		return s.handleInitialize(params)
	case "textDocument/completion":
		var params CompletionParams
		if err := json.Unmarshal(body, &struct{ Params *CompletionParams `json:"params"` }{&params}); err != nil {
			return nil, err
		}
		return s.handleCompletion(params)
	case "textDocument/definition":
		var params DefinitionParams
		if err := json.Unmarshal(body, &struct{ Params *DefinitionParams `json:"params"` }{&params}); err != nil {
			return nil, err
		}
		return s.handleDefinition(params)
	case "textDocument/hover":
		var params HoverParams
		if err := json.Unmarshal(body, &struct{ Params *HoverParams `json:"params"` }{&params}); err != nil {
			return nil, err
		}
		return s.handleHover(params)
	}
	return nil, nil
}

func (s *Server) handleNotification(method string, body []byte) {
	switch method {
	case "textDocument/didOpen":
		var params DidOpenTextDocumentParams
		json.Unmarshal(body, &struct{ Params *DidOpenTextDocumentParams `json:"params"` }{&params})
		s.documents[params.TextDocument.URI] = params.TextDocument.Text
	case "textDocument/didChange":
		var params DidChangeTextDocumentParams
		json.Unmarshal(body, &struct{ Params *DidChangeTextDocumentParams `json:"params"` }{&params})
		if len(params.ContentChanges) > 0 {
			// Full sync assume
			s.documents[params.TextDocument.URI] = params.ContentChanges[0].Text
		}
	}
}

// Handlers

func (s *Server) handleInitialize(params InitializeParams) (InitializeResult, error) {
	trigger := "["
	return InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: 1, // Full
			CompletionProvider: &struct {
				TriggerCharacters []string `json:"triggerCharacters"`
			}{TriggerCharacters: []string{trigger}},
			DefinitionProvider: true,
			HoverProvider:      true,
		},
	}, nil
}

func (s *Server) handleCompletion(params CompletionParams) (CompletionList, error) {
	content, ok := s.documents[params.TextDocument.URI]
	if !ok {
		return CompletionList{}, nil
	}

	lines := strings.Split(content, "\n")
	if params.Position.Line >= len(lines) {
		return CompletionList{}, nil
	}
	line := lines[params.Position.Line]
	
	// Check if we are inside [[
	// Simple check: Last occurrence of [[ before cursor, no ]] after it before cursor.
	cursor := params.Position.Character
	if cursor > len(line) {
		cursor = len(line)
	}
	prefix := line[:cursor]
	
	startIdx := strings.LastIndex(prefix, "[[")
	if startIdx == -1 {
		return CompletionList{}, nil
	}
	
	// Ensure no closing ]] between start and cursor
	if strings.Contains(prefix[startIdx:], "]]") {
		return CompletionList{}, nil
	}
	
	// Query := text between [[ and cursor
	query := prefix[startIdx+2:]
	
	notes, err := s.store.ListNotes() // Maybe optimize to search?
	if err != nil {
		return CompletionList{}, nil
	}

	var items []CompletionItem
	for _, n := range notes {
		// Basic filter
		if query == "" || strings.Contains(strings.ToLower(n.Title), strings.ToLower(query)) || strings.Contains(strings.ToLower(n.ID), strings.ToLower(query)) {
			items = append(items, CompletionItem{
				Label:      n.Title,
				Detail:     n.ID,
				InsertText: n.ID, // Insert ID (filename stem)
				Kind:       18,   // Reference
			})
		}
	}

	return CompletionList{IsIncomplete: false, Items: items}, nil
}

func (s *Server) handleDefinition(params DefinitionParams) ([]Location, error) {
	link, ok := s.getLinkAtPosition(params.TextDocumentPositionParams)
	if !ok {
		return nil, nil
	}

	// Link target is the ID.
	// But link might be [[ID|Alias]]. getLinkAtPosition should return just ID.
	targetID := strings.Split(link, "|")[0]
	targetID = strings.TrimSpace(targetID)

	note, err := s.store.GetNote(targetID)
	if err != nil {
		return nil, nil // Not found
	}

	// Convert Path to URI
	fullPath := filepath.Join(s.rootDir, note.Path)
	uri := "file://" + fullPath // Primitive URI encoding

	return []Location{
		{
			URI: uri,
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 0, Character: 0},
			},
		},
	}, nil
}

func (s *Server) handleHover(params HoverParams) (*Hover, error) {
	link, ok := s.getLinkAtPosition(params.TextDocumentPositionParams)
	if !ok {
		return nil, nil
	}

	targetID := strings.Split(link, "|")[0]
	targetID = strings.TrimSpace(targetID)

	note, err := s.store.GetNote(targetID)
	if err != nil {
		return nil, nil
	}

	// Read content from file
	contentBytes, err := os.ReadFile(filepath.Join(s.rootDir, note.Path))
	content := ""
	if err == nil {
		content = string(contentBytes)
	}

	// Prepare Hover Content
	md := fmt.Sprintf("**%s**\n\nID: `%s`\n\n%s", note.Title, note.ID, truncate(content, 200))

	return &Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: md,
		},
	}, nil
}

// Helpers

// getLinkAtPosition extracts "Link" from [[Link]] at cursor.
func (s *Server) getLinkAtPosition(params TextDocumentPositionParams) (string, bool) {
	content, ok := s.documents[params.TextDocument.URI]
	if !ok {
		return "", false
	}

	lines := strings.Split(content, "\n")
	if params.Position.Line >= len(lines) {
		return "", false
	}
	line := lines[params.Position.Line]
	cursor := params.Position.Character
	
	// Find all [[...]] in line
	re := regexp.MustCompile(`\[\[(.*?)\]\]`)
	matches := re.FindAllStringSubmatchIndex(line, -1)

	for _, m := range matches {
		// m[0] = start of [[, m[1] = end of ]]
		// m[2] = start of content, m[3] = end of content
		if cursor >= m[0] && cursor <= m[1] {
			return line[m[2]:m[3]], true
		}
	}
	
	return "", false
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
