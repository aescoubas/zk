package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/escoubas/zk/internal/model"
	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

var exploreCmd = &cobra.Command{
	Use:   "explore [note_id]",
	Short: "Interactive graph explorer",
	Run: func(cmd *cobra.Command, args []string) {
		startID := ""
		if len(args) > 0 {
			startID = args[0]
		}
		runExplore(startID)
	},
}

func init() {
	rootCmd.AddCommand(exploreCmd)
}

func runExplore(startID string) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	dbPath := filepath.Join(absRoot, ".zk", "index.db")

	st, err := store.NewStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening DB: %v\n", err)
		os.Exit(1)
	}
	defer st.Close()

	// Resolve start note
	var startNote *model.Note
	
	if startID != "" {
		startNote, err = st.GetNote(startID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Note not found: %s\n", startID)
		}
	} else {
		// Try to find "index" or "readme"
		candidates := []string{"index", "Index", "readme", "README", "000-index", "Home"}
		for _, id := range candidates {
			n, err := st.GetNote(id)
			if err == nil {
				startNote = n
				break
			}
		}
	}
	
	if startNote == nil {
		if startID == "" {
			fmt.Println("No index note found. Opening random note...")
		}
		startNote, err = st.GetRandomNote()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting random note: %v\n", err)
			os.Exit(1)
		}
	}

	m := initializeExploreModel(st, absRoot, startNote)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running explorer: %v\n", err)
		os.Exit(1)
	}
}

const (
	focusIncoming = iota
	focusContent
	focusOutgoing
)

type exploreModel struct {
	store    *store.Store
	root     string
	
	current  *model.Note
	history  []string // Stack of IDs
	
incoming []list.Item
	outgoing []list.Item
	
	listIn   list.Model
	listOut  list.Model
	viewport viewport.Model
	
	focus    int // focusIncoming, focusContent, focusOutgoing
	width    int
	height   int
	
	renderer *glamour.TermRenderer
}

func initializeExploreModel(st *store.Store, root string, start *model.Note) exploreModel {
	m := exploreModel{
		store:   st,
		root:    root,
		current: start,
		focus:   focusOutgoing,
	}
	
	// Init Lists
	m.listIn = list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	m.listIn.Title = "Backlinks"
	m.listIn.SetShowHelp(false)
	
	m.listOut = list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	m.listOut.Title = "Links To"
	m.listOut.SetShowHelp(false)

	// Init Viewport for content
	m.viewport = viewport.New(0, 0)
	
	// Init Glamour
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	m.renderer = renderer

	m.loadData()
	return m
}

func (m *exploreModel) loadData() {
	if m.current == nil {
		return
	}

	// 1. Incoming
	backlinks, _ := m.store.GetBacklinks(m.current.ID)
	itemsIn := make([]list.Item, len(backlinks))
	for i, n := range backlinks {
		itemsIn[i] = item{note: n}
	}
	m.listIn.SetItems(itemsIn)

	// 2. Outgoing
	var itemsOut []list.Item
	for _, l := range m.current.OutgoingLinks {
		n, err := m.store.GetNote(l.TargetID)
		if err == nil {
			itemsOut = append(itemsOut, item{note: n})
		} else {
			itemsOut = append(itemsOut, item{note: &model.Note{ID: l.TargetID, Title: l.DisplayText + " (Missing)"}})
		}
	}
	m.listOut.SetItems(itemsOut)

	// 3. Content Viewport
	contentBytes, err := os.ReadFile(filepath.Join(m.root, m.current.Path))
	content := ""
	if err == nil {
		content = string(contentBytes)
	}
	
	// Render Markdown
	rendered, err := m.renderer.Render(content)
	if err != nil {
		rendered = content // Fallback
	}
	
m.viewport.SetContent(rendered)
	// Reset scroll
	m.viewport.GotoTop()
}

func (m exploreModel) Init() tea.Cmd {
	return nil
}

func (m exploreModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			m.focus = (m.focus + 1) % 3
		case "shift+tab":
			m.focus = (m.focus - 1 + 3) % 3
		case "left", "h":
			if m.focus > 0 {
				m.focus--
			}
		case "right", "l":
			if m.focus < 2 {
				m.focus++
			}
		case "enter":
			// Navigate to selected
			var selected *model.Note
			if m.focus == focusIncoming {
				if i, ok := m.listIn.SelectedItem().(item); ok {
					selected = i.note
				}
			} else if m.focus == focusOutgoing {
				if i, ok := m.listOut.SelectedItem().(item); ok {
					selected = i.note
				}
			}
			
			if selected != nil && selected.Path != "" {
				m.history = append(m.history, m.current.ID)
				m.current = selected
				m.loadData()
			}
		case "backspace":
			if len(m.history) > 0 {
				prevID := m.history[len(m.history)-1]
				m.history = m.history[:len(m.history)-1]
				
				n, err := m.store.GetNote(prevID)
				if err == nil {
					m.current = n
					m.loadData()
				}
			}
		case "o":
			return m, openEditor(m.root, m.current.Path)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		colWidth := msg.Width / 4
		centerWidth := msg.Width - (2 * colWidth) - 4
		
		listHeight := msg.Height - 4
		
		m.listIn.SetSize(colWidth, listHeight)
		m.listOut.SetSize(colWidth, listHeight)
		m.viewport.Width = centerWidth
		m.viewport.Height = listHeight
		
		// Update renderer width
		m.renderer, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(centerWidth),
		)
		// Re-render content
		m.loadData()
	}

	if m.focus == focusIncoming {
		m.listIn, cmd = m.listIn.Update(msg)
	} else if m.focus == focusOutgoing {
		m.listOut, cmd = m.listOut.Update(msg)
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
	}
	
	return m, cmd
}

func (m exploreModel) View() string {
	// Styles
	activeBorder := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62"))
	inactiveBorder := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	
	var inStyle, outStyle, centerStyle lipgloss.Style
	
	if m.focus == focusIncoming {
		inStyle = activeBorder
	} else {
		inStyle = inactiveBorder
	}
	
	if m.focus == focusOutgoing {
		outStyle = activeBorder
	} else {
		outStyle = inactiveBorder
	}

	if m.focus == focusContent {
		centerStyle = activeBorder.Copy().Padding(0, 1)
	} else {
		centerStyle = inactiveBorder.Copy().Padding(0, 1)
	}
	
	// Render
	inView := inStyle.Render(m.listIn.View())
	outView := outStyle.Render(m.listOut.View())
	
	// Center Header
	centerHeader := lipgloss.NewStyle().Bold(true).Render(m.current.Title)
	centerContent := m.viewport.View()
	centerView := centerStyle.Render(fmt.Sprintf("%s\n\n%s", centerHeader, centerContent))
	
	return lipgloss.JoinHorizontal(lipgloss.Top, inView, centerView, outView)
}