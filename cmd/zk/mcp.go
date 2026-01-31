package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/escoubas/zk/internal/mcp"
	"github.com/escoubas/zk/internal/store"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the MCP (Model Context Protocol) Server",
	Run: func(cmd *cobra.Command, args []string) {
		runMCP()
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

func runMCP() {
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

	server := mcp.NewServer(st, absRoot)
	if err := server.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "MCP Server error: %v\n", err)
		os.Exit(1)
	}
}

