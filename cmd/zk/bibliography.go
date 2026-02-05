package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/escoubas/zk/internal/model"
	"github.com/escoubas/zk/internal/store"
)

// refItem wraps a RefSummary as a list.Item.
type refItem struct {
	summary model.RefSummary
}

func (i refItem) Title() string {
	return fmt.Sprintf("[%d] %s", i.summary.Citations, i.summary.Ref.Title)
}

func (i refItem) Description() string {
	parts := []string{}
	parts = append(parts, "@"+i.summary.Ref.ID)
	if i.summary.Ref.Author != "" {
		parts = append(parts, i.summary.Ref.Author)
	}
	if i.summary.Ref.Year != "" {
		parts = append(parts, i.summary.Ref.Year)
	}
	if i.summary.Ref.Type != "" {
		parts = append(parts, i.summary.Ref.Type)
	}
	return strings.Join(parts, " | ")
}

func (i refItem) FilterValue() string {
	return i.summary.Ref.Title + " " + i.summary.Ref.Author + " " + i.summary.Ref.ID
}

// bibliographyModel is the TUI for browsing references.
type bibliographyModel struct {
	store  *store.Store
	root   string
	list   list.Model
	width  int
	height int

	// Detail view: citing notes for the selected ref
	showDetail   bool
	detailRefID  string
	detailTitle  string
	citingNotes  []*model.Note
	detailCursor int
	detailOffset int
}

func newBibliographyModel(st *store.Store, root string) bibliographyModel {
	summaries, _ := st.ListRefSummaries()
	items := make([]list.Item, len(summaries))
	for i, s := range summaries {
		items[i] = refItem{summary: s}
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color(GruvboxOrangeBright)).
		BorderLeftForeground(lipgloss.Color(GruvboxOrangeBright))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color(GruvboxYellow)).
		BorderLeftForeground(lipgloss.Color(GruvboxOrangeBright))

	l := list.New(items, delegate, 0, 0)
	l.Title = "Bibliography"
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(lipgloss.Color(GruvboxPurpleBright)).Bold(true)

	return bibliographyModel{
		store: st,
		root:  root,
		list:  l,
	}
}

func (m bibliographyModel) Init() tea.Cmd { return nil }

func (m bibliographyModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Detail view keybindings
		if m.showDetail {
			switch msg.String() {
			case "esc", "q":
				m.showDetail = false
				m.citingNotes = nil
				m.detailCursor = 0
				m.detailOffset = 0
				return m, nil
			case "j", "down":
				if m.detailCursor < len(m.citingNotes)-1 {
					m.detailCursor++
					listHeight := m.height - 8
					if listHeight < 1 {
						listHeight = 1
					}
					if m.detailCursor >= m.detailOffset+listHeight {
						m.detailOffset++
					}
				}
				return m, nil
			case "k", "up":
				if m.detailCursor > 0 {
					m.detailCursor--
					if m.detailCursor < m.detailOffset {
						m.detailOffset = m.detailCursor
					}
				}
				return m, nil
			case "enter":
				if len(m.citingNotes) > 0 && m.detailCursor < len(m.citingNotes) {
					note := m.citingNotes[m.detailCursor]
					return m, func() tea.Msg { return navigateToExploreMsg{note: note} }
				}
				return m, nil
			}
			return m, nil
		}

		// List view keybindings
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "esc", "q":
			return m, func() tea.Msg { return navigateToDashboardMsg{} }
		case "enter":
			if i, ok := m.list.SelectedItem().(refItem); ok {
				notes, _ := m.store.GetCitingNotes(i.summary.Ref.ID)
				m.showDetail = true
				m.detailRefID = i.summary.Ref.ID
				m.detailTitle = i.summary.Ref.Title
				m.citingNotes = notes
				m.detailCursor = 0
				m.detailOffset = 0
				return m, nil
			}
		case "d":
			if i, ok := m.list.SelectedItem().(refItem); ok {
				_ = m.store.DeleteRef(i.summary.Ref.ID)
				// Refresh list
				return newBibliographyModel(m.store, m.root), nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-2)
	}

	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m bibliographyModel) View() string {
	if m.showDetail {
		return m.detailView()
	}

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(GruvboxGray)).
		Width(m.width).
		Align(lipgloss.Center)
	helpText := "[Enter] Citing Notes | [/] Filter | [d] Delete | [Esc] Dashboard"

	return lipgloss.JoinVertical(lipgloss.Left,
		m.list.View(), "\n", helpStyle.Render(helpText))
}

func (m bibliographyModel) detailView() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(GruvboxYellowBright)).
		Bold(true).
		Width(m.width).
		Align(lipgloss.Center)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(GruvboxPurpleBright)).
		Width(m.width).
		Align(lipgloss.Center)

	itemStyle := lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color(GruvboxFg))
	selectedStyle := lipgloss.NewStyle().PaddingLeft(0).Foreground(lipgloss.Color(GruvboxOrangeBright)).SetString("> ")

	title := titleStyle.Render(fmt.Sprintf("Citing Notes for \"%s\"", m.detailTitle))
	subtitle := subtitleStyle.Render(fmt.Sprintf("[@%s] — %d note(s)", m.detailRefID, len(m.citingNotes)))

	content := ""
	if len(m.citingNotes) == 0 {
		content = lipgloss.NewStyle().
			Foreground(lipgloss.Color(GruvboxGray)).
			PaddingLeft(2).
			Render("No notes cite this reference yet.")
	} else {
		listHeight := m.height - 8
		if listHeight < 1 {
			listHeight = 1
		}
		start := m.detailOffset
		end := m.detailOffset + listHeight
		if end > len(m.citingNotes) {
			end = len(m.citingNotes)
		}
		for i := start; i < end; i++ {
			n := m.citingNotes[i]
			line := n.Title
			if i == m.detailCursor {
				content += selectedStyle.Render(line) + "\n"
			} else {
				content += itemStyle.Render(line) + "\n"
			}
		}
	}

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(GruvboxGray)).
		Width(m.width).
		Align(lipgloss.Center)
	helpText := "[Enter] Open Note | [j/k] Navigate | [Esc] Back to Bibliography"

	return lipgloss.JoinVertical(lipgloss.Left,
		title, subtitle, "\n", content, "\n", helpStyle.Render(helpText))
}
