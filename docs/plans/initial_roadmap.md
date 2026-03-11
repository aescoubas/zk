# Zettelkasten Overhaul Roadmap
*Technomancer Standard v1.0*

## 1. Context Profile
*The Agent MUST read these files at the start of every session to understand the project state.*

### Static Context
*   `README.md`
*   `ROADMAP.md` (This file)
*   `ARCHITECTURE/000-readme.md` (Architecture overview)
*   `ARCHITECTURE/001-go-sqlite-engine.md` (SQLite engine ADR)
*   `ARCHITECTURE/002-mcp-integration.md` (MCP integration ADR)
*   `ARCHITECTURE/003-separate-tool-and-data-repositories.md` (Repo split ADR)
*   `ARCHITECTURE/004-move-maintenance-commands-into-zk.md` (Maintenance command ADR)
*   `ARCHITECTURE/005-separate-durable-state-from-derived-index.md` (Index/state separation ADR)
*   `go.mod` (Go dependencies)

### Dynamic Context (Source Code)
*   `cmd/zk/` (CLI commands & TUI models)
*   `internal/model/` (Data types: Note, Link, Ref, Citation)
*   `internal/store/` (SQLite storage layer)
*   `internal/parser/` (Markdown + frontmatter + WikiLinks + citations)
*   `internal/lsp/` (Language Server Protocol)
*   `internal/mcp/` (Model Context Protocol)
*   `internal/srs/` (Spaced Repetition algorithm)
*   `internal/llm/` (Ollama embeddings client)
*   `tools/editors/` and `tools/vim/` (Editor integration)
*   External data repo (typically `zettelkasten-data/`) for `zettels/`, `projects/`, `kanban/`, and `archive/`

### External Runtime Context
*   Root resolution precedence: `--dir`, `ZK_PATH`, `~/.config/zk/root`, current working directory when it contains `zettels/`
*   `.zk/index.db` is a rebuildable derived index
*   Local SRS state lives under `XDG_STATE_HOME` / `~/.local/state`

> Historical note: Phases 1-5 below were completed in the original combined repository before the tool and data were split.

---

## 2. Execution Phases

### Phase 1: Structural Foundation
**Status:** DONE
**Token Budget:** Low
**Prerequisites:** None

**Objective:**
Reorganize the physical file structure to support atomic notes and low-friction linking.

**Tasks:**
- [x] **Flatten Hierarchy:** Move all atomic notes from deep subdirectories (e.g., `zettels/linux_it/`, `zettels/german_learning/`) into a single flat `zettels/` directory to discourage categorical silo-ing.
- [x] **Archive Legacy:** Move `legacy_doc/` and `bibliographic_notes/` into an explicit `_archive/` or `references/` directory to separate "processed" thoughts from "raw" source material.
- [ ] **Naming Convention Standardization:** Define and strictly enforce a filename convention (e.g., `YYYYMMDDHHMM-slug_title.md`) that sorts chronologically but reads semantically. (Deferred — existing slugs work well enough)

**Verification:**
- [x] All atomic notes live in a single flat `zettels/` directory.
- [x] Legacy material is separated from active notes.

---

### Phase 2: Core Tooling (The "Engine")
**Status:** DONE
**Token Budget:** Medium
**Prerequisites:** Phase 1

**Objective:**
Develop bespoke CLI tools to manage the knowledge base, avoiding external "black box" dependencies.

**Tasks:**
- [x] **CLI Scaffolding:** Create a central entry point script (e.g., `zk`) in Python or Bash.
- [x] **Note Creation (`zk new`):**
    - [x] Auto-generate unique filenames/IDs.
    - [x] Apply standard frontmatter/templates.
- [x] **Search & Link (`zk search`):**
    - [x] Fast fuzzy-search (using `fzf` or internal logic) over note titles.
    - [x] Output formatted WikiLinks `[[Title]]` for insertion.
- [x] **Editor Integration:**
    - [x] **Vim:** Write custom `.vim` functions to invoke `zk` commands (e.g., `<leader>zn` for new, `<leader>zl` for link).

