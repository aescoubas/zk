# Zettelkasten Overhaul Roadmap

### Phase 1: Structural Foundation
Reorganizing the physical file structure to support atomic notes and low-friction linking.
- [x] **Flatten Hierarchy**: Move all atomic notes from deep subdirectories (e.g., `permanent_notes/linux_it/`, `permanent_notes/german_learning/`) into a single flat `permanent_notes/` directory to discourage categorical silo-ing.
- [x] **Archive Legacy**: Move `legacy_doc/` and `bibliographic_notes/` into an explicit `_archive/` or `references/` directory to separate "processed" thoughts from "raw" source material.
- [ ] **Naming Convention Standardization**: Define and strictly enforce a filename convention (e.g., `YYYYMMDDHHMM-slug_title.md`) that sorts chronologically but reads semantically.

### Phase 2: Core Tooling (The "Engine")
Developing bespoke CLI tools to manage the knowledge base, avoiding external "black box" dependencies.
- [x] **CLI Scaffolding**: Create a central entry point script (e.g., `zk`) in Python or Bash.
- [x] **Note Creation (`zk new`)**:
    - [x] Auto-generate unique filenames/IDs.
    - [x] Apply standard frontmatter/templates.
- [x] **Search & Link (`zk search`)**:
    - [x] Fast fuzzy-search (using `fzf` or internal logic) over note titles.
    - [x] Output formatted WikiLinks `[[Title]]` for insertion.
- [x] **Editor Integration**:
    - [x] **Vim**: Write custom `.vim` functions to invoke `zk` commands (e.g., `<leader>zn` for new, `<leader>zl` for link).

### Phase 3: Data Migration & Refining
Converting existing data to the new fluid format.
- [x] **Link Migration Script**: Develop a script to parse all existing notes and convert UUID-style links `[Text][UUID]` to standard WikiLinks `[[Filename]]`.
- [ ] **Batch Renaming**: Script to rename existing files to the new naming convention while updating all references to them. (Optional/Skipped for legacy preservation)
- [x] **The "Refinery" Workflow**:
    - [x] Create a tool to easily "extract" a selection of text from a legacy note into a new atomic note and leave a link behind. (Handled via `zk new`)

### Phase 4: Visualization & Emergence
Tools to answer "What connects to what?" and spark new ideas.
- [x] **Graph Visualization**:
    - [x] Generate a `graph.json` representing nodes (notes) and edges (links).
    - [x] Create a minimal local HTML/D3.js viewer to render the network.
- [x] **Serendipity Tools**:
    - [x] **Random Walk**: Command to open a random note to spark rediscovery.
    - [x] **Orphan Detector**: List notes with zero incoming links.
    - [x] **Stale Data Cleaner**: Identify notes untouched for >1 year.

### Phase 5: Quality & Reliability
Ensuring robustness and maintainability of the knowledge base.
- [x] **Linter**: Script to validate frontmatter, check for broken links, and ensure file naming compliance.
- [x] **Automated Backup**: Setup Git hooks to auto-commit changes on a schedule or event.
- [x] **Documentation**: Maintain a meta-note describing the new system's architecture and usage.

---
## The "Blessed Machine" (Go Rewrite)

Transitioning from scripts to a robust, compiled software architecture.

### Phase 6: The Foundation & Indexing
Establishing the project structure and the high-performance local database.
- [x] **Project Initialization**:
    - [x] Initialize Go module `github.com/escoubas/zk`.
    - [x] Define core structs: `Note`, `Link`, `Metadata`.
- [x] **SQLite Shadow Index**:
    - [x] Setup `go-sqlite3` or modern pure-Go variant.
    - [x] Schema Design: Tables for `notes` (id, path, hash), `links` (source, target), `tags`.
    - [x] **FTS5 Integration**: Set up virtual table for instant full-text search.
- [x] **Parser & Watcher**:
    - [x] Implement `goldmark` parser to extract frontmatter, content, and `[[wikilinks]]`.
    - [x] Implement file walker to index all existing notes on startup.
    - [x] Implement `fsnotify` watcher to update the index incrementally on file save.

### Phase 7: Architectural Documentation
Ensuring the system's design is well-documented and decisions are tracked.
- [x] **Architecture Records**: Create `ARCHITECTURE/` directory and populate with initial ADRs.
- [x] **Maintenance Policy**: Ensure README and workflows mandate updating ADRs on significant changes.

### Phase 8: The Interface Layer (CLI & TUI)
Replacing the Python scripts with a unified binary and adding a rich terminal UI.
- [x] **CLI Replacement**:
    - [x] Port `zk new`, `zk link`, `zk random`, `zk stale` to Go using `cobra`.
    - [x] Ensure parity with existing Vim integration.
