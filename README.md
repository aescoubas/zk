# zk

`zk` is the Go CLI, TUI, LSP, and MCP server for a Markdown-based zettelkasten. The note content now lives in a separate data repository such as `zettelkasten-data`.

## Repository Layout

- `cmd/zk/`: CLI entrypoints and TUI models
- `internal/`: parser, store, LSP, MCP, SRS, and embedding internals
- `tools/editors/`: editor integration snippets
- `tools/vim/`: classic Vim helper
- `ARCHITECTURE/`: ADRs for significant design decisions

## Build And Test

```bash
go build -tags fts5 -o bin/zk ./cmd/zk
go test -tags fts5 ./...
```

## Install

Install the binary, shell completions, and bundled user skills into `~/.local` by default:

```bash
./install.sh --data-dir /path/to/zettelkasten-data
```

The data root is resolved in this order:

1. `--dir`
2. `ZK_PATH`
3. `~/.config/zk/root`
4. current working directory, but only when it contains `zettels/`

If you do not set `--data-dir` or `ZK_DATA_DIR` during installation, configure the data repo later with either `ZK_PATH` or `~/.config/zk/root`.

For a system-wide install, pass both the install prefix and the data root explicitly:

```bash
./install.sh --prefix /usr/local --data-dir /path/to/zettelkasten-data
```

The installer also deploys the bundled `zk-project-journaler` skill into these user-local agent directories:

- `~/.codex/skills/zk-project-journaler`
- `~/.claude/skills/zk-project-journaler`
- `~/.gemini/skills/zk-project-journaler`

## Common Usage

```bash
zk --dir /path/to/zettelkasten-data tui
zk --dir /path/to/zettelkasten-data index
zk --dir /path/to/zettelkasten-data lint
zk --dir /path/to/zettelkasten-data graph
zk --dir /path/to/zettelkasten-data list
zk --dir /path/to/zettelkasten-data new "My Note"
zk --dir /path/to/zettelkasten-data lsp
zk --dir /path/to/zettelkasten-data mcp
```

Once `ZK_PATH` or `~/.config/zk/root` is set, the `--dir` flag becomes optional.

The interactive terminal UI is launched explicitly with `zk tui`. Running `zk` with no subcommand shows help.

If `zk` reports that the index is incompatible, run:

```bash
zk index
```

The command will rebuild the disposable `.zk/index.db` automatically when needed.

## Semantic Embedding

`zk embed` generates vector embeddings for every note using a local Ollama instance. These embeddings power `zk ask` (semantic Q&A) and `zk similar` (find related notes).

```bash
zk embed                    # embed new/changed notes (incremental)
zk embed --force            # re-embed all notes
zk embed --batch 100        # send 100 notes per batch request (default 50)
zk embed --model nomic-embed-text --url http://localhost:11434
```

Embedding uses the Ollama `/api/embed` batch endpoint, sending multiple notes per HTTP request. It also requests `num_thread = NumCPU - 2` so Ollama uses most available CPU cores for inference. Unchanged notes are skipped automatically via content-hash caching.

### Ollama Setup for Embedding

Install Ollama and pull the embedding model:

```bash
ollama pull nomic-embed-text
```

For better throughput on multi-core machines, set `OLLAMA_NUM_PARALLEL` in the Ollama service configuration (e.g. `/etc/systemd/system/ollama.service`):

```ini
Environment="OLLAMA_NUM_PARALLEL=4"
```

Then reload and restart:

```bash
sudo systemctl daemon-reload && sudo systemctl restart ollama
```

## Bundled Skill

`zk` ships with `zk-project-journaler`, a cross-agent skill that condenses daily Codex, Claude Code, and Gemini coding activity into a single zettel in the companion data repository.

Preview a note for a given day:

```bash
python3 ~/.codex/skills/zk-project-journaler/scripts/render_daily_zettel.py --date 2026-03-11 --zk-root /path/to/zettelkasten-data
```

Create or refresh the daily zettel:

```bash
python3 ~/.codex/skills/zk-project-journaler/scripts/render_daily_zettel.py --date 2026-03-11 --zk-root /path/to/zettelkasten-data --write-note
```

The generated note lands in `zettels/YYYYMMDD-agent-activity.md`, keeps structured metadata in frontmatter, and preserves the synthesis sections on regeneration.

When you invoke `zk-project-journaler` through Codex, Claude Code, or Gemini, the agent is expected to go one step further than the raw script output:

- refresh the digest
- fill `Manual synthesis`
- fill `Candidate zettels`
- fill `Open loops`

If you run the script directly from the shell, it only produces the factual digest and note template.

## Editor Integration

Neovim and VS Code snippets live under `tools/editors/`. They invoke `zk` from `PATH` and pass the detected workspace root to the language server.

For classic Vim:

```vim
source /path/to/zk/tools/vim/zettelkasten.vim
let g:zettelkasten_zk_cmd = 'zk'
" Optional override when root detection is not enough:
" let g:zettelkasten_root = '/path/to/zettelkasten-data'
```

## Runtime State

- `.zk/index.db` inside the data root is a derived, rebuildable index.
- Local SRS state lives under `XDG_STATE_HOME/zk/...` or `~/.local/state/zk/...`.
- Bibliography entries live in `bibliography.json` in the data repo so they move with the notes.

## Companion Data Repository

The companion data repo contains `zettels/`, `projects/`, `kanban/`, `archive/`, `bibliography.json`, and optional repo-local shell helpers such as `bin/sync`. `zk` reads and writes `.zk/index.db` inside that data root and now provides the lint and graph-generation commands directly.