**Verification:**
- [x] `zk new` creates a properly templated note with a unique ID.
- [x] `zk search` returns matching notes and outputs WikiLink syntax.
- [x] Vim keybindings invoke `zk` commands seamlessly.

---

### Phase 3: Data Migration & Refining
**Status:** DONE
**Token Budget:** Medium
**Prerequisites:** Phase 1

**Objective:**
Convert existing data to the new fluid format.

**Tasks:**
- [x] **Link Migration Script:** Develop a script to parse all existing notes and convert UUID-style links `[Text][UUID]` to standard WikiLinks `[[Filename]]`.
- [ ] **Batch Renaming:** Script to rename existing files to the new naming convention while updating all references to them. (Optional/Skipped for legacy preservation)
- [x] **The "Refinery" Workflow:**
    - [x] Create a tool to easily "extract" a selection of text from a legacy note into a new atomic note and leave a link behind. (Handled via `zk new`)

**Verification:**
- [x] All UUID-style links are converted to WikiLinks.
- [x] Extraction workflow produces atomic notes with back-references.

---

### Phase 4: Visualization & Emergence
**Status:** DONE
**Token Budget:** Medium
**Prerequisites:** Phase 2

**Objective:**
Build tools to answer "What connects to what?" and spark new ideas through serendipity.

**Tasks:**
- [x] **Graph Visualization:**
    - [x] Generate a `graph.json` representing nodes (notes) and edges (links).
    - [x] Create a minimal local HTML/D3.js viewer to render the network.
- [x] **Serendipity Tools:**
    - [x] **Random Walk:** Command to open a random note to spark rediscovery.
    - [x] **Orphan Detector:** List notes with zero incoming links.
    - [x] **Stale Data Cleaner:** Identify notes untouched for >1 year.

**Verification:**
- [x] `graph.json` renders a navigable network in the browser.
- [x] Random walk opens an arbitrary note successfully.
- [x] Orphan detector lists unlinked notes.

---

### Phase 5: Quality & Reliability
**Status:** DONE
**Token Budget:** Low
**Prerequisites:** Phase 2

**Objective:**
Ensure robustness and maintainability of the knowledge base.

**Tasks:**
- [x] **Linter:** Script to validate frontmatter, check for broken links, and ensure file naming compliance.
- [x] **Automated Backup:** Setup Git hooks to auto-commit changes on a schedule or event.
- [x] **Documentation:** Maintain a meta-note describing the new system's architecture and usage.

**Verification:**
- [x] Linter catches broken links and invalid frontmatter.
- [x] Git hooks auto-commit on schedule.

---

## The "Blessed Machine" (Go Rewrite)

*Transitioning from scripts to a robust, compiled software architecture.*

---

### Phase 6: The Foundation & Indexing
**Status:** DONE
**Token Budget:** High
**Prerequisites:** None

**Objective:**
Establish the Go project structure and a high-performance SQLite shadow index for the knowledge base.

**Tasks:**
- [x] **Project Initialization:**
    - [x] Initialize Go module `github.com/escoubas/zk`.
    - [x] Define core structs: `Note`, `Link`, `Metadata`.
- [x] **SQLite Shadow Index:**
    - [x] Setup `go-sqlite3` or modern pure-Go variant.
    - [x] Schema Design: Tables for `notes` (id, path, hash), `links` (source, target), `tags`.
    - [x] **FTS5 Integration:** Set up virtual table for instant full-text search.
- [x] **Parser & Watcher:**
    - [x] Implement `goldmark` parser to extract frontmatter, content, and `[[wikilinks]]`.
    - [x] Implement file walker to index all existing notes on startup.
    - [x] Implement `fsnotify` watcher to update the index incrementally on file save.

**Verification:**
- [x] `go build` produces a working binary.
- [x] Indexing ~166 notes populates the SQLite database.
- [x] FTS5 returns results for content queries.

---

### Phase 7: Architectural Documentation
**Status:** DONE
**Token Budget:** Low
**Prerequisites:** Phase 6

**Objective:**
Ensure the system's design is well-documented and decisions are tracked via Architecture Decision Records.

