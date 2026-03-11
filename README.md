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

Install the binary and shell completions into `~/.local` by default:

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
