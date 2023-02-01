package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/strangelove-ventures/heighliner/build"
	"gopkg.in/yaml.v2"
)

var rootCmd = &cobra.Command{
	Use:   "heighliner",
	Short: "Generate docker images for Cosmos chains",
	Long: `Welcome to Heighliner, provided by Strangelove Ventures.

This tool can generate docker images for all different release versions
of the configured Cosmos blockchains in chains.yaml`,
}

var chains []build.ChainNodeConfig

func Execute(chainsYaml []byte) {
	err := yaml.Unmarshal(chainsYaml, &chains)
	if err != nil {
		panic(fmt.Errorf("error parsing chains.yaml: %v", err))
	}

	err = rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {}
