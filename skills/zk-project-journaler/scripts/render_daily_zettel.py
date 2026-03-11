#!/usr/bin/env python3

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from dataclasses import dataclass
from datetime import date, datetime
from pathlib import Path
from typing import Iterable


START_MARKER = "<!-- zk-project-journaler:start -->"
END_MARKER = "<!-- zk-project-journaler:end -->"


@dataclass
class SessionActivity:
    source: str
    session_id: str
    repo_root: str
    project: str
    start: datetime | None
    end: datetime | None
    prompt: str
    outcome: str
    model: str
    source_file: str


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Render a daily zettel from Codex, Claude Code, and Gemini session logs."
    )
    parser.add_argument(
        "--date",
        default=date.today().isoformat(),
        help="Target local date in YYYY-MM-DD format (default: today).",
    )
    parser.add_argument(
        "--home",
        default=os.path.expanduser("~"),
        help="Home directory containing agent logs (default: current HOME).",
    )
    parser.add_argument(
        "--zk-root",
        default="",
        help="Zettelkasten data root. Falls back to ZK_PATH or ~/.config/zk/root when omitted.",
    )
    parser.add_argument(
        "--write-note",
        action="store_true",
        help="Write or refresh the daily zettel in <zk-root>/zettels/ instead of printing it.",
    )
    return parser.parse_args()


def normalize_space(text: str) -> str:
    return " ".join(text.split())


def truncate(text: str, limit: int = 180) -> str:
    text = normalize_space(text)
    if len(text) <= limit:
        return text
    return text[: limit - 3].rstrip() + "..."


def slugify(text: str) -> str:
    slug = re.sub(r"[^a-z0-9]+", "-", text.lower()).strip("-")
    return slug or "unknown"


def yaml_quote(text: str) -> str:
    return '"' + text.replace("\\", "\\\\").replace('"', '\\"') + '"'


def parse_timestamp(raw: str | None, local_tz) -> datetime | None:
    if not raw:
        return None
    try:
        parsed = datetime.fromisoformat(raw.replace("Z", "+00:00"))
    except ValueError:
        return None
    if parsed.tzinfo is None:
        return parsed.replace(tzinfo=local_tz)
    return parsed.astimezone(local_tz)


def extract_text(value) -> str:
    if value is None:
        return ""
    if isinstance(value, str):
        return normalize_space(value)
    if isinstance(value, list):
        parts = [extract_text(item) for item in value]
        return normalize_space(" ".join(part for part in parts if part))
    if isinstance(value, dict):
        if value.get("type") == "tool_result":
            return ""
        if "tool_use_id" in value and "text" not in value:
            return ""
        if isinstance(value.get("text"), str):
            return normalize_space(value["text"])
        if "content" in value:
            return extract_text(value["content"])
        if "message" in value:
            return extract_text(value["message"])
        parts = []
        for key, item in value.items():
            if key in {"tool_use_id", "is_error"}:
                continue
            part = extract_text(item)
            if part:
                parts.append(part)
        return normalize_space(" ".join(parts))
    return ""


def find_repo_root(path_str: str) -> str:
    if not path_str:
        return ""
    current = Path(path_str).expanduser()
    if not current.exists():
        return str(current)
    current = current.resolve()
    for candidate in [current, *current.parents]:
        if (candidate / ".git").exists():
            return str(candidate)
    return str(current)


def detect_project_name(repo_root: str) -> str:
    if not repo_root:
        return "unknown"
    return Path(repo_root).name or "unknown"


def iter_codex_files(home: Path, target_date: date) -> Iterable[Path]:
    day_dir = home / ".codex" / "sessions" / target_date.strftime("%Y") / target_date.strftime("%m") / target_date.strftime("%d")
    if not day_dir.is_dir():
        return []
    return sorted(day_dir.glob("*.jsonl"))


def iter_claude_files(home: Path) -> Iterable[Path]:
    projects_dir = home / ".claude" / "projects"
    if not projects_dir.is_dir():
        return []
    paths = []
    for path in projects_dir.rglob("*.jsonl"):
        path_str = str(path)
        if "/subagents/" in path_str or path.name == "memory":
            continue
        paths.append(path)
    return sorted(paths)


def iter_gemini_files(home: Path, target_date: date) -> Iterable[Path]:
    tmp_dir = home / ".gemini" / "tmp"
    if not tmp_dir.is_dir():
        return []
    prefix = f"session-{target_date.isoformat()}T"
    matches = []
    for path in tmp_dir.rglob("session-*.json"):
        if "/chats/" not in str(path):
            continue
        if path.name.startswith(prefix):
            matches.append(path)
    return sorted(matches)


