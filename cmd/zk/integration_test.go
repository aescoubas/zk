package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary
	tmpDir, err := os.MkdirTemp("", "zk_build")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath = filepath.Join(tmpDir, "zk")

	// Build the binary using the current directory (cmd/zk)
	cmd := exec.Command("go", "build", "-tags", "fts5", "-o", binaryPath, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Fallback: try building absolute path if relative fails for some reason
		// or panic
		panic("failed to build zk binary: " + err.Error())
	}

	code := m.Run()
	os.Exit(code)
}

func TestWorkflow(t *testing.T) {
	// 1. Setup Zettelkasten Root
	zkRoot, err := os.MkdirTemp("", "zk_test_root")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(zkRoot)

	if err := os.Mkdir(filepath.Join(zkRoot, "zettels"), 0755); err != nil {
		t.Fatal(err)
	}

	// 2. Init / Index (First run creates .zk/index.db)
	_, err = runZK(t, zkRoot, "index")
	if err != nil {
		t.Fatalf("zk index failed: %v", err)
	}

	// Verify DB exists
	if _, err := os.Stat(filepath.Join(zkRoot, ".zk", "index.db")); os.IsNotExist(err) {
		t.Error("index.db not created")
	}

	// 3. Create Note (zk new)
	// Mock EDITOR to just print the filename
	env := os.Environ()
	// We use 'cat' or similar that just prints content/args?
	// 'zk new' executes "$EDITOR <path>".
	// If EDITOR="echo", it executes "echo <path>".
	env = append(env, "EDITOR=echo")

	out, err := runZKEnv(t, zkRoot, env, "new", "My First Note")
	if err != nil {
		t.Fatalf("zk new failed: %v", err)
	}

	// Output should contain the path
	if !strings.Contains(out, ".md") {
		t.Errorf("Expected output to contain .md file path, got: %s", out)
	}

	// Find the created file
	files, _ := filepath.Glob(filepath.Join(zkRoot, "zettels", "*.md"))
	if len(files) != 1 {
		t.Fatalf("Expected 1 note in zettels, found %d", len(files))
	}

	// 4. Re-Index
	_, err = runZK(t, zkRoot, "index")
	if err != nil {
		t.Fatalf("zk index (2) failed: %v", err)
	}

	// 5. List
	out, err = runZK(t, zkRoot, "list")
	if err != nil {
		t.Fatalf("zk list failed: %v", err)
	}
	if !strings.Contains(out, "My First Note") {
		t.Errorf("List output missing note title. Got:\n%s", out)
	}
}

