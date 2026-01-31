package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/escoubas/zk/internal/model"
	"github.com/escoubas/zk/internal/srs"
	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Review notes (SRS)",
	Run: func(cmd *cobra.Command, args []string) {
		runReview()
	},
}

func init() {
	rootCmd.AddCommand(reviewCmd)
}

func runReview() {
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

	// 1. Get Due Reviews
	notes, err := st.GetDueReviews()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting due reviews: %v\n", err)
		os.Exit(1)
	}

	// 2. If no reviews, find stale notes
	if len(notes) == 0 {
		fmt.Println("No reviews due. Looking for stale notes...")
		stale, err := st.GetStaleNotes(180 * 24 * time.Hour) // 6 months
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting stale notes: %v\n", err)
			os.Exit(1)
		}
		if len(stale) == 0 {
			fmt.Println("No stale notes found either. You're all caught up!")
			return
		}
		
		// Filter out notes already in SRS
		var candidates []*model.Note
		for _, n := range stale {
			item, _ := st.GetSRSItem(n.ID)
			if item == nil {
				candidates = append(candidates, n)
			}
		}
		
		if len(candidates) == 0 {
			fmt.Println("No new stale notes to add (others are already scheduled).")
			return
		}

		// Limit to 10 stale notes per session
		limit := 10
		if len(candidates) < limit {
			limit = len(candidates)
		}
		notes = candidates[:limit]
		fmt.Printf("Added %d stale notes to review queue.\n", len(notes))
	}

	// 3. Start TUI
	m := newReviewModel(st, notes, absRoot)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running review: %v\n", err)
		os.Exit(1)
	}
}

type reviewModel struct {
	store    *store.Store
	queue    []*model.Note
	current  *model.Note
	root     string
	viewport viewport.Model
	quitting bool
	showing  bool // true = content shown, false = just title (front/back card style? or just show all?)
	// For now, let's show all content immediately, as Zettels are "atomic" and should be read.
}

func newReviewModel(st *store.Store, notes []*model.Note, root string) reviewModel {
	m := reviewModel{
		store:   st,
		queue:   notes,
		root:    root,
		showing: true,
	}
	if len(m.queue) > 0 {
		m.current = m.queue[0]
		m.queue = m.queue[1:]
	}
	
vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).Padding(1, 2)
	m.viewport = vp
	
m.updateContent()
	return m
}

func (m *reviewModel) updateContent() {
	if m.current == nil {
		return
	}
	
	// Read file content
	contentBytes, err := os.ReadFile(filepath.Join(m.root, m.current.Path))
	content := ""
	if err != nil {
		content = fmt.Sprintf("Error reading file: %v", err)
	} else {
		content = string(contentBytes)
	}

	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render
	
	header := fmt.Sprintf("%s\n%s\n", titleStyle(m.current.Title), strings.Repeat("─", len(m.current.Title)))
	m.viewport.SetContent(header + content)
}

func (m reviewModel) Init() tea.Cmd {
	return nil
}

func (m reviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "1", "2", "3", "4", "5":
			// Rating
			rating := srs.Rating(0)
			switch msg.String() {
			case "1": rating = srs.RatingIncorrect
			case "2": rating = srs.RatingHard
			case "3": rating = srs.RatingPass
			case "4": rating = srs.RatingGood
			case "5": rating = srs.RatingEasy
			}
			m.processReview(rating)
			
			// Next note
			if len(m.queue) == 0 {
				m.current = nil
				m.quitting = true
				return m, tea.Quit
			}
			m.current = m.queue[0]
			m.queue = m.queue[1:]
			m.updateContent()
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 10
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *reviewModel) processReview(rating srs.Rating) {
	if m.current == nil {
		return
	}

	// 1. Get existing SRS state or create new
	item, err := m.store.GetSRSItem(m.current.ID)
	if err != nil {
		// Handle error?
	}
	if item == nil {
		item = srs.InitialState(m.current.ID)
	}

	// 2. Apply Algorithm
	srs.Review(item, rating)

	// 3. Save
	m.store.SaveSRSItem(item)
}

func (m reviewModel) View() string {
	if m.quitting {
		return "Review session finished.\n"
	}
	if m.current == nil {
		return "No more notes to review.\n"
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render

	return fmt.Sprintf(
		"\n%s\n\n%s\n",
		m.viewport.View(),
		helpStyle("Rate recall: 1 (Fail) - 5 (Perfect) | q: Quit | Arrows/jk: Scroll"),
	)
}
