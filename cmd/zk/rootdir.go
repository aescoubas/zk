package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveRootDir(flagValue string) (string, error) {
	candidates := []struct {
		source string
		value  string
	}{
		{source: "--dir", value: flagValue},
		{source: "ZK_PATH", value: os.Getenv("ZK_PATH")},
	}

	fileValue, err := readRootDirFile()
	if err != nil {
		return "", err
	}
	candidates = append(candidates, struct {
		source string
		value  string
	}{source: rootDirConfigLabel(), value: fileValue})

	for _, candidate := range candidates {
		candidate.value = strings.TrimSpace(candidate.value)
		if candidate.value == "" {
			continue
		}
		return validateDataRoot(candidate.value, candidate.source)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if looksLikeDataRoot(cwd) {
		return filepath.Abs(cwd)
	}

	return "", fmt.Errorf("no configured data root found. Use --dir, set ZK_PATH, or write %s. The current working directory does not look like a zettelkasten data root", rootDirConfigLabel())
}

func readRootDirFile() (string, error) {
	path, err := rootDirFilePath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

func rootDirFilePath() (string, error) {
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		return filepath.Join(configHome, "zk", "root"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".config", "zk", "root"), nil
}

func rootDirConfigLabel() string {
	if path, err := rootDirFilePath(); err == nil {
		return path
	}
	return "~/.config/zk/root"
}

func validateDataRoot(path, source string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if !looksLikeDataRoot(absPath) {
		return "", fmt.Errorf("%s points to %q, which is not a zettelkasten data root (missing zettels/)", source, absPath)
	}
	return absPath, nil
}

func looksLikeDataRoot(path string) bool {
	info, err := os.Stat(filepath.Join(path, "zettels"))
	if err != nil {
		return false
	}
	return info.IsDir()
}
