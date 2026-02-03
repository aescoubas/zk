package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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

	m, err := NewDashboardModel(st, absRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing dashboard: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running dashboard: %v\n", err)
		os.Exit(1)
	}
}

type dashboardModel struct {
	store       *store.Store
	root        string
	noteCount   int
	linkCount   int
	orphanCount int
	stubCount   int
	recents     []*model.Note
	topics      []model.TagCount
	snippet     string
	cursor      int
	quitting    bool
	width       int
	height      int
}

func NewDashboardModel(st *store.Store, root string) (dashboardModel, error) {
	noteCount, linkCount, err := st.GetStats()
	if err != nil {
		return dashboardModel{}, err
	}

	orphans, _ := st.GetOrphanCount()
	stubs, _ := st.GetStubCount()

	recents, err := st.GetRecentNotes(8)
	if err != nil {
		return dashboardModel{}, err
	}

	topics, _ := st.GetTopTags(8)
	snippet := getRandomSnippet(root)

	return dashboardModel{
		store:       st,
		root:        root,
		noteCount:   noteCount,
		linkCount:   linkCount,
		orphanCount: orphans,
		stubCount:   stubs,
		recents:     recents,
		topics:      topics,
		snippet:     snippet,
		cursor:      0,
		width:       80,
		height:      24,
	}, nil
}

func getRandomSnippet(root string) string {
	content, err := os.ReadFile(filepath.Join(root, "aphorisms.md"))
	if err != nil {
		return "Write something today!"
	}
	lines := strings.Split(string(content), "\n")
	var valid []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if len(l) > 5 && !strings.HasPrefix(l, "#") && !strings.HasPrefix(l, "---") {
			valid = append(valid, l)
		}
	}
	if len(valid) == 0 {
		return "No aphorisms found."
	}
	rand.Seed(time.Now().UnixNano())
	return valid[rand.Intn(len(valid))]
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
			if len(m.recents) > 0 {
				note := m.recents[m.cursor]
				return m, func() tea.Msg { return navigateToExploreMsg{note: note} }
			}
		case "r":
			n, err := m.store.GetRandomNote()
			if err == nil {
				return m, func() tea.Msg { return navigateToExploreMsg{note: n} }
			}
		case "e":
			return m, func() tea.Msg { return navigateToExploreMsg{note: nil} }
		case "s":
			return m, func() tea.Msg { return navigateToReviewMsg{} }
		case "l", "/":
			return m, func() tea.Msg { return navigateToSearchMsg{} }
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m dashboardModel) View() string {
	if m.quitting {
		return ""
	}

	// Calculate widths
	// Overhead per box: 2 (border) + 2 (padding) = 4
	// Total overhead for 3 columns: 12
	// We subtract a bit more (14) for safety
	totalOverhead := 14
	colWidth := (m.width - totalOverhead) / 3
	if colWidth < 20 {
		colWidth = 20
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(GruvboxYellowBright)).
		Bold(true).
		MarginBottom(1).
		Width(m.width).
		Align(lipgloss.Center)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(GruvboxBlue)).
		Padding(0, 1).
		Width(colWidth).
		Height(m.height - 10) // Slightly smaller to ensure fit

	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxAqua)).Bold(true).Underline(true)
	itemStyle := lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color(GruvboxFg))
	selectedStyle := lipgloss.NewStyle().PaddingLeft(0).Foreground(lipgloss.Color(GruvboxOrangeBright)).SetString("> ")
	statLabel := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxGray))
	statValue := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxPurpleBright)).Bold(true)
	
	snippetStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(GruvboxGrayBright)).
		Italic(true).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color(GruvboxGray)).
		Width(m.width - 4). // Slightly less than full width
		Align(lipgloss.Center)

	// Layout
	// Title
	title := titleStyle.Render("Zettelkasten Dashboard")

	// Snippet
	snippet := snippetStyle.Render(fmt.Sprintf("\"%s\"", m.snippet))

	// Stats Column
	statsContent := ""
	statsContent += fmt.Sprintf("%s %s\n", statLabel.Render("Notes:"), statValue.Render(fmt.Sprint(m.noteCount)))
	statsContent += fmt.Sprintf("%s %s\n", statLabel.Render("Links:"), statValue.Render(fmt.Sprint(m.linkCount)))
	statsContent += fmt.Sprintf("%s %s\n", statLabel.Render("Orphans:"), statValue.Render(fmt.Sprint(m.orphanCount)))
	statsContent += fmt.Sprintf("%s %s\n", statLabel.Render("Stubs:"), statValue.Render(fmt.Sprint(m.stubCount)))
	statsBox := boxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, headerStyle.Render("Stats"), statsContent))

	// Topics Column
	topicsContent := ""
	for _, t := range m.topics {
		topicsContent += fmt.Sprintf("%s (%d)\n", t.Tag, t.Count)
	}
	if len(m.topics) == 0 {
		topicsContent = "No tags found"
	}
	topicsBox := boxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, headerStyle.Render("Top Topics"), topicsContent))

	// Recents Column
	recentsContent := ""
	for i, n := range m.recents {
		line := fmt.Sprintf("%s", n.Title)
		if len(line) > colWidth-2 { // Truncate to fit column
			line = line[:colWidth-5] + "..."
		}
		if i == m.cursor {
			recentsContent += selectedStyle.Render(line) + "\n"
		} else {
			recentsContent += itemStyle.Render(line) + "\n"
		}
	}
	recentsBox := boxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, headerStyle.Render("Recents"), recentsContent))

	// Combine Columns
	// Center the columns horizontally
	columns := lipgloss.JoinHorizontal(lipgloss.Center, statsBox, topicsBox, recentsBox)
	columns = lipgloss.PlaceHorizontal(m.width, lipgloss.Center, columns)

	// Help
	help := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxGray)).Width(m.width).Align(lipgloss.Center).Render("Actions: [Enter] Explore | [l] Search | [r] Random | [s] Review | [q] Quit")

	// Full View
	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		snippet,
		"\n",
		columns,
		"\n",
		help,
	)
	
	// Ensure the entire block is centered in the terminal
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
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