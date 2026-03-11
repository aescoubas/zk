# 004. Move Maintenance Commands Into `zk`

**Date:** 2026-03-10

## Context

After splitting the original repository into `zk` and `zettelkasten-data`, the data repository still carried Python maintenance scripts for linting and static graph generation. That left system-wide installs dependent on scripts and templates outside the Go codebase, and it kept the most useful maintenance operations coupled to one specific checkout layout.

## Decision

We moved the active maintenance commands into the Go binary:

- `zk lint` replaces the dead-link and orphan-note checks that previously lived in the data repository.
- `zk graph` replaces the static HTML graph generator and embeds its HTML template directly into the binary.
- `install.sh` accepts an explicit data-root configuration (`--data-dir` or `ZK_DATA_DIR`) and persists it to `~/.config/zk/root`.

The companion data repository now focuses on Markdown content plus optional repo-local shell helpers, not Python tooling.

## Consequences

- A system-wide `zk` install can lint and generate graphs without depending on Python scripts in the data repo.
- The active maintenance workflow is now versioned, tested, and distributed with the main toolchain.
- The data repository is simpler and closer to a pure content repository.
- Legacy one-off migration helpers are removed from the default workflow rather than ported preemptively.