- [x] **Interactive TUI (The Navigator)**:
    - [x] Build a "File Manager for Ideas" using `bubbletea`.
    - [x] Feature: Browsable list of notes with metadata columns (tags, links count).
    - [x] Feature: Instant fuzzy filter (title + content).
    - [ ] Feature: Split-pane preview of the selected note. (Deferred)

### Phase 9: Deep Integration (LSP)
Making the tool editor-agnostic by implementing the Language Server Protocol.
- [x] **LSP Skeleton**:
    - [x] Implement basic JSON-RPC 2.0 handling over Stdio.
    - [x] Handle `initialize` and `textDocument/didOpen` events.
- [x] **Features**:
    - [x] **Go to Definition**: `textDocument/definition` handling for `[[WikiLinks]]`.
    - [x] **Autocompletion**: `textDocument/completion` offering note titles when typing `[[`.
    - [x] **Hover**: `textDocument/hover` showing note preview when hovering a link.
- [x] **Editor Config**: Write config snippets for Neovim (native LSP) and VS Code (Generic LSP Client).

### Phase 10: Intelligence & Emergence
Leveraging the index for advanced discovery features.
- [x] **Semantic Search (Vector Embeddings)**:
    - [x] Integrate a local embedding model (via `ollama` client in Go).
    - [x] Generate vectors for all notes and store in SQLite (using JSON/BLOB).
    - [x] Implement `zk similar` command to find conceptually related notes.
- [x] **Spaced Repetition System (SRS)**:
    - [x] Algorithm: Implement a review scheduler (SM-2).
    - [x] `zk review`: A specialized TUI mode for reviewing due/stale notes.

### Phase 11: Quality Assurance & Reliability
Hardening the codebase with comprehensive testing and consistency checks.
- [x] **Unit Testing**:
    - [x] Add unit tests for core logic (parser, SRS algorithm, model serialization).
    - [x] Add unit tests for `store` package (SQLite interactions).
- [x] **Integration Testing**:
    - [x] Implement end-to-end CLI tests (using a temporary test directory/DB).
    - [x] Test full workflows: Create -> Index -> Link -> Search -> Review.
- [x] **Code Quality**:
    - [x] Run linters (`golangci-lint`) and fix issues.
    - [x] Ensure consistent error handling and logging throughout.

### Phase 12: External Integration (MCP)
Implementing the Model Context Protocol (MCP) to allow LLM agents (like Gemini CLI) to interact directly with the Zettelkasten.
- [x] **MCP Server**:
    - [x] Implement an MCP-compliant server (stdio or HTTP transport).
    - [x] Expose **Resources**: Allow reading notes as resources (`zettel://<id>`).
    - [x] Expose **Prompts**: Create standard prompts (e.g., "Summarize Note", "Find Connections").
- [x] **Tools Integration**:
    - [x] Expose `zk search`, `zk similar`, `zk new`, and `zk link` as MCP tools.
    - [x] Allow the agent to query the graph structure.

### Phase 13: UI/UX Refinement & Terminal Native Visualization
Moving visualization and interaction entirely into the terminal for a seamless workflow.
- [x] **Terminal Graph Explorer**:
    - [x] Implement a TUI-based graph viewer (using `bubbletea` or similar) to visualize nodes and edges directly in the terminal.
    - [x] Allow navigation of the graph (panning/zooming or traversing nodes) via keyboard.
- [x] **Interactive Dashboard**:
    - [x] Create a "Home" dashboard TUI showing stats, recent notes, and random entry points.
- [x] **Usability Polish**:
    - [x] Standardize keyboard shortcuts across all TUI modes.
    - [x] Improve error messages and help text for all commands.

### Phase 14: Enhanced Navigation and Listing
Refining the core exploration and listing capabilities for better usability.
- [x] **`zk explore` Enhancements**:
    - [x] **Root Index**: Open the root of the zettelkasten with an index page that links various sections.
    - [x] **Syntax Highlighting**: Use syntax highlighting when displaying a note in the explore view.
- [x] **`zk list` Enhancements**:
    - [x] **Consistent Formatting**: Ensure the output format is consistent with the rest of the UI.
    - [x] **Rich Columns**: Display the following columns:
        - Creation date
        - Last modification date
        - Topic
        - Number of backlinks (incoming links)
        - Number of outgoing links

### Phase 15: Graph Connectivity & Traversal
Ensuring the Zettelkasten is a fully connected graph starting from a clear entry point.
- [ ] **Root Node Implementation**: Ensure `zk explore` opens a dedicated "Index" or "Root" note that explicitly links to major entry points/topics, rather than an empty or disconnected view.
- [ ] **Graph Integrity**: Achieve full connectivity where every note is reachable from the root (eliminate "islands" recursively).
