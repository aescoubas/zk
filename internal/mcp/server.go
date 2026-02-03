package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/escoubas/zk/internal/llm"
	"github.com/escoubas/zk/internal/parser"
	"github.com/escoubas/zk/internal/store"
)

type Server struct {
	store   *store.Store
	rootDir string
}

func NewServer(st *store.Store, root string) *Server {
	return &Server{
		store:   st,
		rootDir: root,
	}
}

func (s *Server) Serve() error {
	scanner := bufio.NewScanner(os.Stdin)
	// Increase buffer size to handle large payloads
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var base Request
		if err := json.Unmarshal(line, &base); err != nil {
			log.Printf("Error decoding request: %v", err)
			continue
		}

		// Dispatch
		if base.Method != "" {
			// Is it a request (has ID) or notification?
			if base.ID != nil {
				resp, err := s.handleRequest(base.Method, base.Params)
				response := Response{
					JSONRPC: "2.0",
					ID:      base.ID,
				}
				if err != nil {
					response.Error = err
				} else {
					response.Result = resp
				}
				
				bytes, _ := json.Marshal(response)
				fmt.Println(string(bytes))
			} else {
				// Notification
				s.handleNotification(base.Method, base.Params)
			}
		}
	}
	return scanner.Err()
}

func (s *Server) handleRequest(method string, params json.RawMessage) (interface{}, *RPCError) {
	switch method {
	case "initialize":
		var p InitializeParams
		json.Unmarshal(params, &p)
		return s.handleInitialize(p)
	case "resources/list":
		return s.handleListResources()
	case "resources/read":
		var p ReadResourceParams
		json.Unmarshal(params, &p)
		return s.handleReadResource(p)
	case "tools/list":
		return s.handleListTools()
	case "tools/call":
		var p CallToolParams
		json.Unmarshal(params, &p)
		return s.handleCallTool(p)
	case "prompts/list":
		return s.handleListPrompts()
	case "prompts/get":
		var p GetPromptParams
		json.Unmarshal(params, &p)
		return s.handleGetPrompt(p)
	case "ping":
		return struct{}{}, nil
	}
	return nil, &RPCError{Code: -32601, Message: "Method not found: " + method}
}

func (s *Server) handleNotification(method string, params json.RawMessage) {
	// Handle notifications if needed
}

// Handlers

func (s *Server) handleInitialize(params InitializeParams) (InitializeResult, *RPCError) {
	return InitializeResult{
		ProtocolVersion: "2024-11-05", 
		Capabilities: ServerCapabilities{
			Resources: &ResourcesCapability{ListChanged: false},
			Tools:     &ToolsCapability{ListChanged: false},
			Prompts:   &PromptsCapability{ListChanged: false},
		},
		ServerInfo: Implementation{
			Name:    "zk-mcp",
			Version: "0.1.0",
		},
	}, nil
}

func (s *Server) handleListResources() (ListResourcesResult, *RPCError) {
	notes, err := s.store.ListNotes()
	if err != nil {
		return ListResourcesResult{}, &RPCError{Code: -32000, Message: err.Error()}
	}

	var resources []Resource
	for _, n := range notes {
		resources = append(resources, Resource{
			URI:  "zettel://" + n.ID,
			Name: n.Title,
			MimeType: "text/markdown",
		})
	}
	return ListResourcesResult{Resources: resources}, nil
}

func (s *Server) handleReadResource(params ReadResourceParams) (ReadResourceResult, *RPCError) {
	if !strings.HasPrefix(params.URI, "zettel://") {
		return ReadResourceResult{}, &RPCError{Code: -32002, Message: "Invalid URI scheme"}
	}
	id := strings.TrimPrefix(params.URI, "zettel://")
	
	note, err := s.store.GetNote(id)
	if err != nil {
		return ReadResourceResult{}, &RPCError{Code: -32002, Message: "Note not found"}
	}

	contentBytes, err := os.ReadFile(filepath.Join(s.rootDir, note.Path))
	if err != nil {
		return ReadResourceResult{}, &RPCError{Code: -32000, Message: "Failed to read file"}
	}

	return ReadResourceResult{
		Contents: []ResourceContent{
			{
				URI:      params.URI,
				MimeType: "text/markdown",
				Text:     string(contentBytes),
			},
		},
	}, nil
}

