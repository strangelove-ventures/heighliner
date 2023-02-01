package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/strangelove-ventures/heighliner/build"
)

type GithubRelease struct {
	TagName string `json:"tag_name"`
}

func mostRecentReleasesForChain(
	chainNodeConfig build.ChainNodeConfig,
	number int16,
) (build.HeighlinerQueuedChainBuilds, error) {
	if chainNodeConfig.GithubOrganization == "" || chainNodeConfig.GithubRepo == "" {
		return build.HeighlinerQueuedChainBuilds{}, fmt.Errorf("github organization: %s and/or repo: %s not provided for chain: %s\n", chainNodeConfig.GithubOrganization, chainNodeConfig.GithubRepo, chainNodeConfig.Name)
	}
	client := http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=%d&page=1",
		chainNodeConfig.GithubOrganization, chainNodeConfig.GithubRepo, number), http.NoBody)
	if err != nil {
		return build.HeighlinerQueuedChainBuilds{}, fmt.Errorf("error building github releases request: %v", err)
	}

	basicAuthUser := os.Getenv("GITHUB_USER")
	basicAuthPassword := os.Getenv("GITHUB_PASSWORD")

	req.SetBasicAuth(basicAuthUser, basicAuthPassword)

	res, err := client.Do(req)
	if err != nil {
		return build.HeighlinerQueuedChainBuilds{}, fmt.Errorf("error performing github releases request: %v", err)
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return build.HeighlinerQueuedChainBuilds{}, fmt.Errorf("error reading body from github releases request: %v", err)
	}

	releases := []GithubRelease{}
	err = json.Unmarshal(body, &releases)
	if err != nil {
		return build.HeighlinerQueuedChainBuilds{}, fmt.Errorf("error parsing github releases response: %s, error: %v", body, err)
	}
	chainQueuedBuilds := build.HeighlinerQueuedChainBuilds{ChainConfigs: []build.ChainNodeDockerBuildConfig{}}
	for i, release := range releases {
		chainQueuedBuilds.ChainConfigs = append(chainQueuedBuilds.ChainConfigs, build.ChainNodeDockerBuildConfig{
			Build:  chainNodeConfig,
			Ref:    release.TagName,
			Latest: i == 0,
		})
	}

	return chainQueuedBuilds, nil
}

func queueAndBuild(
	buildConfig build.HeighlinerDockerBuildConfig,
	chain string,
	org string,
	ref string,
	tag string,
	latest bool,
	local bool,
	number int16,
	parallel int16,
) {
	heighlinerBuilder := build.NewHeighlinerBuilder(buildConfig, parallel, local)

	for _, chainNodeConfig := range chains {
		// If chain is provided, only build images for that chain
		// Chain must be declared in chains.yaml
		if chain != "" && chainNodeConfig.Name != chain {
			continue
		}
		if org != "" {
			chainNodeConfig.GithubOrganization = org
		}
		chainQueuedBuilds := build.HeighlinerQueuedChainBuilds{ChainConfigs: []build.ChainNodeDockerBuildConfig{}}
		if ref != "" || local {
			chainConfig := build.ChainNodeDockerBuildConfig{
				Build:  chainNodeConfig,
				Ref:    ref,
				Tag:    tag,
				Latest: latest,
			}
			chainQueuedBuilds.ChainConfigs = append(chainQueuedBuilds.ChainConfigs, chainConfig)
			heighlinerBuilder.AddToQueue(chainQueuedBuilds)
			heighlinerBuilder.BuildImages()
			return
		}
		// If specific version not provided, build images for the last n releases from the chain
		chainBuilds, err := mostRecentReleasesForChain(chainNodeConfig, number)
		if err != nil {
			fmt.Printf("Error queueing docker image builds for chain %s: %v", chainNodeConfig.Name, err)
			continue
		}
		if len(chainQueuedBuilds.ChainConfigs) > 0 {
			heighlinerBuilder.AddToQueue(chainBuilds)
		}
	}
	heighlinerBuilder.BuildImages()
}
