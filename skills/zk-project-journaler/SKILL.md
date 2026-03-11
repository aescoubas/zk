---
name: zk-project-journaler
description: Use when the user wants to condense Codex, Claude Code, or Gemini coding activity into a single daily zettel, generate a daily agent activity note, or reconcile AI-assisted project work across repositories.
---

# zk Project Journaler

Use this skill to turn coding-agent activity into one standard daily zettel in the zettelkasten.

When this skill is invoked by an agent, the agent must not stop after generating the raw note. It must also populate the synthesis sections in the note.

## What This Skill Does

- Reads local session artifacts from Codex, Claude Code, and Gemini.
- Groups activity by repository and project.
- Cross-checks the touched repositories with live git status and diff data.
- Creates or refreshes a daily zettel while preserving existing synthesis sections until the invoking agent updates them.

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
4. Read the generated note and replace the placeholder synthesis sections yourself.
   - `Manual synthesis`: write 1-3 short paragraphs summarizing what was shipped, investigated, and decided.
   - `Candidate zettels`: add concrete checkbox items for durable ideas worth extracting into standalone notes.
   - `Open loops`: add concrete checkbox items for blockers, unresolved questions, and next actions.
5. If those sections already contain real content, update them in place instead of blindly restoring placeholders.
6. Promote durable ideas into separate permanent notes with `zk new` instead of leaving them buried in the daily activity log.

## Required Completion Behavior

When the user asks to summarize the day, create the daily zettel, or invokes `zk-project-journaler`, the task is not complete until all of the following are true:

- the daily note exists in `zettels/`
- the generated digest is up to date for the requested date
- `Manual synthesis` contains real prose, not the default placeholder
- `Candidate zettels` contains concrete candidate notes or an explicit statement that none emerged
- `Open loops` contains concrete follow-ups or an explicit statement that no open loops remain

Do not leave placeholder text such as:

- `Write your own synthesis, decisions, and follow-up zettels here.`
- `Extract at least one durable idea if today produced one.`
- `Record unresolved issues, blockers, or next steps here.`

If the generated digest is noisy, clean it up in the synthesis rather than copying the noise verbatim.

## Commands

Preview the note without writing it:

```bash
python3 ~/.codex/skills/zk-project-journaler/scripts/render_daily_zettel.py --date 2026-03-11 --zk-root /path/to/zettelkasten-data
```

Create or refresh the daily zettel:

```bash
python3 ~/.codex/skills/zk-project-journaler/scripts/render_daily_zettel.py --date 2026-03-11 --zk-root /path/to/zettelkasten-data --write-note
```

After running the script, open the note and fill the synthesis sections before answering the user.

The script also accepts `--home` to inspect a different user profile during debugging or testing.

## Guardrails

- Do not treat raw agent transcripts as permanent knowledge.
- Keep one activity zettel per day.
- Preserve real synthesis content on regeneration, but update it when the user explicitly asks for a refreshed daily summary.
- If no agent activity exists for the requested date, report that clearly instead of creating an empty note.
