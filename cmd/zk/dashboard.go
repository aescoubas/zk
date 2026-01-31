package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/escoubas/zk/internal/model"
	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Open the Zettelkasten interactive dashboard",
	Run: func(cmd *cobra.Command, args []string) {
		runDashboard()
	},
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}

func runDashboard() {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	dbPath := filepath.Join(absRoot, ".zk", "index.db")

	st, err := store.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening DB (run 'zk index' first): %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	// Fetch initial data
	noteCount, linkCount, err := st.GetStats()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting stats: %v\n", err)
		os.Exit(1)
	}

	recents, err := st.GetRecentNotes(5)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting recent notes: %v\n", err)
		os.Exit(1)
	}

	m := dashboardModel{
		store:     st,
		root:      absRoot,
		noteCount: noteCount,
		linkCount: linkCount,
		recents:   recents,
		cursor:    0,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running dashboard: %v\n", err)
		os.Exit(1)
	}
}

type dashboardModel struct {
	store     *store.Store
	root      string
	noteCount int
	linkCount int
	recents   []*model.Note
	cursor    int // 0-4 for recents? Or just actions?
	quitting  bool
}

func (m dashboardModel) Init() tea.Cmd {
	return nil
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.recents)-1 {
				m.cursor++
			}
		case "enter":
			// Open selected recent note
			if len(m.recents) > 0 {
				return m, openEditor(m.root, m.recents[m.cursor].Path)
			}
		case "r":
			// Random
			n, err := m.store.GetRandomNote()
			if err == nil {
				return m, openEditor(m.root, n.Path)
			}
		case "e":
			// Explore - trigger explore command?
			// For now, let's just exit and print a message, or better, we need a way to switch models.
			// Swapping models in Bubbletea is possible but complex if not planned.
			// Simpler: Execute the explore command as a subprocess? No, that's messy TUI in TUI.
			// Best: We should restructure main to have a SessionModel that switches views.
			// For this iteration, let's just print instructions or support basic actions.
			m.quitting = true
			return m, tea.ExecProcess(exec.Command("zk", "explore"), func(err error) tea.Msg {
				return nil // Maybe refresh?
			})
		case "n":
			// New Note
			// We can use ExecProcess to run 'zk new' (interactive) or just open editor on new file
			// But 'zk new' asks for title via args.
			// We could implement a simple input for title here.
			// Deferred for now.
		}
	}
	return m, nil
}

func (m dashboardModel) View() string {
	if m.quitting {
		return ""
	}

	// Styles
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).MarginBottom(1)
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).MarginBottom(1)
	itemStyle := lipgloss.NewStyle().PaddingLeft(2)
	selectedStyle := lipgloss.NewStyle().PaddingLeft(0).Foreground(lipgloss.Color("170")).SetString("> ")
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(2)

	// Build View
	s := titleStyle.Render("Zettelkasten Dashboard") + "\n"
	
s += headerStyle.Render(fmt.Sprintf("Stats: %d Notes, %d Links", m.noteCount, m.linkCount)) + "\n\n"
	
s += headerStyle.Render("Recent Notes") + "\n"
	
	for i, n := range m.recents {
		line := fmt.Sprintf("%s (%s)", n.Title, n.ModTime.Format("2006-01-02"))
		if i == m.cursor {
			s += selectedStyle.Render(line) + "\n"
		} else {
			s += itemStyle.Render(line) + "\n"
		}
	}

	s += helpStyle.Render("Actions: [Enter] Open Note | [r] Random | [e] Explore | [q] Quit")

	return lipgloss.NewStyle().Margin(1, 2).Render(s)
}

func openEditor(root, path string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	fullPath := filepath.Join(root, path)
	c := exec.Command(editor, fullPath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return nil // Maybe refresh?
	})
}
