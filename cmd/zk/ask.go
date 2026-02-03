package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/escoubas/zk/internal/llm"
	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

var askCmd = &cobra.Command{
	Use:   "ask [query]",
	Short: "Semantic search using vector embeddings",
	Long:  `Search your notes by meaning rather than just keywords. Requires Ollama to be running.`, 
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := args[0]
		runAsk(query)
	},
}

func init() {
	rootCmd.AddCommand(askCmd)
}

func runAsk(query string) {
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

	// Initialize LLM Client (reuse defaults or flags if we moved them to root)
	// For now hardcoding default or we could expose flags on askCmd too
	client := llm.NewClient("http://localhost:11434", "nomic-embed-text")

	fmt.Printf("Thinking about \"%s\"...\n", query)
	
	// 1. Embed the query
	vec, err := client.Embed(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error embedding query (is Ollama running?): %v\n", err)
		os.Exit(1)
	}

	// 2. Search
	results, err := st.SearchByVector(vec, 10)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error searching: %v\n", err)
		os.Exit(1)
	}

	if len(results) == 0 {
		fmt.Println("No semantically similar notes found.")
		return
	}

	// 3. Render Results
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxYellowBright)).Bold(true)
	scoreStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxGray)).Italic(true)
	summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(GruvboxFg))

	fmt.Println("")
	for _, match := range results {
		score := fmt.Sprintf("%.2f", match.Score)
		fmt.Printf("%s %s\n", titleStyle.Render(match.Note.Title), scoreStyle.Render(score))
		if match.Note.Summary != "" {
			fmt.Printf("  %s\n", summaryStyle.Render(match.Note.Summary))
		}
		fmt.Println("")
	}
}
