package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/escoubas/zk/internal/model"
	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

// Messages for navigation
type navigateToExploreMsg struct {
	note *model.Note
}

type navigateToDashboardMsg struct{}

type navigateToReviewMsg struct{}

type navigateToSearchMsg struct{}

type navigateToBibliographyMsg struct{}

var navCmd = &cobra.Command{
	Use:   "nav",
	Short: "Open the unified Zettelkasten Navigator",
	Run: func(cmd *cobra.Command, args []string) {
		runNavigator()
	},
}

func init() {
	rootCmd.AddCommand(navCmd)
}

type sessionState int

const (
	stateDashboard sessionState = iota
	stateExplore
	stateReview
	stateSearch
	stateBibliography
	stateWalkGraph
)

type navigatorModel struct {
	state sessionState
	store *store.Store
	root  string

	dashboard     dashboardModel
	explore       exploreModel
	review        reviewModel
	search        searchModel
	bibliography  bibliographyModel
	walkGraphView walkGraphModel

	width  int
	height int
}

func runNavigator() {
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

	// Initialize Dashboard data
	dash, err := NewDashboardModel(st, absRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing dashboard: %v\n", err)
		os.Exit(1)
	}

	m := navigatorModel{
		state:     stateDashboard,
		store:     st,
		root:      absRoot,
		dashboard: dash,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running navigator: %v\n", err)
		os.Exit(1)
	}
}

func (m navigatorModel) Init() tea.Cmd {
	return nil
}

func (m navigatorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Always update dashboard size as it persists
		var dashCmd tea.Cmd
		m.dashboard, dashCmd = updateDashboard(m.dashboard, msg)

		switch m.state {
		case stateDashboard:
			cmd = dashCmd
		case stateExplore:
			m.explore, cmd = updateExplore(m.explore, msg)
		case stateReview:
			m.review, cmd = updateReview(m.review, msg)
		case stateSearch:
			m.search, cmd = updateSearch(m.search, msg)
		case stateBibliography:
			m.bibliography, cmd = updateBibliography(m.bibliography, msg)
		case stateWalkGraph:
			m.walkGraphView, cmd = updateWalkGraph(m.walkGraphView, msg)
		}

		return m, cmd

	case navigateToDashboardMsg:
		m.state = stateDashboard
		return m, nil

	case navigateToSearchMsg:
		m.state = stateSearch
		// Always re-init search to refresh data? Or just once?
		// Re-init allows picking up new files
		m.search = newSearchModel(m.store, m.root)
		m.search, _ = updateSearch(m.search, tea.WindowSizeMsg{Width: m.width, Height: m.height})
		return m, nil

	case navigateToExploreMsg:
		m.state = stateExplore
		if msg.note != nil {
			// Initialize explore with note
			m.explore = initializeExploreModel(m.store, m.root, msg.note)
			// Resize immediately
			m.explore, _ = updateExplore(m.explore, tea.WindowSizeMsg{Width: m.width, Height: m.height})
		} else {
			// Random or default?
			// If msg.note is nil, maybe we just switch state if explore is already init?
			// Or fetch random
			if m.explore.current == nil {
				n, _ := m.store.GetRandomNote()
				if n != nil {
					m.explore = initializeExploreModel(m.store, m.root, n)
					m.explore, _ = updateExplore(m.explore, tea.WindowSizeMsg{Width: m.width, Height: m.height})
				}
			}
		}
		return m, nil

	case navigateToBibliographyMsg:
		m.state = stateBibliography
		m.bibliography = newBibliographyModel(m.store, m.root)
		m.bibliography, _ = updateBibliography(m.bibliography, tea.WindowSizeMsg{Width: m.width, Height: m.height})
		return m, nil

	case navigateToWalkGraphMsg:
		if m.explore.walk != nil {
			m.state = stateWalkGraph
			m.walkGraphView = newWalkGraphModel(m.explore.walk, m.store, m.root)
			m.walkGraphView, _ = updateWalkGraph(m.walkGraphView, tea.WindowSizeMsg{Width: m.width, Height: m.height})
		}
		return m, nil

	case navigateToExploreJumpMsg:
		if m.explore.walk != nil {
			noteID := m.explore.walk.jumpTo(msg.nodeID)
			if noteID != "" {
				n, err := m.store.GetNote(noteID)
				if err == nil {
					m.state = stateExplore
					m.explore, _ = updateExplore(m.explore, exploreJumpMsg{note: n})
				}
			}
		}
		return m, nil

	case navigateToReviewMsg:
		m.state = stateReview
		// Init review
		notes, _ := m.store.GetDueReviews()
		if len(notes) == 0 {
			// Fallback to stale notes logic?
			// Copied from review.go logic roughly
			// For now just pass empty list, reviewModel handles "No reviews"
		}
		m.review = newReviewModel(m.store, notes, m.root)
		// Resize
		m.review, _ = updateReview(m.review, tea.WindowSizeMsg{Width: m.width, Height: m.height})
		return m, nil
	}

	// Delegate to sub-models based on state
	switch m.state {
	case stateDashboard:
		m.dashboard, cmd = updateDashboard(m.dashboard, msg)
	case stateExplore:
		m.explore, cmd = updateExplore(m.explore, msg)
	case stateReview:
		m.review, cmd = updateReview(m.review, msg)
	case stateSearch:
		m.search, cmd = updateSearch(m.search, msg)
	case stateBibliography:
		m.bibliography, cmd = updateBibliography(m.bibliography, msg)
	case stateWalkGraph:
		m.walkGraphView, cmd = updateWalkGraph(m.walkGraphView, msg)
	}

	return m, cmd
}

func (m navigatorModel) View() string {
	switch m.state {
	case stateDashboard:
		return m.dashboard.View()
	case stateExplore:
		return m.explore.View()
	case stateReview:
		return m.review.View()
	case stateSearch:
		return m.search.View()
	case stateBibliography:
		return m.bibliography.View()
	case stateWalkGraph:
		return m.walkGraphView.View()
	}
	return ""
}

// Helpers
func updateDashboard(m dashboardModel, msg tea.Msg) (dashboardModel, tea.Cmd) {
	mod, cmd := m.Update(msg)
	return mod.(dashboardModel), cmd
}

func updateExplore(m exploreModel, msg tea.Msg) (exploreModel, tea.Cmd) {
	mod, cmd := m.Update(msg)
	return mod.(exploreModel), cmd
}

func updateReview(m reviewModel, msg tea.Msg) (reviewModel, tea.Cmd) {
	mod, cmd := m.Update(msg)
	return mod.(reviewModel), cmd
}

func updateSearch(m searchModel, msg tea.Msg) (searchModel, tea.Cmd) {
	mod, cmd := m.Update(msg)
	return mod.(searchModel), cmd
}

func updateBibliography(m bibliographyModel, msg tea.Msg) (bibliographyModel, tea.Cmd) {
	mod, cmd := m.Update(msg)
	return mod.(bibliographyModel), cmd
}

func updateWalkGraph(m walkGraphModel, msg tea.Msg) (walkGraphModel, tea.Cmd) {
	mod, cmd := m.Update(msg)
	return mod.(walkGraphModel), cmd
}
