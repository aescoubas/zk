package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/escoubas/zk/internal/model"
	"github.com/escoubas/zk/internal/store"
)

type searchModel struct {
	store    *store.Store
	root     string
	list     list.Model
	selected *model.NoteSummary
	width    int
	height   int
}

type noteItem struct {
	summary model.NoteSummary
}

func (i noteItem) Title() string       { return i.summary.Title }
func (i noteItem) Description() string { 
	return fmt.Sprintf("%s | In: %d | Out: %d | Modified: %s", 
		i.summary.ID, 
		i.summary.Backlinks, 
		i.summary.OutgoingLinks, 
		i.summary.ModTime.Format("2006-01-02"),
	)
}
func (i noteItem) FilterValue() string { return i.summary.Title + " " + i.summary.ID }

type noteItemDelegate struct{}

func (d noteItemDelegate) Height() int                               { return 1 }
func (d noteItemDelegate) Spacing() int                              { return 0 }
func (d noteItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d noteItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(noteItem)
	if !ok {
		return
	}

	str := fmt.Sprintf("%d. %s", index+1, i.Title())

	fn := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxFg)).Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxOrangeBright)).Bold(true).SetString("> ").Render(strings.Join(s, " "))
		}
	}

	fmt.Fprint(w, fn(str))
}

func newSearchModel(st *store.Store, root string) searchModel {
	summaries, _ := st.ListNoteSummaries()
	items := make([]list.Item, len(summaries))
	for i, s := range summaries {
		items[i] = noteItem{summary: *s}
	}

	// Use default delegate for now, gives us Title+Desc
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color(GruvboxOrangeBright)).BorderLeftForeground(lipgloss.Color(GruvboxOrangeBright))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color(GruvboxYellow)).BorderLeftForeground(lipgloss.Color(GruvboxOrangeBright))

	l := list.New(items, delegate, 0, 0)
	l.Title = "Search Notes"
	l.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxAqua)).Bold(true)
	
	return searchModel{
		store: st,
		root:  root,
		list:  l,
	}
}

func (m searchModel) Init() tea.Cmd {
	return nil
}

func (m searchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break 
		}
		switch msg.String() {
		case "enter":
			if i, ok := m.list.SelectedItem().(noteItem); ok {
				// Fetch full note
				n, err := m.store.GetNote(i.summary.ID)
				if err == nil {
					return m, func() tea.Msg { return navigateToExploreMsg{note: n} }
				}
			}
		case "e":
			if i, ok := m.list.SelectedItem().(noteItem); ok {
				n, err := m.store.GetNote(i.summary.ID)
				if err == nil {
					return m, openEditor(m.root, n.Path)
				}
			}
		case "esc":
			return m, func() tea.Msg { return navigateToDashboardMsg{} }
		}
	
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height)
	}

	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m searchModel) View() string {
	return m.list.View()
}