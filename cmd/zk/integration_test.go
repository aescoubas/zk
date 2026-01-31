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

	// Create permanent_notes directory
	if err := os.Mkdir(filepath.Join(zkRoot, "permanent_notes"), 0755); err != nil {
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
	files, _ := filepath.Glob(filepath.Join(zkRoot, "permanent_notes", "*.md"))
	if len(files) != 1 {
		t.Fatalf("Expected 1 note in permanent_notes, found %d", len(files))
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
	if err := os.Mkdir(filepath.Join(zkRoot, "permanent_notes"), 0755); err != nil {
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

func runZK(t *testing.T, dir string, args ...string) (string, error) {
	return runZKEnv(t, dir, os.Environ(), args...)
}

func runZKEnv(t *testing.T, dir string, env []string, args ...string) (string, error) {
	// append --dir flag
	finalArgs := append(args, "--dir", dir)
	cmd := exec.Command(binaryPath, finalArgs...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Don't fail immediately, let caller handle or inspect
		return string(out), err
	}
	return string(out), nil
}
