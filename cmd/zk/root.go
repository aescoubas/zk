package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	rootDir string
)

var rootCmd = &cobra.Command{
	Use:   "zk",
	Short: "Zettelkasten CLI and TUI",
	Long:  `A comprehensive tool for managing your Zettelkasten knowledge base.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Parent() == nil {
			return nil
		}
		resolved, err := resolveRootDir(rootDir)
		if err != nil {
			return err
		}
		rootDir = resolved
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		var coded interface{ ExitCode() int }
		if errors.As(err, &coded) {
			if msg := strings.TrimSpace(err.Error()); msg != "" {
				fmt.Fprintln(os.Stderr, msg)
			}
			os.Exit(coded.ExitCode())
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&rootDir, "dir", "", "Root directory of the Zettelkasten data repo (defaults to ZK_PATH, then ~/.config/zk/root, then the current working directory)")
}
