package main

import (
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/escoubas/zk/internal/model"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type selectorModel struct {
	list     list.Model
	selected *model.Note
	quitting bool
}

func (m selectorModel) Init() tea.Cmd {
	return nil
}

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
		if msg.String() == "enter" {
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.selected = i.note
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m selectorModel) View() string {
	if m.quitting {
		return ""
	}
	return docStyle.Render(m.list.View())
}

// RunSelector runs the TUI selector and returns the selected note (or nil).
func RunSelector(notes []*model.Note, title string) (*model.Note, error) {
	items := make([]list.Item, len(notes))
	for i, n := range notes {
		items[i] = item{note: n}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = title
	l.SetShowStatusBar(false)

	m := selectorModel{list: l}

	// Run with output to Stderr so Stdout can be used for the result if needed
	p := tea.NewProgram(m, tea.WithOutput(os.Stderr))
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	if m, ok := finalModel.(selectorModel); ok {
		return m.selected, nil
	}
	return nil, nil
}

// item wraps model.Note to implement list.Item
type item struct {
	note *model.Note
}

func (i item) Title() string       { return i.note.Title }
func (i item) Description() string { return i.note.ID }
func (i item) FilterValue() string { return i.note.Title + " " + i.note.ID }