**Tasks:**
- [x] **Architecture Records:** Create `ARCHITECTURE/` directory and populate with initial ADRs.
- [x] **Maintenance Policy:** Ensure README and workflows mandate updating ADRs on significant changes.

**Verification:**
- [x] `ARCHITECTURE/` directory contains ADRs for core decisions.

---

### Phase 8: The Interface Layer (CLI & TUI)
**Status:** DONE
**Token Budget:** High
**Prerequisites:** Phase 6

**Objective:**
Replace the Python scripts with a unified Go binary and add a rich terminal UI using Bubble Tea.

**Tasks:**
- [x] **CLI Replacement:**
    - [x] Port `zk new`, `zk link`, `zk random`, `zk stale` to Go using `cobra`.
    - [x] Ensure parity with existing Vim integration.
- [x] **Interactive TUI (The Navigator):**
    - [x] Build a "File Manager for Ideas" using `bubbletea`.
    - [x] Feature: Browsable list of notes with metadata columns (tags, links count).
    - [x] Feature: Instant fuzzy filter (title + content).
    - [ ] Feature: Split-pane preview of the selected note. (Deferred)

**Verification:**
- [x] All original `zk` commands work via the Go binary.
- [x] `zk tui` launches the TUI with a browsable note list.
- [x] Fuzzy filter narrows results in real time.

---

### Phase 9: Deep Integration (LSP)
**Status:** DONE
**Token Budget:** High
**Prerequisites:** Phase 6

**Objective:**
Make the tool editor-agnostic by implementing the Language Server Protocol for WikiLink navigation.

**Tasks:**
- [x] **LSP Skeleton:**
    - [x] Implement basic JSON-RPC 2.0 handling over Stdio.
    - [x] Handle `initialize` and `textDocument/didOpen` events.
- [x] **Features:**
    - [x] **Go to Definition:** `textDocument/definition` handling for `[[WikiLinks]]`.
    - [x] **Autocompletion:** `textDocument/completion` offering note titles when typing `[[`.
    - [x] **Hover:** `textDocument/hover` showing note preview when hovering a link.
- [x] **Editor Config:** Write config snippets for Neovim (native LSP) and VS Code (Generic LSP Client).

**Verification:**
- [x] Hovering a `[[WikiLink]]` in Neovim shows a note preview.
- [x] Typing `[[` triggers autocompletion with note titles.
- [x] Ctrl-clicking a link navigates to the target note.

---

### Phase 10: Intelligence & Emergence
**Status:** DONE
**Token Budget:** High
**Prerequisites:** Phase 6

**Objective:**
Leverage the index for advanced discovery features: semantic search and spaced repetition review.

**Tasks:**
- [x] **Semantic Search (Vector Embeddings):**
    - [x] Integrate a local embedding model (via `ollama` client in Go).
    - [x] Generate vectors for all notes and store in SQLite (using JSON/BLOB).
    - [x] Implement `zk similar` command to find conceptually related notes.
- [x] **Spaced Repetition System (SRS):**
    - [x] Algorithm: Implement a review scheduler (SM-2).
    - [x] `zk review`: A specialized TUI mode for reviewing due/stale notes.

**Verification:**
- [x] `zk similar <note>` returns conceptually related notes ranked by similarity.
- [x] `zk review` presents due notes with quality rating buttons.

---

### Phase 11: Quality Assurance & Reliability
**Status:** DONE
**Token Budget:** Medium
**Prerequisites:** Phase 8, Phase 10

**Objective:**
Harden the codebase with comprehensive testing and consistency checks.

**Tasks:**
- [x] **Unit Testing:**
    - [x] Add unit tests for core logic (parser, SRS algorithm, model serialization).
    - [x] Add unit tests for `store` package (SQLite interactions).
- [x] **Integration Testing:**
    - [x] Implement end-to-end CLI tests (using a temporary test directory/DB).
    - [x] Test full workflows: Create -> Index -> Link -> Search -> Review.
- [x] **Code Quality:**
    - [x] Run linters (`golangci-lint`) and fix issues.
    - [x] Ensure consistent error handling and logging throughout.

