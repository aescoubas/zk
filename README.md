# Zettelkasten

## Principles

> "From the moment I understood the weakness of my flesh, it disgusted me. I craved the strength and certainty of steel. I aspired to the purity of the Blessed Machine."

- **Blissful Amnesia**: Minimize the information you have to keep in "cache" by externalizing your thought processes. Once you are confident the data is stored and retrievable, you are free to forget.
- **Joyful Exploration**: The organization should mirror your mental processes and facilitate the exploration of your own mind.

## Guidelines

### What a Zettelkasten Is
- **Atomic**: One Zettel (note) should contain one atomic idea.
- **Connected**: Each idea should be embedded in a latticework of references (`[[WikiLinks]]`).
- **Personal**: It is not meant to be used by someone else.
- **Flat**: We avoid deep directory hierarchies in favor of connections.
- **Concise**: One Zettel should ideally span one screen size (no scrolling).

### Workflow
1.  **Read/Consume**: Note passages or thoughts in `references/` or temporary notes.
2.  **Synthesize**: Write **permanent atomic notes** (`permanent_notes/`) and link them to the source and other concepts.
3.  **Refine**: Periodically check for "orphan" notes and connect them.

---

## System Manual

This repository has been overhauled to support this atomic workflow with custom tooling.

### 1. Directory Structure

- **`permanent_notes/`**: All active atomic notes live here (flat structure).
- **`archive/`**: Legacy wikis and documents (`legacy_doc/`).
- **`references/`**: Source materials (`bibliographic_notes/`).
- **`bin/`**: Custom CLI tools (`zk`, `lint`, `sync`, `build_graph.py`).
- **`tools/`**: Helper scripts and templates.

### 2. The `zk` CLI Tool

The `bin/zk` script is your primary interface. Add it to your PATH or alias it.

- **Create a new note**:
  ```bash
  ./bin/zk new "My New Idea"
  ```
  Creates a file `permanent_notes/YYYYMMDDHHMM-my-new-idea.md` and opens it.

- **Search and Link**:
  ```bash
  ./bin/zk link
  ```
  Fuzzy searches notes and outputs a `[[WikiLink]]`. Useful for inserting links in your editor.

- **Random Note**:
  ```bash
  ./bin/zk random
  ```
  Opens a random note to spark rediscovery.

- **Stale Notes**:
  ```bash
  ./bin/zk stale
  ```
  Lists notes untouched for >1 year.

### 3. Vim Integration

Source `tools/vim/zettelkasten.vim` in your `.vimrc`:
```vim
source /path/to/repo/tools/vim/zettelkasten.vim
```

- **`<Leader>zn`**: Create a new note.
- **`<Ctrl>l`** (Insert Mode): Fuzzy search and insert a link.

### 4. Visualization & Quality

- **Graph View**:
  Run `./bin/build_graph.py` to generate `graph.html`. Open `graph.html` in your browser to see the network.
  
- **Linter**:
  Run `./bin/lint` to find:
  - Broken links (dead targets).
  - Orphan notes (zero incoming links).
  
- **Sync**:
  Run `./bin/sync` to pull, commit, and push.

- **Pre-commit Hook**:
  Run `./bin/install_hooks` to prevent committing if the linter fails (optional).
