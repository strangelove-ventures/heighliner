package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
)

func loadLocalChainsYaml() error {
	// try to load a local chains.yaml, but do not panic for any error, will fall back to embedded chains.
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	chainsYamlSearchPath := filepath.Join(cwd, "chains.yaml")
	err = loadChainsYaml(chainsYamlSearchPath)
	if err != nil {
		return fmt.Errorf("No config found at %s, using embedded chains. pass -f to configure chains.yaml path.\n", chainsYamlSearchPath)
	}
	fmt.Printf("Loaded chains from %s\n", chainsYamlSearchPath)
	return nil
}

func ListCmd() *cobra.Command {
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List the docker images",
		Run: func(cmd *cobra.Command, args []string) {
			cmdFlags := cmd.Flags()

			configFile, _ := cmdFlags.GetString(flagFile)
			if configFile == "" {
				if err := loadLocalChainsYaml(); err != nil {
					fmt.Println(err)
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

func printError(err error) {
	fmt.Printf("  error: %s\n", err)
}

func list() {
	requires := []string{
		"cosmos-sdk",
		"ibc-go",
	}
	for _, chain := range chains {
		fmt.Printf("\n%s:\n", chain.Name)
		if chain.GithubOrganization == "" || chain.GithubRepo == "" {
			printError(fmt.Errorf("not enough repo info"))
			continue
		}
		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/go.mod",
			chain.GithubOrganization, chain.GithubRepo)
		resp, err := http.Get(url)
		if err != nil {
			printError(err)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			printError(fmt.Errorf(resp.Status))
			continue
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			printError(err)
			continue
		}
		mod, err := modfile.Parse("", body, nil)
		if err != nil {
			printError(err)
			continue
		}
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
			printError(fmt.Errorf("no versions found"))
		}
	}
}