func (s *Server) handleListTools() (ListToolsResult, *RPCError) {
	return ListToolsResult{
		Tools: []Tool{
			{
				Name:        "search_notes",
				Description: "Search for notes by keyword or phrase using full-text search.",
				InputSchema: json.RawMessage(`{
					"type": "object",
					"properties": {
						"query": { "type": "string", "description": "The search query" }
					},
					"required": ["query"]
				}`),
			},
			{
				Name:        "find_similar_notes",
				Description: "Find semantically similar notes using vector embeddings.",
				InputSchema: json.RawMessage(`{
					"type": "object",
					"properties": {
						"note_id": { "type": "string", "description": "The ID of the note to find similarities for" }
					},
					"required": ["note_id"]
				}`),
			},
			{
				Name:        "create_note",
				Description: "Create a new note with the given title and content.",
				InputSchema: json.RawMessage(`{
					"type": "object",
					"properties": {
						"title": { "type": "string", "description": "Title of the note" },
						"content": { "type": "string", "description": "Content of the note (Markdown)" },
						"tags": { "type": "array", "items": { "type": "string" }, "description": "Tags for the note" }
					},
					"required": ["title", "content"]
				}`),
			},
		},
	}, nil
}

func (s *Server) handleCallTool(params CallToolParams) (CallToolResult, *RPCError) {
	switch params.Name {
	case "search_notes":
		var args struct {
			Query string `json:"query"`
		}
		json.Unmarshal(params.Arguments, &args)
		
		notes, err := s.store.SearchNotes(args.Query)
		if err != nil {
			return CallToolResult{IsError: true, Content: []Content{{Type: "text", Text: err.Error()}}}, nil
		}
		
		var sb strings.Builder
		for _, n := range notes {
			sb.WriteString(fmt.Sprintf("- [%s](%s) (ID: %s)\n", n.Title, n.ID, n.ID))
		}
		if len(notes) == 0 {
			sb.WriteString("No matches found.")
		}
		
		return CallToolResult{Content: []Content{{Type: "text", Text: sb.String()}}}, nil

	case "find_similar_notes":
		var args struct {
			NoteID string `json:"note_id"`
		}
		json.Unmarshal(params.Arguments, &args)
		
		targetEmb, err := s.store.GetEmbedding(args.NoteID)
		if err != nil {
			return CallToolResult{IsError: true, Content: []Content{{Type: "text", Text: "Embedding not found for note. Run 'zk embed' first."}}}, nil
		}
		
		allEmbs, err := s.store.GetAllEmbeddings()
		if err != nil {
			return CallToolResult{IsError: true, Content: []Content{{Type: "text", Text: err.Error()}}}, nil
		}
		
		type match struct {
			ID    string
			Score float64
		}
		var matches []match
		for _, e := range allEmbs {
			if e.NoteID == args.NoteID {
				continue
			}
			score := llm.CosineSimilarity(targetEmb.Vector, e.Vector)
			matches = append(matches, match{ID: e.NoteID, Score: score})
		}
		sort.Slice(matches, func(i, j int) bool { return matches[i].Score > matches[j].Score })
		
		var sb strings.Builder
		limit := 10
		if len(matches) < limit { limit = len(matches) }
		
		for i := 0; i < limit; i++ {
			m := matches[i]
			n, _ := s.store.GetNote(m.ID)
			title := m.ID
			if n != nil { title = n.Title }
			sb.WriteString(fmt.Sprintf("- %.4f %s (ID: %s)\n", m.Score, title, m.ID))
		}
		return CallToolResult{Content: []Content{{Type: "text", Text: sb.String()}}}, nil

	case "create_note":
		var args struct {
			Title   string   `json:"title"`
			Content string   `json:"content"`
			Tags    []string `json:"tags"`
		}
		json.Unmarshal(params.Arguments, &args)
		
		slug := slugify(args.Title)
		timestamp := time.Now().Format("200601021504")
		filename := fmt.Sprintf("%s-%s.md", timestamp, slug)
		
		// Create in zettels
		relDir := "zettels"
		fullPath := filepath.Join(s.rootDir, relDir, filename)
		// Ensure dir exists
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		
		tagsStr := ""
		if len(args.Tags) > 0 {
			tagsStr = fmt.Sprintf("tags: [%s]", strings.Join(args.Tags, ", "))
		} else {
			tagsStr = "tags: []"
		}
		
		fileContent := fmt.Sprintf("---\ntitle: %s\ndate: %s\n%s\n---\n\n%s", 
			args.Title, time.Now().Format("2006-01-02"), tagsStr, args.Content)
			
		if err := os.WriteFile(fullPath, []byte(fileContent), 0644); err != nil {
			return CallToolResult{IsError: true, Content: []Content{{Type: "text", Text: err.Error()}}}, nil
		}

		// Index immediately
		p := parser.NewParser()
		note, err := p.ParseFile(s.rootDir, filepath.Join(relDir, filename))
		if err == nil {
			if err := s.store.IndexNote(note); err != nil {
				// Log but don't fail tool call?
				log.Printf("Failed to index created note: %v", err)
			}
		} else {
			log.Printf("Failed to parse created note: %v", err)
		}
		
		return CallToolResult{Content: []Content{{Type: "text", Text: "Created note: " + fullPath}}}, nil
	}

	return CallToolResult{IsError: true, Content: []Content{{Type: "text", Text: "Tool not found"}}}, nil
}

