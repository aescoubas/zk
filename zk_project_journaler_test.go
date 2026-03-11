package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectJournalerSkillRequiresPopulatedSynthesisSections(t *testing.T) {
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	skillPath := filepath.Join(repoRoot, "skills", "zk-project-journaler", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}

	skill := string(data)
	for _, want := range []string{
		"the task is not complete until",
		"replace the placeholder synthesis sections yourself",
		"Manual synthesis",
		"Candidate zettels",
		"Open loops",
		"Do not leave placeholder text",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("expected skill to contain %q\n%s", want, skill)
		}
	}
}

func TestProjectJournalerRendersAndUpsertsDailyZettel(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	scriptPath := filepath.Join(repoRoot, "skills", "zk-project-journaler", "scripts", "render_daily_zettel.py")
	home := t.TempDir()
	zkRoot := filepath.Join(home, "zettelkasten-data")
	projectRoot := filepath.Join(home, "work", "demo")

	for _, dir := range []string{
		filepath.Join(zkRoot, "zettels"),
		filepath.Join(home, ".codex", "sessions", "2026", "03", "11"),
		filepath.Join(home, ".claude", "projects", "demo"),
		filepath.Join(home, ".gemini", "tmp", "demo", "chats"),
		projectRoot,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = projectRoot
		cmd.Env = append(os.Environ(), "HOME="+home)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	run("git", "init")
	run("git", "config", "user.name", "Test User")
	run("git", "config", "user.email", "test@example.com")

	readmePath := filepath.Join(projectRoot, "README.md")
	if err := os.WriteFile(readmePath, []byte("# demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", "README.md")
	run("git", "commit", "-m", "feat: initial commit")

	if err := os.WriteFile(filepath.Join(projectRoot, "app.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "router.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	codexLog := `{"timestamp":"2026-03-11T08:00:00.000Z","type":"session_meta","payload":{"id":"codex-session","timestamp":"2026-03-11T08:00:00.000Z","cwd":"` + projectRoot + `","originator":"codex_cli_rs","model_provider":"openai"}}
{"timestamp":"2026-03-11T08:01:00.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"Implement authentication middleware"}]}}
{"timestamp":"2026-03-11T08:15:00.000Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Added authentication middleware and tests."}],"phase":"final_answer"}}
`
	if err := os.WriteFile(filepath.Join(home, ".codex", "sessions", "2026", "03", "11", "rollout-2026-03-11T08-00-00-codex.jsonl"), []byte(codexLog), 0o644); err != nil {
		t.Fatal(err)
	}

	claudeLog := `{"parentUuid":null,"cwd":"` + projectRoot + `","sessionId":"claude-session","type":"user","timestamp":"2026-03-11T09:00:00.000Z","message":{"role":"user","content":"Refactor the router to use the auth middleware"}}
{"parentUuid":"1","cwd":"` + projectRoot + `","sessionId":"claude-session","type":"assistant","timestamp":"2026-03-11T09:18:00.000Z","message":{"role":"assistant","content":[{"type":"text","text":"Router refactor complete."}]}}
`
	if err := os.WriteFile(filepath.Join(home, ".claude", "projects", "demo", "claude-session.jsonl"), []byte(claudeLog), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(home, ".gemini", "tmp", "demo", ".project_root"), []byte(projectRoot+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	geminiLog := `{
  "sessionId": "gemini-session",
  "projectHash": "demo-hash",
  "startTime": "2026-03-11T10:00:00.000Z",
  "lastUpdated": "2026-03-11T10:12:00.000Z",
  "messages": [
    {
      "id": "1",
      "timestamp": "2026-03-11T10:00:00.000Z",
      "type": "user",
      "content": [
        {
          "text": "Review the current auth flow and identify gaps."
        }
      ]
    },
    {
      "id": "2",
      "timestamp": "2026-03-11T10:12:00.000Z",
      "type": "gemini",
      "content": "Identified missing route guards and weak session validation.",
      "model": "gemini-3.1-pro-preview"
    }
  ]
}
`
	if err := os.WriteFile(filepath.Join(home, ".gemini", "tmp", "demo", "chats", "session-2026-03-11T10-00-gemini.json"), []byte(geminiLog), 0o644); err != nil {
		t.Fatal(err)
	}

	runScript := func() string {
		t.Helper()
		cmd := exec.Command("python3", scriptPath, "--date", "2026-03-11", "--home", home, "--zk-root", zkRoot, "--write-note")
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(), "HOME="+home)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("journaler failed: %v\n%s", err, out)
		}
		return strings.TrimSpace(string(out))
	}

	firstOut := runScript()
	notePath := filepath.Join(zkRoot, "zettels", "20260311-agent-activity.md")
	if !strings.Contains(firstOut, notePath) {
		t.Fatalf("expected script output to mention %s, got %q", notePath, firstOut)
	}

	noteData, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatal(err)
	}
	note := string(noteData)
	for _, want := range []string{
		"title: Agent activity 2026-03-11",
		"type: agent_activity_log",
		"- agent-log",
		"## Agent Session Digest",
		"### demo",
		"Implement authentication middleware",
		"Router refactor complete.",
		"Review the current auth flow and identify gaps.",
		"codex:codex-session",
		"claude:claude-session",
		"gemini:gemini-session",
	} {
		if !strings.Contains(note, want) {
			t.Fatalf("expected note to contain %q\n%s", want, note)
		}
	}

	manualText := "Manual synthesis survives regeneration."
	updated := strings.Replace(note, "Write your own synthesis, decisions, and follow-up zettels here.", manualText, 1)
	if err := os.WriteFile(notePath, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}

	secondOut := runScript()
	if !strings.Contains(secondOut, notePath) {
		t.Fatalf("expected rerun output to mention %s, got %q", notePath, secondOut)
	}

	noteData, err = os.ReadFile(notePath)
	if err != nil {
		t.Fatal(err)
	}
	note = string(noteData)
	if !strings.Contains(note, manualText) {
		t.Fatalf("expected regenerated note to preserve manual text\n%s", note)
	}
}
