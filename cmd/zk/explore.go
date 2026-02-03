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
	focusSimilar
)

type exploreModel struct {
	store    *store.Store
	root     string
	
	current  *model.Note
	history  []string
	
	incoming []list.Item
	outgoing []list.Item
	similar  []list.Item
	
	listIn      list.Model
	listOut     list.Model
	listSimilar list.Model
	viewport    viewport.Model
	
	focus    int
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
	m.listIn.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxAqua)).Bold(true)
	
	m.listOut = list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	m.listOut.Title = "Links To"
	m.listOut.SetShowHelp(false)
	m.listOut.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxBlue)).Bold(true)

	m.listSimilar = list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	m.listSimilar.Title = "Similar"
	m.listSimilar.SetShowHelp(false)
	m.listSimilar.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxPurple)).Bold(true)

	// Init Viewport
	m.viewport = viewport.New(0, 0)
	
	// Init Glamour with Gruvbox Style
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStylesFromJSONBytes([]byte(gruvboxStyle)),
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

	// 3. Similar
	similars, err := m.store.FindSimilar(m.current.ID, 10)
	var itemsSim []list.Item
	if err == nil {
		for _, s := range similars {
			itemsSim = append(itemsSim, similarItem{note: s.Note, score: s.Score})
		}
	}
	m.listSimilar.SetItems(itemsSim)

	// 4. Content Viewport
	contentBytes, err := os.ReadFile(filepath.Join(m.root, m.current.Path))
	content := ""
	if err == nil {
		content = string(contentBytes)
	}
	
	rendered, err := m.renderer.Render(content)
	if err != nil {
		rendered = content
	}
	
m.viewport.SetContent(rendered)
	m.viewport.GotoTop()
}

func (m exploreModel) Init() tea.Cmd {
	return nil
}

type similarItem struct {
	note  *model.Note
	score float64
}

func (i similarItem) Title() string       { return fmt.Sprintf("%.2f %s", i.score, i.note.Title) }
func (i similarItem) Description() string { return i.note.ID }
func (i similarItem) FilterValue() string { return i.note.Title }

func (m exploreModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, func() tea.Msg { return navigateToDashboardMsg{} }
		case "/":
			return m, func() tea.Msg { return navigateToSearchMsg{} }
		case "tab":
			m.focus = (m.focus + 1) % 4
		case "shift+tab":
			m.focus = (m.focus - 1 + 4) % 4
		case "left", "h":
			if m.focus > 0 {
				m.focus--
			}
		case "right", "l":
			if m.focus < 3 {
				m.focus++
			}
		case "enter":
			var selected *model.Note
			if m.focus == focusIncoming {
				if i, ok := m.listIn.SelectedItem().(item); ok {
					selected = i.note
				}
			} else if m.focus == focusOutgoing {
				if i, ok := m.listOut.SelectedItem().(item); ok {
					selected = i.note
				}
			} else if m.focus == focusSimilar {
				if i, ok := m.listSimilar.SelectedItem().(similarItem); ok {
					selected = i.note
				}
			}
			
			if selected != nil && selected.Path != "" {
				m.history = append(m.history, m.current.ID)
				m.current = selected
				m.loadData()
			}
		case "+", "a":
			if m.focus == focusSimilar {
				if i, ok := m.listSimilar.SelectedItem().(similarItem); ok {
					f, err := os.OpenFile(filepath.Join(m.root, m.current.Path), os.O_APPEND|os.O_WRONLY, 0644)
					if err == nil {
						link := fmt.Sprintf("\n- Related: [[%s]]\n", i.note.Title) 
						f.WriteString(link)
						f.Close()
						m.loadData()
					}
				}
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
		
		// Calculate available width for content by subtracting borders and padding
		// 3 side columns: 2 chars border each = 6
		// 1 center column: 2 chars border + 2 chars padding = 4
		// Total overhead = 10
		availableWidth := msg.Width - 10
		if availableWidth < 0 {
			availableWidth = 0
		}
		
		// Assign ~45% to center, rest divided by 3 for sides (~18% each)
		centerWidth := int(float64(availableWidth) * 0.45)
		colWidth := (availableWidth - centerWidth) / 3
		
		// Recalculate center to absorb rounding errors and fill space
		centerWidth = availableWidth - (colWidth * 3)

		if centerWidth < 20 {
			centerWidth = 20
		}
		
		listHeight := msg.Height - 4
		
		m.listIn.SetSize(colWidth, listHeight)
		m.listOut.SetSize(colWidth, listHeight)
		m.listSimilar.SetSize(colWidth, listHeight)
		
		m.viewport.Width = centerWidth
		m.viewport.Height = listHeight
		
		// Update renderer width with Gruvbox style
		m.renderer, _ = glamour.NewTermRenderer(
			glamour.WithStylesFromJSONBytes([]byte(gruvboxStyle)),
			glamour.WithWordWrap(centerWidth),
		)
		m.loadData()
	}

	if m.focus == focusIncoming {
		m.listIn, cmd = m.listIn.Update(msg)
	} else if m.focus == focusOutgoing {
		m.listOut, cmd = m.listOut.Update(msg)
	} else if m.focus == focusSimilar {
		m.listSimilar, cmd = m.listSimilar.Update(msg)
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
	}
	
	return m, cmd
}

func (m exploreModel) View() string {
	// Styles using Gruvbox Colors
	activeBorder := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(GruvboxOrangeBright))
	inactiveBorder := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(GruvboxGray))
	
	var inStyle, outStyle, simStyle, centerStyle lipgloss.Style
	
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

	if m.focus == focusSimilar {
		simStyle = activeBorder
	} else {
		simStyle = inactiveBorder
	}

	if m.focus == focusContent {
		centerStyle = activeBorder.Copy().Padding(0, 1)
	} else {
		centerStyle = inactiveBorder.Copy().Padding(0, 1)
	}
	
	// Render
	inView := inStyle.Render(m.listIn.View())
	outView := outStyle.Render(m.listOut.View())
	simView := simStyle.Render(m.listSimilar.View())
	
	// Center Header
	centerHeader := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(GruvboxYellowBright)).Render(m.current.Title)
	centerContent := m.viewport.View()
	centerView := centerStyle.Render(fmt.Sprintf("%s\n\n%s", centerHeader, centerContent))
	
	content := lipgloss.JoinHorizontal(lipgloss.Top, inView, centerView, outView, simView)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
