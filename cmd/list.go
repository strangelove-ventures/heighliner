package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func ListCmd() *cobra.Command {
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List the docker images",
		Run: func(cmd *cobra.Command, args []string) {
			cmdFlags := cmd.Flags()

			configFile, _ := cmdFlags.GetString(flagFile)
			if configFile == "" {
				// try to load a local chains.yaml, but do not panic for any error, will fall back to embedded chains.
				cwd, err := os.Getwd()
				if err == nil {
					chainsYamlSearchPath := filepath.Join(cwd, "chains.yaml")
					if err := loadChainsYaml(chainsYamlSearchPath); err != nil {
						fmt.Printf("No config found at %s, using embedded chains. pass -f to configure chains.yaml path.\n", chainsYamlSearchPath)
					} else {
						fmt.Printf("Loaded chains from %s\n", chainsYamlSearchPath)
					}
				}
			} else {
				// if flag is explicitly provided, panic on error since intent was to override embedded chains.
				if err := loadChainsYaml(configFile); err != nil {
					panic(err)
				}
			}
			list()
		},
	}

	listCmd.PersistentFlags().StringP(flagFile, "f", "", "chains.yaml config file path (searches for chains.yaml in current directory by default)")

	return listCmd
}

func list() {
	requires := []string{
		"cosmos-sdk",
		"ibc-go",
	}
	for _, chain := range chains {
		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/go.mod",
			chain.GithubOrganization, chain.GithubRepo)
		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}
		mod, err := modfile.Parse("", body, nil)
		if err != nil {
			continue
		}
		fmt.Printf("\n%s/%s:\n", chain.GithubOrganization, chain.GithubRepo)
		found := 0
		for _, require := range mod.Require {
			for _, r := range requires {
				if strings.Contains(require.Mod.Path, r) {
					fmt.Printf("  %s@%s\n", r, require.Mod.Version)
					found += 1
				}
			}
		}
		if found == 0 {
			fmt.Printf("  no versions found\n")
		}
	}
}

func init() {
	rootCmd.AddCommand(ListCmd())
}
