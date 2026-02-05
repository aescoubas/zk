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
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/escoubas/zk/internal/llm"
	"github.com/escoubas/zk/internal/model"
	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Open the Zettelkasten interactive dashboard",
	Run: func(cmd *cobra.Command, args []string) {
		runNavigator()
	},
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}

// runDashboard removed, replaced by runNavigator in navigator.go

type dashboardModel struct {
	store       *store.Store
	root        string
	noteCount   int
	linkCount   int
	orphanCount int
	stubCount   int
	refCount     int
	refSummaries []model.RefSummary
	bibCursor    int
	allNotes     []*model.Note
	searchResults []model.SimilarNote
	topics      []model.TagCount
	snippet     string
	cursor      int
	offset      int
	quitting    bool
	width       int
	height      int
	
	// Semantic Search
	searchParams textinput.Model
	// FTS Search
	ftsParams    textinput.Model
	// Create Note Input
	createInput  textinput.Model
	
	// Navigation State
	activeColumn int // 0: Stats, 1: Middle, 2: Right
	middleFocus  int // 0: Semantic Input, 1: FTS Input
	mode         int // 0: Normal, 1: Insert, 2: DeleteConfirm, 3: CreateNote
	
	pendingDelete *model.Note // Note to be deleted

	// Search State
	lastSearchType  int    // focusSemantic or focusFTS
	lastSearchQuery string 

	isSearching  bool // Are we currently typing in the search bar? (Deprecated/Managed by mode)
	isResults    bool // Are we displaying search results instead of all notes?
	showHelp     bool // Show help overlay
	statusMsg    string // Status message (e.g. "Indexing...")
	llmClient    *llm.Client
}

