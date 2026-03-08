package cmd

import (
	"github.com/spf13/cobra"
)

var vaultName string

var rootCmd = &cobra.Command{
	Use:     "notesmd-cli",
	Short:   "A CLI for interacting with your Obsidian vault",
	Version: "v0.3.0",
}

func Execute() error {
	return rootCmd.Execute()
}
