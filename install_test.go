package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallScriptSkipsOptionalRuntimeManagement(t *testing.T) {
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	prefix := filepath.Join(tmp, "prefix")
	home := filepath.Join(tmp, "home")
	xdgConfig := filepath.Join(tmp, "xdg")
	dataDir := filepath.Join(tmp, "data")
	stubDir := filepath.Join(tmp, "stubs")
	callLog := filepath.Join(tmp, "calls.log")

	dirs := []string{
		prefix,
		home,
		xdgConfig,
		dataDir,
		stubDir,
		filepath.Join(prefix, "share", "bash-completion", "completions"),
		filepath.Join(prefix, "share", "zsh", "site-functions"),
		filepath.Join(prefix, "share", "fish", "vendor_completions.d"),
		filepath.Join(home, ".local", "share", "bash-completion", "completions"),
		filepath.Join(home, ".zsh", "completions"),
		filepath.Join(home, ".config", "fish", "completions"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(dataDir, "README.md"), []byte("test data root\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	scripts := map[string]string{
		"go": `#!/bin/sh
set -eu
printf 'go %s\n' "$*" >> "$ZK_INSTALL_TEST_LOG"
if [ "${1:-}" = "build" ]; then
	out=""
	while [ "$#" -gt 0 ]; do
		if [ "$1" = "-o" ]; then
			out="$2"
			shift 2
			continue
		fi
		shift
	done
	cat > "$out" <<'EOF'
#!/bin/sh
set -eu
if [ "${1:-}" = "completion" ]; then
	printf '# completion for %s\n' "${2:-unknown}"
	exit 0
fi
printf 'fake zk %s\n' "$*"
EOF
	chmod +x "$out"
fi
`,
		"curl": `#!/bin/sh
set -eu
printf 'curl %s\n' "$*" >> "$ZK_INSTALL_TEST_LOG"
exit 1
`,
		"ollama": `#!/bin/sh
set -eu
printf 'ollama %s\n' "$*" >> "$ZK_INSTALL_TEST_LOG"
exit 0
`,
		"pkill": `#!/bin/sh
set -eu
printf 'pkill %s\n' "$*" >> "$ZK_INSTALL_TEST_LOG"
exit 0
`,
		"sudo": `#!/bin/sh
set -eu
printf 'sudo %s\n' "$*" >> "$ZK_INSTALL_TEST_LOG"
exit 0
`,
		"systemctl": `#!/bin/sh
set -eu
printf 'systemctl %s\n' "$*" >> "$ZK_INSTALL_TEST_LOG"
exit 0
`,
		"sleep": `#!/bin/sh
exit 0
`,
	}
	for name, content := range scripts {
		if err := os.WriteFile(filepath.Join(stubDir, name), []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cmd := exec.Command("bash", "./install.sh", "--prefix", prefix, "--data-dir", dataDir)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"XDG_CONFIG_HOME="+xdgConfig,
		"PATH="+stubDir+":"+os.Getenv("PATH"),
		"ZK_INSTALL_TEST_LOG="+callLog,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, out)
	}

	configPath := filepath.Join(xdgConfig, "zk", "root")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected config file %s: %v", configPath, err)
	}
	if strings.TrimSpace(string(configData)) != dataDir {
		t.Fatalf("expected config root %q, got %q", dataDir, strings.TrimSpace(string(configData)))
	}

	for _, path := range []string{
		filepath.Join(prefix, "bin", "zk"),
		filepath.Join(prefix, "share", "bash-completion", "completions", "zk"),
		filepath.Join(prefix, "share", "zsh", "site-functions", "_zk"),
		filepath.Join(prefix, "share", "fish", "vendor_completions.d", "zk.fish"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected installed file %s: %v", path, err)
		}
	}

	callData, err := os.ReadFile(callLog)
	if err != nil {
		t.Fatalf("expected call log %s: %v", callLog, err)
	}
	calls := string(callData)
	for _, forbidden := range []string{"curl ", "ollama ", "pkill ", "sudo ", "systemctl "} {
		if strings.Contains(calls, forbidden) {
			t.Fatalf("installer should not invoke %q during default install, calls:\n%s", strings.TrimSpace(forbidden), calls)
		}
	}
}
