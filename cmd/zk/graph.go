package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed graph_template.html
var graphTemplateHTML string

type graphNode struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Group int    `json:"group"`
}

type graphLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Value  int    `json:"value"`
}

type graphData struct {
	Nodes []graphNode `json:"nodes"`
	Links []graphLink `json:"links"`
}

var graphOutputPath string

var graphCmd = &cobra.Command{
	Use:           "graph",
	Short:         "Generate a standalone HTML graph for zettels",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGraph()
	},
}

func init() {
	rootCmd.AddCommand(graphCmd)
	graphCmd.Flags().StringVarP(&graphOutputPath, "output", "o", "", "Output path for the generated graph HTML (defaults to <root>/graph.html)")
}

func runGraph() error {
	notes, err := loadZettels(rootDir)
	if err != nil {
		return err
	}

	nodes := make([]graphNode, 0, len(notes))
	noteIDs := make(map[string]struct{}, len(notes))
	for _, note := range notes {
		nodes = append(nodes, graphNode{
			ID:    note.ID,
			Title: note.Title,
			Group: 1,
		})
		noteIDs[note.ID] = struct{}{}
	}

	links := make([]graphLink, 0)
	for _, note := range notes {
		for _, link := range note.OutgoingLinks {
			target := strings.TrimSpace(link.TargetID)
			if _, ok := noteIDs[target]; !ok {
				continue
			}
			links = append(links, graphLink{
				Source: note.ID,
				Target: target,
				Value:  1,
			})
		}
	}

	payload := graphData{
		Nodes: nodes,
		Links: links,
	}

	jsonData, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal graph data: %w", err)
	}

	outputPath := graphOutputPath
	if outputPath == "" {
		outputPath = filepath.Join(rootDir, "graph.html")
	}
	if !filepath.IsAbs(outputPath) {
		outputPath, err = filepath.Abs(outputPath)
		if err != nil {
			return fmt.Errorf("resolve output path: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	html := strings.Replace(graphTemplateHTML, "{{GRAPH_DATA}}", string(jsonData), 1)
	if err := os.WriteFile(outputPath, []byte(html), 0644); err != nil {
		return fmt.Errorf("write graph HTML: %w", err)
	}

	fmt.Printf("Graph built: %d nodes, %d links. Saved to %s\n", len(nodes), len(links), outputPath)
	return nil
}
