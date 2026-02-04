package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/escoubas/zk/internal/store"
)

func TestDashboardLogic(t *testing.T) {
	// Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "zk_dashboard_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Set global rootDir for createNoteFile
	rootDir = tmpDir

	// Init DB
	dbPath := filepath.Join(tmpDir, ".zk", "index.db")
	st, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// Init Dashboard
	m, err := NewDashboardModel(st, tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Test Help Toggle
	if m.showHelp {
		t.Error("Help should be off by default")
	}

	// Press '?'
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	dash := m2.(dashboardModel)
	if !dash.showHelp {
		t.Error("Help should be on after pressing '?'")
	}

	// Press '?' again
	m3, _ := dash.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	dash = m3.(dashboardModel)
	if dash.showHelp {
		t.Error("Help should be off after pressing '?' again")
	}

	// 2. Test New Note Mode
	if dash.mode != modeNormal {
		t.Errorf("Expected modeNormal, got %d", dash.mode)
	}

	// Press 'n'
	m4, _ := dash.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	dash = m4.(dashboardModel)
	if dash.mode != modeCreateNote {
		t.Errorf("Expected modeCreateNote, got %d", dash.mode)
	}
	if !dash.createInput.Focused() {
		t.Error("Create Input should be focused")
	}

	// Type "My Note"
	// Simulating text input is a bit tedious message by message, but let's try injecting the value directly 
	// since we trust textinput model works.
	dash.createInput.SetValue("My Test Note")

	// Press Enter
	// This calls createNoteFile which uses global rootDir (tmpDir)
	// And then calls openEditor. openEditor uses tea.ExecProcess.
	// We can't easily check ExecProcess result in unit test without running the command.
	// But we can check if file was created and mode reset.
	
	m5, cmd := dash.Update(tea.KeyMsg{Type: tea.KeyEnter})
	dash = m5.(dashboardModel)
	
	// Mode should be reset to Normal
	if dash.mode != modeNormal {
		t.Errorf("Expected modeNormal after creation, got %d", dash.mode)
	}
	
	// File should exist
	// Timestamp? We don't know exact timestamp.
	// But we know it's in zettels/
	// Check if any file exists in zettels/
	matches, _ := filepath.Glob(filepath.Join(tmpDir, "zettels", "*.md"))
	if len(matches) == 0 {
		// Maybe it fell back to root?
		matches, _ = filepath.Glob(filepath.Join(tmpDir, "*.md"))
	}
	
	if len(matches) == 0 {
		t.Error("No note file created")
	} else {
		// Read content to verify title
		content, _ := os.ReadFile(matches[0])
		if !strings.Contains(string(content), "My Test Note") {
			t.Errorf("Note content missing title. Got: %s", string(content))
		}
	}
	
	// Check cmd
	if cmd == nil {
		t.Error("Expected command to open editor")
	}
}
