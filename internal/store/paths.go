package store

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

const (
	bibliographyFileName = "bibliography.json"
	stateDBFileName      = "srs.db"
)

func stateDBPathForRoot(root string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	stateHome, err := stateHomePath()
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256([]byte(absRoot))
	key := sanitizePathComponent(filepath.Base(absRoot))
	if key == "" {
		key = "root"
	}

	return filepath.Join(stateHome, "zk", "roots", key+"-"+hex.EncodeToString(sum[:8]), stateDBFileName), nil
}

func bibliographyPathForRoot(root string) string {
	return filepath.Join(root, bibliographyFileName)
}

func stateHomePath() (string, error) {
	if stateHome := os.Getenv("XDG_STATE_HOME"); stateHome != "" {
		return stateHome, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".local", "state"), nil
}

func sanitizePathComponent(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