def parse_codex_file(path: Path, target_date: date, local_tz) -> SessionActivity | None:
    session_id = path.stem
    cwd = ""
    first_user = ""
    last_assistant = ""
    final_assistant = ""
    model = ""
    start = None
    end = None
    saw_target_day = False

    with path.open(encoding="utf-8") as handle:
        for raw_line in handle:
            raw_line = raw_line.strip()
            if not raw_line:
                continue
            try:
                record = json.loads(raw_line)
            except json.JSONDecodeError:
                continue

            payload = record.get("payload", {})
            timestamp = parse_timestamp(record.get("timestamp") or payload.get("timestamp"), local_tz)
            if timestamp and timestamp.date() == target_date:
                saw_target_day = True
                start = min(start, timestamp) if start else timestamp
                end = max(end, timestamp) if end else timestamp

            if record.get("type") == "session_meta":
                session_id = payload.get("id", session_id)
                cwd = payload.get("cwd", cwd)
                model = payload.get("model_provider", model)
                continue

            if record.get("type") != "response_item":
                continue
            if payload.get("type") != "message":
                continue

            role = payload.get("role")
            text = extract_text(payload.get("content"))
            if role == "user" and text and not first_user:
                first_user = text
            if role == "assistant" and text:
                last_assistant = text
                if payload.get("phase") == "final_answer":
                    final_assistant = text

    if not saw_target_day:
        return None

    repo_root = find_repo_root(cwd)
    return SessionActivity(
        source="codex",
        session_id=session_id,
        repo_root=repo_root,
        project=detect_project_name(repo_root),
        start=start,
        end=end,
        prompt=first_user,
        outcome=final_assistant or last_assistant,
        model=model,
        source_file=str(path),
    )


def parse_claude_file(path: Path, target_date: date, local_tz) -> SessionActivity | None:
    session_id = path.stem
    cwd = ""
    first_user = ""
    last_assistant = ""
    model = ""
    start = None
    end = None
    saw_target_day = False

    with path.open(encoding="utf-8") as handle:
        for raw_line in handle:
            raw_line = raw_line.strip()
            if not raw_line:
                continue
            try:
                record = json.loads(raw_line)
            except json.JSONDecodeError:
                continue

            timestamp = parse_timestamp(record.get("timestamp"), local_tz)
            if not timestamp or timestamp.date() != target_date:
                continue

            saw_target_day = True
            start = min(start, timestamp) if start else timestamp
            end = max(end, timestamp) if end else timestamp
            session_id = record.get("sessionId", session_id)
            cwd = record.get("cwd", cwd)

            record_type = record.get("type")
            message = record.get("message", {})
            role = message.get("role")
            text = extract_text(message.get("content"))
            if record_type == "user" and role == "user" and text and not first_user:
                first_user = text
            if record_type == "assistant" and role == "assistant" and text:
                last_assistant = text
                model = message.get("model", model)

    if not saw_target_day:
        return None

    repo_root = find_repo_root(cwd)
    return SessionActivity(
        source="claude",
        session_id=session_id,
        repo_root=repo_root,
        project=detect_project_name(repo_root),
        start=start,
        end=end,
        prompt=first_user,
        outcome=last_assistant,
        model=model,
        source_file=str(path),
    )


def parse_gemini_file(path: Path, target_date: date, local_tz) -> SessionActivity | None:
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return None

    start = parse_timestamp(payload.get("startTime"), local_tz)
    end = parse_timestamp(payload.get("lastUpdated"), local_tz)
    if not start and not end:
        return None

    if start and start.date() != target_date and end and end.date() != target_date:
        return None

    first_user = ""
    last_assistant = ""
    model = ""
    messages = payload.get("messages", [])
    for message in messages:
        timestamp = parse_timestamp(message.get("timestamp"), local_tz)
        if timestamp and timestamp.date() != target_date:
            continue
        if message.get("type") == "user":
            if not first_user:
                first_user = extract_text(message.get("content"))
            continue
        text = extract_text(message.get("content"))
        if text:
            last_assistant = text
            model = message.get("model", model)

    project_dir = path.parent.parent
    project_root_file = project_dir / ".project_root"
    cwd = ""
    if project_root_file.is_file():
        cwd = project_root_file.read_text(encoding="utf-8").strip()

    repo_root = find_repo_root(cwd or str(project_dir))
    return SessionActivity(
        source="gemini",
        session_id=payload.get("sessionId", path.stem),
        repo_root=repo_root,
        project=detect_project_name(repo_root),
        start=start,
        end=end,
        prompt=first_user,
        outcome=last_assistant,
        model=model,
        source_file=str(path),
    )


