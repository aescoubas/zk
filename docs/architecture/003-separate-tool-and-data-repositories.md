# 003. Separate Tool and Data Repositories

**Date:** 2026-03-09

## Context

The original repository stored both the Markdown knowledge base and the `zk` tool source in a single git history. That mixed high-volume personal note changes with tool development and made installation, editor integration, and testing depend on a specific checkout layout.

## Decision

We split the system into two repositories:

- `zk`: the Go codebase, install script, editor integrations, and ADRs
- `zettelkasten-data`: the Markdown notes, project files, archive, aphorisms, and repo-local maintenance scripts

The tool now treats the data root as an explicit runtime concern. It resolves that root in this order:

1. `--dir`
2. `ZK_PATH`
3. `~/.config/zk/root`
4. the current working directory

The installer may write `~/.config/zk/root` when `ZK_DATA_DIR` is provided.

## Consequences

- Tool history is easier to review and publish independently from personal note churn.
- The same `zk` checkout can operate on any compatible data repository.
- Editor integrations must either detect the data root or pass it explicitly.
- End-to-end manual testing now requires both repositories.