func NewDashboardModel(st *store.Store, root string) (dashboardModel, error) {
	noteCount, linkCount, err := st.GetStats()
	if err != nil {
		return dashboardModel{}, err
	}

	orphans, _ := st.GetOrphanCount()
	stubs, _ := st.GetStubCount()
	refs, _ := st.GetRefCount()
	refSummaries, _ := st.ListRefSummaries()

	allNotes, err := st.ListNotes()
	if err != nil {
		return dashboardModel{}, err
	}

	topics, _ := st.GetTopTags(8)
	snippet := getRandomSnippet(root)

	// Init Semantic Input
	ti := textinput.New()
	ti.Placeholder = "Ask your Zettelkasten..."
	ti.CharLimit = 156
	ti.Width = 30
	ti.Prompt = "AI: "

	// Init FTS Input
	fts := textinput.New()
	fts.Placeholder = "Search keywords..."
	fts.CharLimit = 156
	fts.Width = 30
	fts.Prompt = "/: "

	// Init Create Input
	ci := textinput.New()
	ci.Placeholder = "Note Title"
	ci.CharLimit = 100
	ci.Width = 30
	ci.Prompt = "New: "

	return dashboardModel{
		store:        st,
		root:         root,
		noteCount:    noteCount,
		linkCount:    linkCount,
		orphanCount:  orphans,
		stubCount:    stubs,
		refCount:     refs,
		refSummaries: refSummaries,
		allNotes:     allNotes,
		topics:       topics,
		snippet:      snippet,
		cursor:       0,
		offset:       0,
		width:        80,
		height:       24,
		searchParams: ti,
		ftsParams:    fts,
		createInput:  ci,
		activeColumn: 2, // Default to Notes list (Right)
		middleFocus:  0, // Default to Semantic Input
		mode:         0, // Normal mode
		llmClient:    llm.NewClient("http://localhost:11434", "nomic-embed-text"),
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

type searchResultMsg []model.SimilarNote

func performSemanticSearch(client *llm.Client, store *store.Store, query string) tea.Cmd {
	return func() tea.Msg {
		vec, err := client.Embed(query)
		if err != nil {
			return searchResultMsg(nil)
		}
		results, err := store.SearchByVector(vec, 20)
		if err != nil {
			return searchResultMsg(nil)
		}
		return searchResultMsg(results)
	}
}

func performFullTextSearch(store *store.Store, query string) tea.Cmd {
	return func() tea.Msg {
		notes, err := store.SearchNotes(query)
		if err != nil {
			return searchResultMsg(nil)
		}
		// Convert to SimilarNote for compatibility
		results := make([]model.SimilarNote, len(notes))
		for i, n := range notes {
			results[i] = model.SimilarNote{
				Note: n,
				Score: 1.0, // Mock score for exact matches
			}
		}
		return searchResultMsg(results)
	}
}

const (
	modeNormal = 0
	modeInsert = 1
	modeDeleteConfirm = 2
	modeCreateNote = 3
	
	colStats  = 0
	colMiddle = 1
	colRight  = 2
	
	focusSemantic = 0
	focusFTS      = 1
)

func (m dashboardModel) Init() tea.Cmd {
	return nil
}

// New Msg type
type editorFinishedMsg struct {
	path string
	err  error
}

type indexingFinishedMsg struct {
	err error
}

type clearStatusMsg struct{}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case editorFinishedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Editor error: %v", msg.err)
			return m, nil
		}
		// Trigger indexing in background
		m.statusMsg = fmt.Sprintf("Indexing %s...", filepath.Base(msg.path))
		return m, func() tea.Msg {
			err := IndexAndEmbedNote(m.root, msg.path)
			return indexingFinishedMsg{err: err}
		}

	case indexingFinishedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Indexing failed: %v", msg.err)
		} else {
			m.statusMsg = "Indexing complete."
		}
		
		// Refresh list and stats
		notes, _ := m.store.ListNotes()
		m.allNotes = notes
		nc, lc, _ := m.store.GetStats()
		m.noteCount = nc
		m.linkCount = lc
		m.refCount, _ = m.store.GetRefCount()
		m.refSummaries, _ = m.store.ListRefSummaries()

		// If we are viewing results, refresh the search
		if m.isResults {
			if m.lastSearchType == focusSemantic {
				return m, performSemanticSearch(m.llmClient, m.store, m.lastSearchQuery)
			} else {
				return m, performFullTextSearch(m.store, m.lastSearchQuery)
			}
		}
		
		// Clear status after 3 seconds
		return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return clearStatusMsg{}
		})
	
	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil

	case tea.KeyMsg:
		// Delete Confirm Mode Logic
		if m.mode == modeDeleteConfirm {
			switch msg.String() {
			case "y", "Y":
				if m.pendingDelete != nil {
					// Physical delete
					fullPath := filepath.Join(m.root, m.pendingDelete.Path)
					if err := os.Remove(fullPath); err != nil {
						// TODO: Show error message?
					}
					// Index delete
					if err := m.store.DeleteNote(m.pendingDelete.ID); err == nil {
						// Refresh list
						notes, _ := m.store.ListNotes()
						m.allNotes = notes
						
						// Refresh stats
						nc, lc, _ := m.store.GetStats()
						m.noteCount = nc
						m.linkCount = lc
						
						// If we were in search results, re-run the search
						if m.isResults {
							m.mode = modeNormal
							m.pendingDelete = nil
							
							// Adjust cursor to avoid OOB if possible (heuristic)
							if m.cursor > 0 {
								m.cursor--
							}
							
							if m.lastSearchType == focusSemantic {
								return m, performSemanticSearch(m.llmClient, m.store, m.lastSearchQuery)
							} else {
								return m, performFullTextSearch(m.store, m.lastSearchQuery)
							}
						}

						// Reset search state to avoid stale results (only if NOT in results mode, which is handled above)
						// Actually if we were NOT in results mode, we just stay in all notes.
						// But if we were, we re-ran search.
						
						// Adjust cursor for All Notes view
						if m.cursor >= len(m.allNotes) && m.cursor > 0 {
							m.cursor--
						}
					}
				}
				m.mode = modeNormal
				m.pendingDelete = nil
				return m, nil
			case "n", "N", "esc":
				m.mode = modeNormal
				m.pendingDelete = nil
				return m, nil
			}
			return m, nil
		}

		// Create Note Mode Logic
		if m.mode == modeCreateNote {
			switch {
			case msg.Type == tea.KeyEnter:
				title := m.createInput.Value()
				if title != "" {
					path, err := createNoteFile(title)
					if err == nil {
						m.mode = modeNormal
						m.createInput.SetValue("")
						relPath, _ := filepath.Rel(m.root, path)
						// Refresh list (optional, but good)
						// notes, _ := m.store.ListNotes()
						// m.allNotes = notes
						return m, openEditor(m.root, relPath)
					}
				}
				m.mode = modeNormal
				m.createInput.Blur()
				return m, nil
			case msg.Type == tea.KeyEsc:
				m.mode = modeNormal
				m.createInput.Blur()
				return m, nil
			case msg.Type == tea.KeyCtrlC:
				m.quitting = true
				return m, tea.Quit
			}
			m.createInput, cmd = m.createInput.Update(msg)
			return m, cmd
		}

		// Insert Mode Logic
		if m.mode == modeInsert {
			switch {
			case msg.Type == tea.KeyEnter:
				m.mode = modeNormal
				m.isResults = true
				var query string
				if m.middleFocus == focusSemantic {
					query = m.searchParams.Value()
					m.lastSearchType = focusSemantic
					m.lastSearchQuery = query
					m.searchParams.Blur()
					return m, performSemanticSearch(m.llmClient, m.store, query)
				} else {
					query = m.ftsParams.Value()
					m.lastSearchType = focusFTS
					m.lastSearchQuery = query
					m.ftsParams.Blur()
					return m, performFullTextSearch(m.store, query)
				}
			case msg.Type == tea.KeyEsc:
				m.mode = modeNormal
				m.searchParams.Blur()
				m.ftsParams.Blur()
				return m, nil
			case msg.Type == tea.KeyCtrlC:
				m.quitting = true
				return m, tea.Quit
			}
			
			if m.middleFocus == focusSemantic {
				m.searchParams, cmd = m.searchParams.Update(msg)
			} else {
				m.ftsParams, cmd = m.ftsParams.Update(msg)
			}
			return m, cmd
		}

		// Normal Mode Logic
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		
		case "n":
			m.mode = modeCreateNote
			m.createInput.Focus()
			return m, textinput.Blink

		// Column Navigation
		case "h", "left":
			if m.activeColumn > colStats {
				m.activeColumn--
			}
		case "l", "right":
			if m.activeColumn < colRight {
				m.activeColumn++
			}
			
		// Vertical Navigation within Columns
		case "j", "down":
			if m.activeColumn == colStats {
				if m.bibCursor < len(m.refSummaries)-1 {
					m.bibCursor++
				}
			} else if m.activeColumn == colMiddle {
				if m.middleFocus < focusFTS {
					m.middleFocus++
				}
			} else if m.activeColumn == colRight {
				// Navigate List
				listLen := len(m.allNotes)
				if m.isResults {
					listLen = len(m.searchResults)
				}
				
				if m.cursor < listLen-1 {
					m.cursor++
					listHeight := m.height - 14
					if listHeight < 1 { listHeight = 1 }
					if m.cursor >= m.offset+listHeight {
						m.offset++
					}
				}
			}
		case "k", "up":
			if m.activeColumn == colStats {
				if m.bibCursor > 0 {
					m.bibCursor--
				}
			} else if m.activeColumn == colMiddle {
				if m.middleFocus > focusSemantic {
					m.middleFocus--
				}
			} else if m.activeColumn == colRight {
				// Navigate List
				if m.cursor > 0 {
					m.cursor--
					if m.cursor < m.offset {
						m.offset = m.cursor
					}
				}
			}

		// Enter Insert Mode
		case "i", "enter":
			if m.activeColumn == colStats {
				if len(m.refSummaries) > 0 && m.bibCursor < len(m.refSummaries) {
					return m, func() tea.Msg { return navigateToBibliographyMsg{} }
				}
			} else if m.activeColumn == colMiddle {
				m.mode = modeInsert
				if m.middleFocus == focusSemantic {
					m.searchParams.Focus()
				} else {
					m.ftsParams.Focus()
				}
				return m, textinput.Blink
			} else if m.activeColumn == colRight {
				// Open Note
				if m.isResults {
					if len(m.searchResults) > 0 {
						note := m.searchResults[m.cursor].Note
						return m, func() tea.Msg { return navigateToExploreMsg{note: note} }
					}
				} else {
					if len(m.allNotes) > 0 {
						note := m.allNotes[m.cursor]
						return m, func() tea.Msg { return navigateToExploreMsg{note: note} }
					}
				}
			}

		case "esc":
			if m.isResults {
				m.isResults = false
				m.searchParams.SetValue("")
				m.ftsParams.SetValue("")
				m.offset = 0
				m.cursor = 0
			}
		
		// Delete Note
		case "d", "delete":
			if m.activeColumn == colRight {
				if m.isResults {
					if len(m.searchResults) > 0 {
						m.pendingDelete = m.searchResults[m.cursor].Note
						m.mode = modeDeleteConfirm
					}
				} else {
					if len(m.allNotes) > 0 {
						m.pendingDelete = m.allNotes[m.cursor]
						m.mode = modeDeleteConfirm
					}
				}
			}

		// Legacy shortcuts (still useful?)
		case "r":
			n, err := m.store.GetRandomNote()
			if err == nil {
				return m, func() tea.Msg { return navigateToExploreMsg{note: n} }
			}
		case "e":
			if m.activeColumn == colRight {
				var note *model.Note
				if m.isResults {
					if len(m.searchResults) > 0 {
						note = m.searchResults[m.cursor].Note
					}
				} else {
					if len(m.allNotes) > 0 {
						note = m.allNotes[m.cursor]
					}
				}
				
				if note != nil {
					return m, openEditor(m.root, note.Path)
				}
			}
		case "s":
			return m, func() tea.Msg { return navigateToReviewMsg{} }
		case "b":
			return m, func() tea.Msg { return navigateToBibliographyMsg{} }
		}
	case searchResultMsg:
		m.searchResults = msg
		m.isResults = true
		m.activeColumn = colRight // Auto switch to results
		m.cursor = 0
		m.offset = 0
		return m, nil
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
	totalOverhead := 14
	colWidth := (m.width - totalOverhead) / 3
	if colWidth < 20 {
		colWidth = 20
	}

	// Styles
	baseBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(colWidth).
		Height(m.height - 10)

	activeBorderColor := lipgloss.Color(GruvboxOrangeBright)
	inactiveBorderColor := lipgloss.Color(GruvboxBlue)

	statsBoxStyle := baseBoxStyle.Copy().BorderForeground(inactiveBorderColor)
	middleBoxStyle := baseBoxStyle.Copy().BorderForeground(inactiveBorderColor)
	notesBoxStyle := baseBoxStyle.Copy().BorderForeground(inactiveBorderColor)

	if m.activeColumn == colStats {
		statsBoxStyle = statsBoxStyle.BorderForeground(activeBorderColor)
	} else if m.activeColumn == colMiddle {
		middleBoxStyle = middleBoxStyle.BorderForeground(activeBorderColor)
	} else if m.activeColumn == colRight {
		notesBoxStyle = notesBoxStyle.BorderForeground(activeBorderColor)
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(GruvboxYellowBright)).
		Bold(true).
		MarginBottom(1).
		Width(m.width).
		Align(lipgloss.Center)

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
		Width(m.width - 4).
		Align(lipgloss.Center)

	// Layout
	title := titleStyle.Render("Zettelkasten Dashboard")
	snippet := snippetStyle.Render(fmt.Sprintf("\"%s\"", m.snippet))

	// Stats Column
	statsContent := ""
	statsContent += fmt.Sprintf("%s %s\n", statLabel.Render("Notes:"), statValue.Render(fmt.Sprint(m.noteCount)))
	statsContent += fmt.Sprintf("%s %s\n", statLabel.Render("Links:"), statValue.Render(fmt.Sprint(m.linkCount)))
	statsContent += fmt.Sprintf("%s %s\n", statLabel.Render("Orphans:"), statValue.Render(fmt.Sprint(m.orphanCount)))
	statsContent += fmt.Sprintf("%s %s\n", statLabel.Render("Stubs:"), statValue.Render(fmt.Sprint(m.stubCount)))
	statsContent += fmt.Sprintf("%s %s\n", statLabel.Render("Refs:"), statValue.Render(fmt.Sprint(m.refCount)))

	// Bibliography section
	bibHeader := headerStyle.Render("Bibliography")
	bibContent := ""
	if len(m.refSummaries) == 0 {
		bibContent = lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxGray)).Render("No refs yet")
	} else {
		bibItemStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxFg))
		bibSelectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxOrangeBright)).SetString("> ")
		bibCountStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxPurpleBright))
		maxBibItems := m.height - 22
		if maxBibItems < 3 {
			maxBibItems = 3
		}
		shown := len(m.refSummaries)
		if shown > maxBibItems {
			shown = maxBibItems
		}
		for i := 0; i < shown; i++ {
			rs := m.refSummaries[i]
			line := fmt.Sprintf("%s %s", bibCountStyle.Render(fmt.Sprintf("[%d]", rs.Citations)), rs.Ref.Title)
			avail := colWidth - 6
			if len(line) > avail && avail > 5 {
				line = line[:avail-1] + "…"
			}
			if m.activeColumn == colStats && i == m.bibCursor {
				bibContent += bibSelectedStyle.Render(line) + "\n"
			} else {
				bibContent += bibItemStyle.Render("  "+line) + "\n"
			}
		}
		if len(m.refSummaries) > maxBibItems {
			bibContent += lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxGray)).Render(
				fmt.Sprintf("  … +%d more", len(m.refSummaries)-maxBibItems))
		}
	}

	statsBox := statsBoxStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		headerStyle.Render("Stats"), statsContent, "\n", bibHeader, bibContent))

	// Topics Column
	topicsContent := ""
	for _, t := range m.topics {
		topicsContent += fmt.Sprintf("%s (%d)\n", t.Tag, t.Count)
	}
	if len(m.topics) == 0 {
		topicsContent = "No tags found"
	}
	
	// Input Styles
	activeInputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxOrangeBright))
	inactiveInputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxGray))
	
	semanticHeader := "Semantic Search"
	ftsHeader := "Full Text Search"
	
	if m.activeColumn == colMiddle {
		if m.middleFocus == focusSemantic {
			semanticHeader = activeInputStyle.Render("> " + semanticHeader)
			ftsHeader = inactiveInputStyle.Render("  " + ftsHeader)
		} else {
			semanticHeader = inactiveInputStyle.Render("  " + semanticHeader)
			ftsHeader = activeInputStyle.Render("> " + ftsHeader)
		}
	} else {
		semanticHeader = inactiveInputStyle.Render("  " + semanticHeader)
		ftsHeader = inactiveInputStyle.Render("  " + ftsHeader)
	}

	searchBar := m.searchParams.View()
	ftsBar := m.ftsParams.View()
	
	topicsBox := middleBoxStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left, 
			headerStyle.Render("Top Topics"), 
			topicsContent,
			"\n",
			semanticHeader,
			searchBar,
			"\n",
			ftsHeader,
			ftsBar,
		),
	)

	// All Notes / Results Column
	allNotesContent := ""
	listHeight := m.height - 14
	if listHeight < 1 {
		listHeight = 1
	}
	
	listLen := 0
	headerTitle := "All Notes"
	if m.isResults {
		listLen = len(m.searchResults)
		headerTitle = fmt.Sprintf("Results (%d)", listLen)
	} else {
		listLen = len(m.allNotes)
	}

	start := m.offset
	end := m.offset + listHeight
	if end > listLen {
		end = listLen
	}

	for i := start; i < end; i++ {
		var title, summary string
		if m.isResults {
			n := m.searchResults[i].Note
			title = n.Title
			// Use TrimSpace first to remove trailing/leading newlines, then ReplaceAll for internal ones
			cleanSummary := strings.ReplaceAll(strings.TrimSpace(n.Summary), "\n", " ")
			summary = fmt.Sprintf("[%.2f] %s", m.searchResults[i].Score, cleanSummary)
		} else {
			n := m.allNotes[i]
			title = n.Title
			cleanSummary := strings.ReplaceAll(strings.TrimSpace(n.Summary), "\n", " ")
			summary = cleanSummary
		}
		
		// Build line with Title and Summary
		avail := colWidth - 4 // Padding/Border safety
		if avail < 10 {
			avail = 10
		}
		
		line := title
		if len(title) < avail && summary != "" {
			rem := avail - len(title) - 3
			if rem > 5 {
				if len(summary) > rem {
					summary = summary[:rem-1] + "…"
				}
				line = fmt.Sprintf("%s · %s", title, summary)
			}
		}

		if len(line) > avail {
			line = line[:avail-1] + "…"
		}

		if m.activeColumn == colRight && i == m.cursor {
			allNotesContent += selectedStyle.Render(line) + "\n"
		} else {
			allNotesContent += itemStyle.Render(line) + "\n"
		}
	}
	recentsBox := notesBoxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, headerStyle.Render(headerTitle), allNotesContent))

	// Combine Columns
	columns := lipgloss.JoinHorizontal(lipgloss.Center, statsBox, topicsBox, recentsBox)
	columns = lipgloss.PlaceHorizontal(m.width, lipgloss.Center, columns)

	// Mode Status
	modeStr := "NORMAL"
	modeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxGreenBright)).Bold(true)
	if m.mode == modeInsert {
		modeStr = "INSERT"
		modeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxRedBright)).Bold(true)
	} else if m.mode == modeDeleteConfirm {
		modeStr = "CONFIRM DELETE"
		modeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxRedBright)).Bold(true).Blink(true)
	} else if m.mode == modeCreateNote {
		modeStr = "CREATE NOTE"
		modeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxBlueBright)).Bold(true)
	}

	status := ""
	if m.statusMsg != "" {
		status = lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxPurple)).Italic(true).Render(m.statusMsg)
	}

	// Help
	helpText := fmt.Sprintf("[%s] h/l: Nav Cols | j/k: Nav Items | i: Insert | Enter: Open/Search | ?: Help %s", modeStyle.Render(modeStr), status)
	if m.mode == modeDeleteConfirm {
		helpText = fmt.Sprintf("[%s] Are you sure you want to delete '%s'? (y/n) %s", modeStyle.Render(modeStr), m.pendingDelete.Title, status)
	} else if m.mode == modeCreateNote {
		helpText = fmt.Sprintf("[%s] %s %s", modeStyle.Render(modeStr), m.createInput.View(), status)
	}
	help := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxGray)).Width(m.width).Align(lipgloss.Center).Render(helpText)

	// Full View
	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		snippet,
		"\n",
		columns,
		"\n",
		help,
	)
	
	if m.showHelp {
		helpBoxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(GruvboxYellow)).
			Padding(1, 2).
			Width(60)
		
		helpContent := `
      Available Commands

      Navigation
        h/l, Left/Right   Navigate Columns
        j/k, Down/Up      Navigate Items
        Tab               Cycle Focus
      
      Actions
        i, Enter          Insert Mode / Open Note
        e                 Edit Selected Note
        d                 Delete Selected Note
        n                 New Note
        s                 Review Mode (SRS)
        b                 Bibliography
        r                 Random Note
        ?                 Toggle Help
        q, Ctrl+C         Quit
		`
		box := helpBoxStyle.Render(helpContent)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}

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
		return editorFinishedMsg{path: path, err: err}
	})
}