def collect_sessions(home: Path, target_date: date) -> list[SessionActivity]:
    local_tz = datetime.now().astimezone().tzinfo
    sessions = []
    for path in iter_codex_files(home, target_date):
        session = parse_codex_file(path, target_date, local_tz)
        if session and (session.prompt or session.outcome):
            sessions.append(session)
    for path in iter_claude_files(home):
        session = parse_claude_file(path, target_date, local_tz)
        if session and (session.prompt or session.outcome):
            sessions.append(session)
    for path in iter_gemini_files(home, target_date):
        session = parse_gemini_file(path, target_date, local_tz)
        if session and (session.prompt or session.outcome):
            sessions.append(session)
    sessions.sort(key=lambda item: ((item.start or item.end or datetime.min), item.source, item.session_id))
    return sessions


def run_git(repo_root: str, *args: str) -> str:
    if not repo_root:
        return ""
    try:
        completed = subprocess.run(
            ["git", "-C", repo_root, *args],
            check=True,
            capture_output=True,
            text=True,
        )
    except (subprocess.CalledProcessError, FileNotFoundError):
        return ""
    return completed.stdout.strip()


def git_snapshot(repo_root: str, target_date: date) -> dict[str, str]:
    if not repo_root:
        return {}
    return {
        "branch": run_git(repo_root, "rev-parse", "--abbrev-ref", "HEAD"),
        "status": run_git(repo_root, "status", "--short"),
        "diff_stat": run_git(repo_root, "diff", "--stat", "--compact-summary"),
        "commits": run_git(
            repo_root,
            "log",
            f"--since={target_date.isoformat()} 00:00:00",
            f"--until={target_date.isoformat()} 23:59:59",
            "--format=%h %s",
            "--max-count=10",
        ),
    }


def render_frontmatter(target_date: date, sessions: list[SessionActivity]) -> str:
    agents = sorted({session.source for session in sessions})
    projects = sorted({session.project for session in sessions if session.project})
    repos = sorted({session.repo_root for session in sessions if session.repo_root})
    source_sessions = [f"{session.source}:{session.session_id}" for session in sessions]
    tags = ["agent-log", "daily", "coding"]
    tags.extend(f"project-{slugify(project)}" for project in projects[:8])

    lines = [
        "---",
        f"title: Agent activity {target_date.isoformat()}",
        f"date: {target_date.isoformat()}",
        "type: agent_activity_log",
        "tags:",
    ]
    for tag in tags:
        lines.append(f"  - {tag}")
    lines.append("agents:")
    for agent in agents:
        lines.append(f"  - {agent}")
    lines.append("projects:")
    for project in projects:
        lines.append(f"  - {yaml_quote(project)}")
    lines.append("repos:")
    for repo in repos:
        lines.append(f"  - {yaml_quote(repo)}")
    lines.append("source_sessions:")
    for source_session in source_sessions:
        lines.append(f"  - {yaml_quote(source_session)}")
    lines.append("generated_by: zk-project-journaler")
    lines.append("---")
    return "\n".join(lines)


def render_block(lines: list[str]) -> str:
    if not lines:
        return "```text\n(none)\n```"
    return "```text\n" + "\n".join(lines) + "\n```"


def format_time_range(session: SessionActivity) -> str:
    if session.start and session.end:
        return f"{session.start.strftime('%H:%M')}-{session.end.strftime('%H:%M')}"
    if session.start:
        return session.start.strftime("%H:%M")
    if session.end:
        return session.end.strftime("%H:%M")
    return "unknown-time"