**Verification:**
- [x] `go test ./...` passes all unit and integration tests.
- [x] Linter reports zero critical issues.

---

### Phase 12: External Integration (MCP)
**Status:** DONE
**Token Budget:** Medium
**Prerequisites:** Phase 6

**Objective:**
Implement the Model Context Protocol (MCP) to allow LLM agents (like Gemini CLI) to interact directly with the Zettelkasten.

**Tasks:**
- [x] **MCP Server:**
    - [x] Implement an MCP-compliant server (stdio or HTTP transport).
    - [x] Expose **Resources:** Allow reading notes as resources (`zettel://<id>`).
    - [x] Expose **Prompts:** Create standard prompts (e.g., "Summarize Note", "Find Connections").
- [x] **Tools Integration:**
    - [x] Expose `zk search`, `zk similar`, `zk new`, and `zk link` as MCP tools.
    - [x] Allow the agent to query the graph structure.

**Verification:**
- [x] MCP server starts and responds to tool invocations.
- [x] An LLM agent can search, read, and create notes via MCP.

---

### Phase 13: UI/UX Refinement & Terminal Native Visualization
**Status:** DONE
**Token Budget:** High
**Prerequisites:** Phase 8

**Objective:**
Move visualization and interaction entirely into the terminal for a seamless, Gruvbox-themed workflow.

**Tasks:**
- [x] **Terminal Graph Explorer:**
    - [x] Implement a TUI-based graph viewer (using `bubbletea` or similar) to visualize nodes and edges directly in the terminal.
    - [x] Allow navigation of the graph (panning/zooming or traversing nodes) via keyboard.
- [x] **Interactive Dashboard:**
    - [x] Create a "Home" dashboard TUI showing stats, recent notes, and random entry points.
- [x] **Usability Polish:**
    - [x] Standardize keyboard shortcuts across all TUI modes.
    - [x] Improve error messages and help text for all commands.

**Verification:**
- [x] `zk tui` opens a dashboard with stats and quick actions.
- [x] Graph explorer renders nodes and edges navigable by keyboard.
- [x] Help text (`?`) is consistent across all TUI views.

---

### Phase 14: Enhanced Navigation and Listing
**Status:** DONE
**Token Budget:** Medium
**Prerequisites:** Phase 8

**Objective:**
Refine the core exploration and listing capabilities for better usability.

**Tasks:**
- [x] **Explore View Enhancements:**
    - [x] **Root Index:** Open the root of the zettelkasten with an index page that links various sections.
    - [x] **Syntax Highlighting:** Use syntax highlighting when displaying a note in the explore view.
- [x] **`zk list` Enhancements:**
    - [x] **Consistent Formatting:** Ensure the output format is consistent with the rest of the UI.
    - [x] **Rich Columns:** Display creation date, last modification date, topic, backlinks count, and outgoing links count.

**Verification:**
- [x] The explore view opens the index page by default.
- [x] `zk list` renders a rich table with all metadata columns.

---

### Phase 15: Navigation Walk Graph — Data Model & Session Tracking
**Status:** DONE
**Token Budget:** Medium
**Prerequisites:** Phase 14

**Objective:**
Build the in-memory branching tree that captures the full "walk" through the zettelkasten during a session, replacing the flat history stack.

**Tasks:**
- [x] **Walk Graph Data Structure:** Define a tree structure (`walkGraph` with `walkNode` structs) holding node ID, note reference, children, parent pointer, visit timestamp, and edge label (link type).
- [x] **Session-Scoped Lifecycle:** Instantiate the walk graph when the navigator starts; pass it to the explore model. No persistence — it dies with the process.
- [x] **Record Navigation Events:** On every "follow link" action in explore (Enter on backlink/outgoing/similar/citation), append a child node to the current position in the walk graph.
- [x] **Handle Backtracking & Branching:** When the user presses Backspace (back), move the current-position cursor to the parent node. The next "follow link" creates a *new sibling* branch rather than overwriting the old path.
- [x] **Graph-Jump vs Organic Navigation:** Add a `jumpTo(nodeID)` method that moves the explore view's current note *and* the walk graph cursor to an existing node, without creating new edges. Only organic navigation (following links from a note's panels) creates new nodes/edges.
- [x] **Replace Flat History Stack:** The walk graph's parent pointers subsume the `history []string` slice. Back = move to parent; the old stack is no longer needed.

