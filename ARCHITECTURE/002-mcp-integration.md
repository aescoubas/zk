# 002. Model Context Protocol (MCP) Integration

**Date:** 2026-01-31

## Context
As LLM agents become more capable, allowing them to directly interact with the Zettelkasten enables powerful workflows like automated summarization, connection discovery, and assisted writing. The Model Context Protocol (MCP) is a standard for exposing data and tools to these agents.

## Decision
We decided to implement an MCP server directly within the `zk` binary, accessible via the `zk mcp` command.

- **Transport**: Standard Input/Output (stdio) using JSON-RPC 2.0, which is the standard for local MCP integrations.
- **Scope**:
    - **Resources**: Expose notes as `zettel://<id>` resources.
    - **Tools**: Expose `search_notes`, `find_similar_notes`, and `create_note` tools.
    - **Prompts**: Provide "Summarize Note" and "Find Connections" prompts.
- **Architecture**: The MCP server reuses the existing `internal/store` for database access and `internal/llm` for embeddings, ensuring consistency with the CLI and TUI.

## Consequences
- **Pros**:
    - **Agentic Workflows**: Agents can now autonomously explore and contribute to the Zettelkasten.
    - **Unified Binary**: No separate server process or binary required; `zk` does it all.
    - **Reusability**: Leverages the robust Go/SQLite engine and FTS5 search.
- **Cons**:
    - **Security**: Granting agents "create" permissions requires trust; currently, the tool allows note creation without explicit confirmation steps (relying on the agent's host environment for safety/sandboxing).
    - **Complexity**: Adds another protocol handler (JSON-RPC for MCP) alongside the existing LSP handler.
