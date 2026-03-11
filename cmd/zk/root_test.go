package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRootDirFlagWins(t *testing.T) {
	flagDir := t.TempDir()
	envDir := t.TempDir()
	configDir := t.TempDir()
	for _, dir := range []string{flagDir, envDir} {
		if err := os.Mkdir(filepath.Join(dir, "zettels"), 0755); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("ZK_PATH", envDir)
	t.Setenv("XDG_CONFIG_HOME", configDir)

	if err := os.MkdirAll(filepath.Join(configDir, "zk"), 0755); err != nil {
		t.Fatal(err)
	}
	configRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(configRoot, "zettels"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "zk", "root"), []byte(configRoot+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveRootDir(flagDir)
	if err != nil {
		t.Fatal(err)
	}

	want, err := filepath.Abs(flagDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("expected flag dir %q, got %q", want, got)
	}
}

func TestResolveRootDirEnvWinsOverConfig(t *testing.T) {
	envDir := t.TempDir()
	configDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(envDir, "zettels"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ZK_PATH", envDir)
	t.Setenv("XDG_CONFIG_HOME", configDir)

	if err := os.MkdirAll(filepath.Join(configDir, "zk"), 0755); err != nil {
		t.Fatal(err)
	}
	configRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(configRoot, "zettels"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "zk", "root"), []byte(configRoot+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveRootDir("")
	if err != nil {
		t.Fatal(err)
	}

	want, err := filepath.Abs(envDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("expected env dir %q, got %q", want, got)
	}
}

func TestResolveRootDirUsesConfigFile(t *testing.T) {
	configHome := t.TempDir()
	configRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(configRoot, "zettels"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ZK_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", configHome)

	if err := os.MkdirAll(filepath.Join(configHome, "zk"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configHome, "zk", "root"), []byte(configRoot+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveRootDir("")
	if err != nil {
		t.Fatal(err)
	}

	want, err := filepath.Abs(configRoot)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("expected config dir %q, got %q", want, got)
	}
}

func TestResolveRootDirFallsBackToWorkingDirectory(t *testing.T) {
	cwd := t.TempDir()
	configHome := t.TempDir()

	t.Setenv("ZK_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", configHome)

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(previousWD); chdirErr != nil {
			t.Fatalf("restore working directory: %v", chdirErr)
		}
	})

	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(cwd, "zettels"), 0755); err != nil {
		t.Fatal(err)
	}

	got, err := resolveRootDir("")
	if err != nil {
		t.Fatal(err)
	}

	want, err := filepath.Abs(cwd)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("expected working directory %q, got %q", want, got)
	}
}

func TestResolveRootDirRejectsUnknownWorkingDirectory(t *testing.T) {
	cwd := t.TempDir()
	configHome := t.TempDir()

	t.Setenv("ZK_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", configHome)

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(previousWD); chdirErr != nil {
			t.Fatalf("restore working directory: %v", chdirErr)
		}
	})

	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}

	_, err = resolveRootDir("")
	if err == nil {
		t.Fatal("expected resolveRootDir to reject a non-data working directory")
	}
	if !strings.Contains(err.Error(), "configured data root") {
		t.Fatalf("expected guidance about configuring the data root, got: %v", err)
	}
}
