package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	rootDir string
)

var rootCmd = &cobra.Command{
	Use:   "zk",
	Short: "Zettelkasten CLI tool",
	Long:  `A comprehensive tool for managing your Zettelkasten knowledge base.`,
	Run: func(cmd *cobra.Command, args []string) {
		runNavigator()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flag for root directory
	defaultDir := "."
	if envDir := os.Getenv("ZK_PATH"); envDir != "" {
		defaultDir = envDir
	}
	rootCmd.PersistentFlags().StringVar(&rootDir, "dir", defaultDir, "Root directory of the Zettelkasten")
}
