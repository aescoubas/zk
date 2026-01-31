# 001. Switch to Go and SQLite Engine

**Date:** 2026-01-31

## Context
The initial Zettelkasten tools were a collection of Bash and Python scripts (`zk`, `build_graph.py`, `lint`). As the number of notes grows, these scripts (especially graph building and search) may suffer from performance issues. Additionally, we want a more robust, "blessed machine" foundation that can support advanced features like a TUI, Language Server Protocol (LSP), and semantic search.

## Decision
We decided to rewrite the core engine in **Go**, utilizing **SQLite** as the backend data store.

- **Language**: Go (Golang) was chosen for its performance, static typing, single-binary deployment, and excellent ecosystem for CLIs (`cobra`) and TUIs (`bubbletea`).
- **Database**: SQLite with `FTS5` (Full-Text Search) was chosen to provide instant search capabilities and a structured index of the notes graph without needing a heavy database server. The database acts as a "shadow index" that can be rebuilt from the Markdown files at any time.

## Consequences
- **Pros**:
    - Faster search and graph operations.
    - Single binary distribution (`bin/zk-go`).
    - Type safety and better maintainability.
    - Ecosystem ready for TUI and LSP.
- **Cons**:
    - Increased complexity compared to simple scripts.
    - Need to maintain synchronization between the filesystem (truth) and SQLite (cache).
    - Requires a compilation step.
