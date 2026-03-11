# 005. Separate Durable State From The Derived Index

**Date:** 2026-03-11

## Context

After the tool and data repositories were split, `zk` still stored several kinds of information in `.zk/index.db` inside the data repository. Some of that data was derived and disposable, such as the parsed note graph and search index. Some of it was durable user state, such as SRS progress. Bibliography entries were also stored only in SQLite, which meant index rebuilds could destroy user-authored metadata.

The previous root-resolution fallback to "current working directory" also made it too easy to open the wrong repository and hit confusing database errors.

## Decision

We tightened the boundary between durable state and derived data:

- `.zk/index.db` remains a rebuildable derived index for parsed notes, links, tags, citations, embeddings, and optional FTS data.
- Local SRS state moves to a separate SQLite database under `XDG_STATE_HOME/zk/...` or `~/.local/state/zk/...`.
- Bibliography entries move to a portable `bibliography.json` file in the data repository.
- Index databases now carry explicit schema/version metadata and feature flags.
- `zk index` automatically rebuilds incompatible indexes.
- Root resolution only falls back to the current working directory when it clearly looks like a data repository.

## Consequences

- Normal indexing remains fast and disposable.
- SRS progress survives index rebuilds and stays local to the machine by design.
- Bibliography data now travels with the Markdown repository.
- Old indexes can be detected and rebuilt with a clear error path instead of surfacing raw SQLite compatibility failures.
- Commands launched from unrelated directories now fail early with explicit configuration guidance instead of silently targeting the wrong root.