def render_generated_digest(target_date: date, sessions: list[SessionActivity]) -> str:
    repos = sorted({session.repo_root for session in sessions if session.repo_root})
    projects = sorted({session.project for session in sessions if session.project})
    agents = sorted({session.source for session in sessions})

    lines = [
        "## Agent Session Digest",
        "",
        f"- Date: `{target_date.isoformat()}`",
        f"- Sessions captured: `{len(sessions)}`",
        f"- Agents seen: `{', '.join(agents)}`",
        f"- Projects touched: `{', '.join(projects)}`",
        "",
    ]

    by_repo: dict[str, list[SessionActivity]] = {}
    for session in sessions:
        by_repo.setdefault(session.repo_root, []).append(session)

    for repo_root in repos:
        repo_sessions = by_repo[repo_root]
        snapshot = git_snapshot(repo_root, target_date)
        lines.extend(
            [
                f"### {detect_project_name(repo_root)}",
                "",
                f"- Repo: `{repo_root}`",
                f"- Agents: `{', '.join(sorted({session.source for session in repo_sessions}))}`",
            ]
        )
        if snapshot.get("branch"):
            lines.append(f"- Branch: `{snapshot['branch']}`")
        lines.extend(["", "Sessions:", ""])
        for session in repo_sessions:
            lines.append(
                f"- `{format_time_range(session)}` `{session.source}` `{session.session_id}`"
            )
            if session.prompt:
                lines.append(f"  Task: {truncate(session.prompt)}")
            if session.outcome:
                lines.append(f"  Outcome: {truncate(session.outcome)}")
            if session.model:
                lines.append(f"  Model: `{session.model}`")
            lines.append(f"  Source: `{session.source_file}`")
        lines.extend(["", "Git status:", render_block(snapshot.get("status", "").splitlines()), ""])
        lines.extend(["Diff stat:", render_block(snapshot.get("diff_stat", "").splitlines()), ""])
        lines.extend(["Commits on this date:", render_block(snapshot.get("commits", "").splitlines()), ""])

    lines.extend(
        [
            "## Source Sessions",
            "",
        ]
    )
    for session in sessions:
        lines.append(
            f"- `{session.source}:{session.session_id}` -> `{session.source_file}`"
        )

    return "\n".join(lines).rstrip()


def manual_template() -> str:
    return "\n".join(
        [
            "## Manual synthesis",
            "",
            "Write your own synthesis, decisions, and follow-up zettels here.",
            "",
            "## Candidate zettels",
            "",
            "- [ ] Extract at least one durable idea if today produced one.",
            "",
            "## Open loops",
            "",
            "- [ ] Record unresolved issues, blockers, or next steps here.",
        ]
    )


def build_note(target_date: date, sessions: list[SessionActivity], existing: str | None = None) -> str:
    frontmatter = render_frontmatter(target_date, sessions)
    heading = f"# Agent activity {target_date.isoformat()}"
    generated = render_generated_digest(target_date, sessions)
    generated_block = f"{START_MARKER}\n{generated}\n{END_MARKER}"
    prefix = f"{frontmatter}\n\n{heading}\n\n{generated_block}"

    if existing is None:
        return prefix + "\n\n" + manual_template() + "\n"

    if START_MARKER not in existing or END_MARKER not in existing:
        raise RuntimeError(
            "refusing to overwrite an existing note without zk-project-journaler markers"
        )

    suffix = existing.split(END_MARKER, 1)[1].lstrip("\n")
    if suffix:
        return prefix + "\n\n" + suffix.rstrip() + "\n"
    return prefix + "\n"


def resolve_zk_root(home: Path, explicit: str) -> Path:
    if explicit:
        return Path(explicit).expanduser()

    env_root = os.environ.get("ZK_PATH", "").strip()
    if env_root:
        return Path(env_root).expanduser()

    config_home = Path(os.environ.get("XDG_CONFIG_HOME", str(home / ".config")))
    config_file = config_home / "zk" / "root"
    if config_file.is_file():
        configured = config_file.read_text(encoding="utf-8").strip()
        if configured:
            return Path(configured).expanduser()

    raise RuntimeError("could not resolve zk root; pass --zk-root or configure ZK_PATH/~/.config/zk/root")


def write_note(zk_root: Path, target_date: date, content: str) -> Path:
    zettels_dir = zk_root / "zettels"
    if not zettels_dir.is_dir():
        raise RuntimeError(f"{zettels_dir} does not exist")
    note_path = zettels_dir / f"{target_date.strftime('%Y%m%d')}-agent-activity.md"
    note_path.write_text(content, encoding="utf-8")
    return note_path


def main() -> int:
    args = parse_args()
    try:
        target_date = date.fromisoformat(args.date)
    except ValueError:
        print(f"invalid date: {args.date}", file=sys.stderr)
        return 2

    home = Path(args.home).expanduser()
    sessions = collect_sessions(home, target_date)
    if not sessions:
        print(f"no coding-agent activity found for {target_date.isoformat()}", file=sys.stderr)
        return 1

    existing = None
    if args.write_note:
        zk_root = resolve_zk_root(home, args.zk_root)
        note_path = zk_root / "zettels" / f"{target_date.strftime('%Y%m%d')}-agent-activity.md"
        if note_path.exists():
            existing = note_path.read_text(encoding="utf-8")
        content = build_note(target_date, sessions, existing=existing)
        written = write_note(zk_root, target_date, content)
        print(str(written))
        return 0

    content = build_note(target_date, sessions)
    print(content)
    return 0


if __name__ == "__main__":
    sys.exit(main())
