package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/strangelove-ventures/heighliner/builder"
)

type GithubRelease struct {
	TagName string `json:"tag_name"`
}

func mostRecentReleasesForChain(
	chainNodeConfig builder.ChainNodeConfig,
	number int16,
) (builder.HeighlinerQueuedChainBuilds, error) {
	if chainNodeConfig.GithubOrganization == "" || chainNodeConfig.GithubRepo == "" {
		return builder.HeighlinerQueuedChainBuilds{}, fmt.Errorf("github organization: %s and/or repo: %s not provided for chain: %s\n", chainNodeConfig.GithubOrganization, chainNodeConfig.GithubRepo, chainNodeConfig.Name)
	}
	client := http.Client{Timeout: 5 * time.Second}

	if chainNodeConfig.RepoHost != "" && chainNodeConfig.RepoHost != "github.com" {
		return builder.HeighlinerQueuedChainBuilds{}, nil
	}

	fmt.Printf("Fetching most recent releases for github.com/%s/%s\n", chainNodeConfig.GithubOrganization, chainNodeConfig.GithubRepo)

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=%d&page=1",
		chainNodeConfig.GithubOrganization, chainNodeConfig.GithubRepo, number), http.NoBody)
	if err != nil {
		return builder.HeighlinerQueuedChainBuilds{}, fmt.Errorf("error building github releases request: %v", err)
	}

	res, err := client.Do(req)
	if err != nil {
		return builder.HeighlinerQueuedChainBuilds{}, fmt.Errorf("error performing github releases request: %v", err)
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return builder.HeighlinerQueuedChainBuilds{}, fmt.Errorf("error reading body from github releases request: %v", err)
	}

	releases := []GithubRelease{}
	err = json.Unmarshal(body, &releases)
	if err != nil {
		return builder.HeighlinerQueuedChainBuilds{}, fmt.Errorf("error parsing github releases response: %s, error: %v", body, err)
	}
	chainQueuedBuilds := builder.HeighlinerQueuedChainBuilds{}
	for i, release := range releases {
		fmt.Printf("Adding release tag to build queue: %s\n", release.TagName)
		chainQueuedBuilds.ChainConfigs = append(chainQueuedBuilds.ChainConfigs, builder.ChainNodeDockerBuildConfig{
			Build:  chainNodeConfig,
			Ref:    release.TagName,
			Latest: i == 0,
		})
	}

	return chainQueuedBuilds, nil
}

func queueAndBuild(
	buildConfig builder.HeighlinerDockerBuildConfig,
	chain string,
	org string,
	repo string,
	repoHost string,
	dockerfile string,
	buildDir string,
	preBuild string,
	buildTarget string,
	buildEnv string,
	binaries string,
	libraries string,
	ref string,
	tag string,
	latest bool,
	local bool,
	number int16,
	parallel int16,
) {
	heighlinerBuilder := builder.NewHeighlinerBuilder(buildConfig, parallel, local)

	for _, chainNodeConfig := range chains {
		// If chain is provided, only build images for that chain
		// Chain must be declared in chains.yaml
		if chain != "" && chainNodeConfig.Name != chain {
			continue
		}
		if org != "" {
			chainNodeConfig.GithubOrganization = org
		}
		if repo != "" {
			chainNodeConfig.GithubRepo = repo
		}
		if repoHost != "" {
			chainNodeConfig.RepoHost = repoHost
		}
		if buildTarget != "" {
			chainNodeConfig.BuildTarget = buildTarget
		}
		if buildEnv != "" {
			chainNodeConfig.BuildEnv = strings.Split(buildEnv, " ")
		}
		if binaries != "" {
			chainNodeConfig.Binaries = strings.Split(binaries, " ")
		}
		if libraries != "" {
			chainNodeConfig.Libraries = strings.Split(libraries, " ")
		}
		chainQueuedBuilds := builder.HeighlinerQueuedChainBuilds{ChainConfigs: []builder.ChainNodeDockerBuildConfig{}}
		if ref != "" || local {
			chainConfig := builder.ChainNodeDockerBuildConfig{
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
		heighlinerBuilder.AddToQueue(chainBuilds)
	}

	if heighlinerBuilder.QueueLen() == 0 {
		chainQueuedBuilds := builder.HeighlinerQueuedChainBuilds{ChainConfigs: []builder.ChainNodeDockerBuildConfig{}}
		chainConfig := builder.ChainNodeDockerBuildConfig{
			Build: builder.ChainNodeConfig{
				Name:               chain,
				RepoHost:           repoHost,
				GithubOrganization: org,
				GithubRepo:         repo,
				Dockerfile:         builder.DockerfileType(dockerfile),
				PreBuild:           preBuild,
				BuildTarget:        buildTarget,
				BuildEnv:           strings.Split(buildEnv, " "),
				BuildDir:           buildDir,
				Binaries:           strings.Split(binaries, " "),
				Libraries:          strings.Split(libraries, " "),
			},
			Ref:    ref,
			Tag:    tag,
			Latest: latest,
		}
		chainQueuedBuilds.ChainConfigs = append(chainQueuedBuilds.ChainConfigs, chainConfig)
		heighlinerBuilder.AddToQueue(chainQueuedBuilds)
	}

	heighlinerBuilder.BuildImages()
}