func (s *Server) handleListPrompts() (ListPromptsResult, *RPCError) {
	return ListPromptsResult{
		Prompts: []Prompt{
			{
				Name: "summarize_note",
				Description: "Summarize a specific note.",
				Arguments: []PromptArgument{
					{Name: "note_id", Description: "ID of the note to summarize", Required: true},
				},
			},
			{
				Name: "find_connections",
				Description: "Find connections for a specific note.",
				Arguments: []PromptArgument{
					{Name: "note_id", Description: "ID of the note", Required: true},
				},
			},
		},
	}, nil
}

func (s *Server) handleGetPrompt(params GetPromptParams) (GetPromptResult, *RPCError) {
	switch params.Name {
	case "summarize_note":
		noteID := params.Arguments["note_id"]
		res, err := s.handleReadResource(ReadResourceParams{URI: "zettel://" + noteID})
		if err != nil {
			return GetPromptResult{}, err
		}
		text := res.Contents[0].Text
		
		return GetPromptResult{
			Description: "Summarize this note",
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: Content{
						Type: "text",
						Text: fmt.Sprintf("Please summarize the following note:\n\n%s", text),
					},
				},
			},
		}, nil
	case "find_connections":
		noteID := params.Arguments["note_id"]
		res, err := s.handleReadResource(ReadResourceParams{URI: "zettel://" + noteID})
		if err != nil {
			return GetPromptResult{}, err
		}
		text := res.Contents[0].Text
		
		return GetPromptResult{
			Description: "Find connections",
			Messages: []PromptMessage{
				{
					Role: "user",
					Content: Content{
						Type: "text",
						Text: fmt.Sprintf("Read the following note and suggest 3-5 existing notes in the Zettelkasten that might be related, based on the topics found in the note content. If possible, query for similar notes first.\n\nNote Content:\n%s", text),
					},
				},
			},
		}, nil
	}
	return GetPromptResult{}, &RPCError{Code: -32601, Message: "Prompt not found"}
}

func slugify(s string) string {
	s = strings.ToLower(s)
	reg := regexp.MustCompile("[^a-z0-9]+")
	s = reg.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
