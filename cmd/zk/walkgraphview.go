package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/escoubas/zk/internal/model"
	"github.com/escoubas/zk/internal/store"
)

// Navigation messages for the walk graph view.
type navigateToWalkGraphMsg struct{}

type navigateToExploreJumpMsg struct {
	nodeID int
}

type exploreJumpMsg struct {
	note *model.Note
}

// treeLine is one rendered row in the tree display.
type treeLine struct {
	node   *walkNode
	prefix string // box-drawing indentation (│ ├── └── etc.)
}

// walkGraphModel is the TUI view for the navigation walk graph.
type walkGraphModel struct {
	graph  *walkGraph
	store  *store.Store
	root   string
	lines  []treeLine
	cursor int
	offset int
	width  int
	height int
}

func newWalkGraphModel(graph *walkGraph, st *store.Store, root string) walkGraphModel {
	m := walkGraphModel{
		graph: graph,
		store: st,
		root:  root,
	}
	m.lines = buildTreeLines(graph)

	// Position cursor on the current walk-graph node.
	for i, line := range m.lines {
		if line.node == graph.current {
			m.cursor = i
			break
		}
	}

	return m
}

// buildTreeLines produces a depth-first list of tree lines with
// box-drawing prefixes, similar to the `tree` command.
func buildTreeLines(graph *walkGraph) []treeLine {
	var lines []treeLine
	buildTreeLinesRecursive(graph.root, "", true, &lines)
	return lines
}

func buildTreeLinesRecursive(node *walkNode, indent string, isLast bool, lines *[]treeLine) {
	prefix := ""
	nextIndent := ""

	if node.parent == nil {
		// root — no connector
		prefix = ""
		nextIndent = ""
	} else if isLast {
		prefix = indent + "└── "
		nextIndent = indent + "    "
	} else {
		prefix = indent + "├── "
		nextIndent = indent + "│   "
	}

	*lines = append(*lines, treeLine{node: node, prefix: prefix})

	for i, child := range node.children {
		buildTreeLinesRecursive(child, nextIndent, i == len(node.children)-1, lines)
	}
}

// Truncate title to maxLen runes, appending "..." if needed.
func truncateTitle(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

// Edge indicator symbols.
func edgeIndicator(edge edgeLabel) string {
	switch edge {
	case edgeRoot:
		return "●"
	case edgeOutgoing:
		return "→"
	case edgeBacklink:
		return "←"
	case edgeSimilar:
		return "~"
	case edgeCitation:
		return "@"
	default:
		return "·"
	}
}

// Styled edge indicator using Gruvbox palette.
func styledEdgeIndicator(edge edgeLabel) string {
	switch edge {
	case edgeRoot:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxYellowBright)).Render("●")
	case edgeOutgoing:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxBlue)).Render("→")
	case edgeBacklink:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxAqua)).Render("←")
	case edgeSimilar:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxPurple)).Render("~")
	case edgeCitation:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxPurpleBright)).Render("@")
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxGray)).Render("·")
	}
}

func (m walkGraphModel) Init() tea.Cmd { return nil }

func (m walkGraphModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, func() tea.Msg { return navigateToExploreMsg{note: nil} }
		case "j", "down":
			if m.cursor < len(m.lines)-1 {
				m.cursor++
				m.ensureVisible()
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}
		case "enter":
			if m.cursor >= 0 && m.cursor < len(m.lines) {
				nodeID := m.lines[m.cursor].node.id
				return m, func() tea.Msg { return navigateToExploreJumpMsg{nodeID: nodeID} }
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureVisible()
	}
	return m, nil
}

func (m *walkGraphModel) ensureVisible() {
	vh := m.viewHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vh {
		m.offset = m.cursor - vh + 1
	}
}

func (m walkGraphModel) viewHeight() int {
	// height minus: title(1) + blank(1) + stats(1) + help(1) + blank(1) = 5
	h := m.height - 5
	if h < 1 {
		h = 1
	}
	return h
}

func (m walkGraphModel) View() string {
	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(GruvboxYellowBright)).
		Width(m.width).
		Align(lipgloss.Center)
	title := titleStyle.Render("Walk Graph")

	// Visible slice of tree lines
	vh := m.viewHeight()
	start := m.offset
	end := start + vh
	if end > len(m.lines) {
		end = len(m.lines)
	}

	// Styles
	prefixStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxGray))
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxFg))
	selectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxOrangeBright)).Bold(true)
	currentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxGreenBright)).Bold(true)
	markerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxOrangeBright))
	posStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxGreenBright))

	var content strings.Builder
	for i := start; i < end; i++ {
		line := m.lines[i]
		node := line.node
		isCurrent := node == m.graph.current
		isSelected := i == m.cursor

		// Cursor marker
		marker := "  "
		if isSelected {
			marker = markerStyle.Render("▸ ")
		}

		// Prefix (box-drawing)
		prefix := prefixStyle.Render(line.prefix)

		// Edge indicator
		indicator := styledEdgeIndicator(node.edge)

		// Title
		titleText := truncateTitle(node.title, 30)
		var styledTitle string
		if isSelected {
			styledTitle = selectedStyle.Render(titleText)
		} else if isCurrent {
			styledTitle = currentStyle.Render(titleText)
		} else {
			styledTitle = normalStyle.Render(titleText)
		}

		// Current position marker
		posMarker := ""
		if isCurrent {
			posMarker = posStyle.Render(" ◀")
		}

		content.WriteString(marker + prefix + indicator + " " + styledTitle + posMarker + "\n")
	}

	// Stats footer
	statsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(GruvboxAqua)).
		Width(m.width).
		Align(lipgloss.Center)
	stats := fmt.Sprintf("Nodes: %d │ Max Depth: %d │ Branches: %d",
		m.graph.nodeTotal(), m.graph.maxDepth(), m.graph.branchCount())

	// Help footer
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(GruvboxGray)).
		Width(m.width).
		Align(lipgloss.Center)
	helpText := "[j/k] Navigate | [Enter] Jump to Note | [Esc] Return to Explore"

	return lipgloss.JoinVertical(lipgloss.Left,
		title, "\n", content.String(), statsStyle.Render(stats), helpStyle.Render(helpText))
}
