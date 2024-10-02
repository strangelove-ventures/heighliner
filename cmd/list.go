package cmd

import (
	"crypto/tls"
	"fmt"
	"github.com/hashicorp/go-version"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	chainsYamlSearchPath := filepath.Join(cwd, "chains/")
	err = loadChainsYaml(chainsYamlSearchPath)
	if err != nil {
		fmt.Printf("No config found at %s, using embedded chains. pass -f to configure chains.yaml path.\n", chainsYamlSearchPath)
		return nil
	}
	fmt.Printf("Loaded chains from %s\n", chainsYamlSearchPath)
	return nil
}

func ListCmd() *cobra.Command {
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List the docker images. Currently only supports cosmos-based images.",
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
	stats := map[string]map[string]int{}
	for _, r := range requires {
		stats[r] = map[string]int{}
	}
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	for _, chain := range chains {
		fmt.Printf("\n%s:\n", chain.Name)
		if chain.GithubOrganization == "" || chain.GithubRepo == "" {
			printError(fmt.Errorf("not enough repo info; missing organization or repo"))
			continue
		}
		url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/go.mod",
			chain.GithubOrganization, chain.GithubRepo)
		resp, err := http.Get(url)
		if err != nil {
			printError(fmt.Errorf("GET %s: %w", url, err))
			continue
		}
		if resp.StatusCode != http.StatusOK {
			printError(fmt.Errorf("GET %s: %s", url, resp.Status))
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
			printError(fmt.Errorf("parsing go.mod: %w", err))
			continue
		}
		newStats := 0
		for _, require := range mod.Require {
			for _, r := range requires {
				if strings.Contains(require.Mod.Path, r) {
					fmt.Printf("  %s@%s\n", r, require.Mod.Version)
					v, err := version.NewVersion(require.Mod.Version)
					if err != nil {
						printError(fmt.Errorf("parsing module version: %w", err))
						continue
					}
					segments := v.Segments()
					majorVersion := strconv.Itoa(segments[0]) + "." + strconv.Itoa(segments[1])
					if _, found := stats[r][majorVersion]; !found {
						stats[r][majorVersion] = 0
					}
					stats[r][majorVersion]++
					newStats++
				}
			}
		}
		if newStats == 0 {
			printError(fmt.Errorf("no versions found"))
		}
	}
	fmt.Printf("\nSummary:\n")
	for _, r := range requires {
		fmt.Printf("\n  %s versions:\n", r)
		total := 0
		versions := make([]string, 0, len(stats[r]))
		for v := range stats[r] {
			versions = append(versions, v)
		}
		sort.Strings(versions)
		for _, version := range versions {
			count := stats[r][version]
			fmt.Printf("    %s (%d)\n", version, count)
			total += count
		}
		fmt.Printf("    total: %d chains\n", total)
	}
}
