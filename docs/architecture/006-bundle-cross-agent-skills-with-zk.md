# ADR 006: Bundle Cross-Agent Skills With zk

## Context

`zk` now lives separately from the markdown data repository, but part of the actual workflow also lives outside the Go binary. In particular, daily journaling based on Codex, Claude Code, and Gemini activity is better implemented as a skill than as a first-class CLI feature.

Without a repo-tracked distribution path, those skills drift:

- the workflow definition is not versioned with `zk`
- agent-specific copies diverge
- installation becomes manual and easy to miss

## Decision

Bundle supported agent skills inside the `zk` repository under `skills/` and deploy them from `install.sh` into user-local skill directories for:

- Codex: `~/.codex/skills/`
- Claude Code: `~/.claude/skills/`
- Gemini: `~/.gemini/skills/`

The first bundled skill is `zk-project-journaler`, which condenses coding-agent activity into one daily zettel by parsing local session artifacts and live git state.

## Consequences

### Positive

- Workflow automation is versioned and testable inside the `zk` repo.
- Codex, Claude Code, and Gemini receive the same skill content on install.
- The Go CLI stays focused on note/index operations while higher-level workflows remain scriptable.

### Negative

- `install.sh` now manages user-local agent skill directories in addition to the binary.
- Bundled skills add a small amount of non-Go surface area that must be documented and tested.