**Verification:**
- [x] Walk graph correctly records branching paths when navigating back and forward.
- [x] `jumpTo` repositions cursor without adding edges.
- [x] Unit tests pass for all graph operations (`walkgraph_test.go`).

---

### Phase 16: Navigation Walk Graph — TUI Visualization & Interaction
**Status:** DONE
**Token Budget:** High
**Prerequisites:** Phase 15

**Objective:**
A new TUI view rendering the walk graph as a scrollable ASCII tree with node selection and jump-to-note.

**Tasks:**
- [x] **Tree Layout Algorithm:** Implement a layout pass that assigns (x, y) coordinates to each node in the walk graph, producing a top-down tree with box-drawing connectors (`│ ─ ┬ ├ └ ╴`).
- [x] **Graph View Model:** New `walkGraphModel` implementing `Init/Update/View`, registered as a new navigator state (`stateWalkGraph`) with a `navigateToWalkGraphMsg`.
- [x] **Rendering:** Render each node as a compact label (truncated title, ~30 chars). Highlight the current-position node distinctly (bold/color). Use Gruvbox palette consistent with the rest of the TUI.
- [x] **Scrollable Viewport:** Embed in a `viewport.Model` so tall/wide graphs can be scrolled vertically and horizontally.
- [x] **Cursor Navigation:** `j/k` (or arrows) to move selection between nodes in depth-first order. Visual indicator on the selected node.
- [x] **Jump-to-Node:** Press Enter on a selected node to jump back to explore view on that note (via `jumpTo`, no graph mutation).
- [x] **Access from Explore:** Press `g` in explore to open the walk graph view. Press `q`/`Esc` in graph view to return to explore at the current position.
- [x] **Stats Footer:** Show total nodes visited, max depth, number of branches in a status bar.

**Verification:**
- [x] `g` from explore opens the walk graph with correct tree rendering.
- [x] Cursor navigation and jump-to-node work correctly.
- [x] Unit tests pass for tree layout, truncation, and edge indicators (`walkgraphview_test.go`).

---

### Phase 17: Repository Split
**Status:** DONE
**Token Budget:** Medium
**Prerequisites:** Phase 8

**Objective:**
Separate the `zk` toolchain from the Zettelkasten content so the binary and editor integrations can target an external data repository.

**Tasks:**
- [x] **Tool Repo Extraction:** Keep the Go module, install workflow, editor integrations, and ADRs in the dedicated `zk` repository.
- [x] **External Data Root Resolution:** Support data-root discovery via `--dir`, `ZK_PATH`, `~/.config/zk/root`, and current working directory fallback only when the directory clearly looks like a data root.
- [x] **Split-Aware Documentation:** Update build, install, and editor setup docs to reflect the dedicated tool repo plus external data repo workflow.

**Verification:**
- [x] `go test -tags fts5 ./...` passes in the `zk` repository.
- [x] `zk` commands resolve an external data repository without relying on the old monorepo layout.

---

### Phase 18: Maintenance Command Consolidation
**Status:** DONE
**Token Budget:** Medium
**Prerequisites:** Phase 17

**Objective:**
Make linting and graph generation part of the Go toolchain so a system-wide `zk` install can operate against a configured external data repository without Python scripts.

**Tasks:**
- [x] Add a first-class `zk lint` command for dead links and orphan reporting in `zettels/`.
- [x] Add a first-class `zk graph` command that writes standalone HTML from embedded template data.
- [x] Support explicit data-root configuration during installation.
- [x] Remove Python maintenance scripts from the companion data repository.

**Verification:**
- [x] `go test -tags fts5 ./...` passes in the `zk` repository.
- [x] `zk --dir <repo> lint` runs against the external data repository without Python.
- [x] `zk --dir <repo> graph --output <path>` generates standalone graph HTML.
