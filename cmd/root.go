package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "heighliner",
	Short: "Generate docker images for Cosmos chains",
	Long: `Welcome to Heighliner, provided by Strangelove Ventures.

This tool can generate docker images for all different release versions
of the configured Cosmos blockchains in chains.yaml`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {}