func TestMCP(t *testing.T) {
	// Setup
	zkRoot, err := os.MkdirTemp("", "zk_mcp_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(zkRoot)

	// Create DB
	if err := os.Mkdir(filepath.Join(zkRoot, "zettels"), 0755); err != nil {
		t.Fatal(err)
	}
	_, err = runZK(t, zkRoot, "index")
	if err != nil {
		t.Fatal(err)
	}

	// Start MCP process
	cmd := exec.Command(binaryPath, "mcp", "--dir", zkRoot)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// Send Initialize
	req := `{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "test", "version": "1.0"}}}`
	if _, err := stdin.Write([]byte(req + "\n")); err != nil {
		t.Fatal(err)
	}

	// Read Response
	scanner := bufio.NewScanner(stdout)
	if scanner.Scan() {
		resp := scanner.Text()
		if !strings.Contains(resp, "zk-mcp") {
			t.Errorf("Expected response to contain zk-mcp, got: %s", resp)
		}
	} else {
		t.Error("No response from MCP server")
	}
}

func TestLintReportsDeadLinksAndOrphans(t *testing.T) {
	zkRoot, err := os.MkdirTemp("", "zk_lint_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(zkRoot)

	zettelsDir := filepath.Join(zkRoot, "zettels")
	if err := os.Mkdir(zettelsDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"alpha.md": "# Alpha\n\nLinks to [[beta]] and [[missing]].\n",
		"beta.md":  "# Beta\n\nBacklink target.\n",
		"gamma.md": "# Gamma\n\nNobody links to me.\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(zettelsDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	out, err := runZK(t, zkRoot, "lint")
	if err == nil {
		t.Fatalf("expected zk lint to fail on dead links, output:\n%s", out)
	}
	if !strings.Contains(out, "--- Dead Links") {
		t.Fatalf("expected dead links section, got:\n%s", out)
	}
	if !strings.Contains(out, "alpha.md") || !strings.Contains(out, "[[missing]]") {
		t.Fatalf("expected missing-link details, got:\n%s", out)
	}
	if !strings.Contains(out, "--- Orphans") || !strings.Contains(out, "gamma") {
		t.Fatalf("expected orphan report, got:\n%s", out)
	}
}

func TestGraphWritesStandaloneHTML(t *testing.T) {
	zkRoot, err := os.MkdirTemp("", "zk_graph_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(zkRoot)

	zettelsDir := filepath.Join(zkRoot, "zettels")
	if err := os.Mkdir(zettelsDir, 0755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"alpha.md": "---\ntitle: Alpha Title\n---\n\n# Alpha\n\nSee [[beta]].\n",
		"beta.md":  "# Beta\n\nLinked note.\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(zettelsDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	outputPath := filepath.Join(zkRoot, "custom-graph.html")
	out, err := runZK(t, zkRoot, "graph", "--output", outputPath)
	if err != nil {
		t.Fatalf("zk graph failed: %v\n%s", err, out)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	if !strings.Contains(html, "Alpha Title") || !strings.Contains(html, "\"target\": \"beta\"") {
		t.Fatalf("expected graph HTML to contain note data, got:\n%s", html)
	}
}

func TestIndexAutomaticallyRebuildsLegacyIndex(t *testing.T) {
	zkRoot, err := os.MkdirTemp("", "zk_rebuild_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(zkRoot)

	zettelsDir := filepath.Join(zkRoot, "zettels")
	if err := os.Mkdir(zettelsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zettelsDir, "alpha.md"), []byte("# Alpha\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(filepath.Join(zkRoot, ".zk"), 0755); err != nil {
		t.Fatal(err)
	}
	out, err := runZK(t, zkRoot, "index")
	if err != nil {
		t.Fatalf("initial zk index failed: %v\n%s", err, out)
	}
	if err := os.Remove(filepath.Join(zkRoot, ".zk", "index.db")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zkRoot, ".zk", "index.db"), []byte("not-a-sqlite-db"), 0644); err != nil {
		t.Fatal(err)
	}

	out, err = runZK(t, zkRoot, "index")
	if err != nil {
		t.Fatalf("zk index rebuild failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Rebuilding incompatible index") {
		t.Fatalf("expected rebuild message, got:\n%s", out)
	}

	out, err = runZK(t, zkRoot, "list")
	if err != nil {
		t.Fatalf("zk list failed after rebuild: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Alpha") {
		t.Fatalf("expected list output to include rebuilt note, got:\n%s", out)
	}
}

func TestListReportsClearMessageForIncompatibleIndex(t *testing.T) {
	zkRoot, err := os.MkdirTemp("", "zk_incompatible_index")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(zkRoot)

	zettelsDir := filepath.Join(zkRoot, "zettels")
	if err := os.Mkdir(zettelsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(zkRoot, ".zk"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zkRoot, ".zk", "index.db"), []byte("not-a-sqlite-db"), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := runZK(t, zkRoot, "list")
	if err == nil {
		t.Fatalf("expected zk list to fail on incompatible index, output:\n%s", out)
	}
	if !strings.Contains(out, "Run 'zk index' to rebuild") {
		t.Fatalf("expected clear rebuild guidance, got:\n%s", out)
	}
}

func TestHelpListsTUIAndOmitsRemovedCommands(t *testing.T) {
	zkRoot, err := os.MkdirTemp("", "zk_help_surface")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(zkRoot)

	if err := os.Mkdir(filepath.Join(zkRoot, "zettels"), 0755); err != nil {
		t.Fatal(err)
	}

	out, err := runZK(t, zkRoot, "--help")
	if err != nil {
		t.Fatalf("zk --help failed: %v\n%s", err, out)
	}

	if !helpListsCommand(out, "tui") {
		t.Fatalf("expected help to list zk tui, got:\n%s", out)
	}
	for _, removed := range []string{"dashboard", "nav", "dump", "explore"} {
		if helpListsCommand(out, removed) {
			t.Fatalf("expected help to omit removed command %q, got:\n%s", removed, out)
		}
	}
}

func TestRootWithoutSubcommandShowsHelp(t *testing.T) {
	zkRoot, err := os.MkdirTemp("", "zk_root_help")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(zkRoot)

	if err := os.Mkdir(filepath.Join(zkRoot, "zettels"), 0755); err != nil {
		t.Fatal(err)
	}

	out, err := runZK(t, zkRoot)
	if err != nil {
		t.Fatalf("zk without a subcommand failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Usage:") || !helpListsCommand(out, "tui") {
		t.Fatalf("expected root command to show help mentioning tui, got:\n%s", out)
	}
}

func TestRemovedCommandsReturnUnknownCommand(t *testing.T) {
	zkRoot, err := os.MkdirTemp("", "zk_removed_commands")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(zkRoot)

	if err := os.Mkdir(filepath.Join(zkRoot, "zettels"), 0755); err != nil {
		t.Fatal(err)
	}

	for _, removed := range []string{"dashboard", "nav", "dump", "explore"} {
		out, err := runZK(t, zkRoot, removed)
		if err == nil {
			t.Fatalf("expected %q to fail after removal, output:\n%s", removed, out)
		}
		if !strings.Contains(out, "unknown command") {
			t.Fatalf("expected unknown command error for %q, got:\n%s", removed, out)
		}
	}
}

func runZK(t *testing.T, dir string, args ...string) (string, error) {
	return runZKEnv(t, dir, os.Environ(), args...)
}

func runZKEnv(t *testing.T, dir string, env []string, args ...string) (string, error) {
	finalArgs := append([]string{"--dir", dir}, args...)
	cmd := exec.Command(binaryPath, finalArgs...)
	cmd.Env = ensureXDGStateHome(env, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Don't fail immediately, let caller handle or inspect
		return string(out), err
	}
	return string(out), nil
}

func ensureXDGStateHome(env []string, dir string) []string {
	for _, kv := range env {
		if strings.HasPrefix(kv, "XDG_STATE_HOME=") {
			return env
		}
	}
	return append(env, "XDG_STATE_HOME="+filepath.Join(dir, ".test-state"))
}

func helpListsCommand(helpOutput string, name string) bool {
	prefix := "  " + name
	for _, line := range strings.Split(helpOutput, "\n") {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}
