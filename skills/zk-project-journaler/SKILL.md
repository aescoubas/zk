---
name: zk-project-journaler
description: Use when the user wants to condense Codex, Claude Code, or Gemini coding activity into a single daily zettel, generate a daily agent activity note, or reconcile AI-assisted project work across repositories.
---

# zk Project Journaler

Use this skill to turn coding-agent activity into one standard daily zettel in the zettelkasten.

## What This Skill Does

- Reads local session artifacts from Codex, Claude Code, and Gemini.
- Groups activity by repository and project.
- Cross-checks the touched repositories with live git status and diff data.
- Creates or refreshes a daily zettel while preserving manual synthesis sections.

## Default Log Sources

- Codex: `~/.codex/sessions/YYYY/MM/DD/*.jsonl`
- Claude Code: `~/.claude/projects/**/*.jsonl`
- Gemini: `~/.gemini/tmp/*/chats/session-YYYY-MM-DDT*.json`

Claude subagent logs are skipped by default to reduce noise.

## Workflow

1. Resolve the zettelkasten root.
   - Prefer `--zk-root` when the user gives one.
   - Otherwise use `ZK_PATH`.
   - Otherwise use `~/.config/zk/root`.
2. Run the bundled script for the requested date.
   - Codex install path: `~/.codex/skills/zk-project-journaler/scripts/render_daily_zettel.py`
   - Claude install path: `~/.claude/skills/zk-project-journaler/scripts/render_daily_zettel.py`
   - Gemini install path: `~/.gemini/skills/zk-project-journaler/scripts/render_daily_zettel.py`
3. Use `--write-note` when the user wants the zettel created or refreshed in `zettels/`.
4. Review the generated note and keep the manual sections human-written.
5. Promote durable ideas into separate permanent notes with `zk new` instead of leaving them buried in the daily activity log.

## Commands

Preview the note without writing it:

```bash
python3 ~/.codex/skills/zk-project-journaler/scripts/render_daily_zettel.py --date 2026-03-11 --zk-root /path/to/zettelkasten-data
```

Create or refresh the daily zettel:

```bash
python3 ~/.codex/skills/zk-project-journaler/scripts/render_daily_zettel.py --date 2026-03-11 --zk-root /path/to/zettelkasten-data --write-note
```

The script also accepts `--home` to inspect a different user profile during debugging or testing.

## Guardrails

- Do not treat raw agent transcripts as permanent knowledge.
- Keep one activity zettel per day.
- Preserve the manual synthesis section on regeneration.
- If no agent activity exists for the requested date, report that clearly instead of creating an empty note.
