package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
		return builder.HeighlinerQueuedChainBuilds{}, fmt.Errorf("github organization: %s and/or repo: %s not provided for chain: %s", chainNodeConfig.GithubOrganization, chainNodeConfig.GithubRepo, chainNodeConfig.Name)
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

	githubUser, githubPAT := os.Getenv("GH_USER"), os.Getenv("GH_PAT")
	if githubUser != "" && githubPAT != "" {
		req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(githubUser+":"+githubPAT)))
	}

	res, err := client.Do(req)
	if err != nil {
		return builder.HeighlinerQueuedChainBuilds{}, fmt.Errorf("error performing github releases request: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		return builder.HeighlinerQueuedChainBuilds{}, fmt.Errorf("status code: %v", res.StatusCode)
	}

	defer func() { _ = res.Body.Close() }()

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
	chainConfig chainConfigFlags,
) {
	heighlinerBuilder := builder.NewHeighlinerBuilder(buildConfig, chainConfig.parallel, chainConfig.local, chainConfig.race)

	for _, chainNodeConfig := range chains {
		// If chain is provided, only build images for that chain
		// Chain must be declared in chains.yaml
		if chainConfig.chain != "" && chainNodeConfig.Name != chainConfig.chain {
			continue
		}
		chainNodeConfig = applyChainOverrides(chainNodeConfig, chainConfig)
		chainQueuedBuilds := builder.HeighlinerQueuedChainBuilds{ChainConfigs: []builder.ChainNodeDockerBuildConfig{}}
		if chainConfig.ref != "" || chainConfig.local {
			chainConfig := builder.ChainNodeDockerBuildConfig{
				Build:  chainNodeConfig,
				Ref:    chainConfig.ref,
				Tag:    chainConfig.tag,
				Latest: chainConfig.latest,
			}
			chainQueuedBuilds.ChainConfigs = append(chainQueuedBuilds.ChainConfigs, chainConfig)
			heighlinerBuilder.AddToQueue(chainQueuedBuilds)
			heighlinerBuilder.BuildImages()
			return
		}
		// If specific version not provided, build images for the last n releases from the chain
		chainBuilds, err := mostRecentReleasesForChain(chainNodeConfig, chainConfig.number)
		if err != nil {
			fmt.Printf("Error queueing docker image builds for chain %s: %v", chainNodeConfig.Name, err)
			continue
		}
		heighlinerBuilder.AddToQueue(chainBuilds)
	}

	if heighlinerBuilder.QueueLen() == 0 {
		chainQueuedBuilds := builder.HeighlinerQueuedChainBuilds{ChainConfigs: []builder.ChainNodeDockerBuildConfig{}}
		chainConfig := builder.ChainNodeDockerBuildConfig{
			Build: applyChainOverrides(builder.ChainNodeConfig{
				Name:     chainConfig.chain,
				PreBuild: chainConfig.preBuildOverride,
				BuildDir: chainConfig.buildDirOverride,
			}, chainConfig),
			Ref:    chainConfig.ref,
			Tag:    chainConfig.tag,
			Latest: chainConfig.latest,
		}
		chainQueuedBuilds.ChainConfigs = append(chainQueuedBuilds.ChainConfigs, chainConfig)
		heighlinerBuilder.AddToQueue(chainQueuedBuilds)
	}

	heighlinerBuilder.BuildImages()
}

func applyChainOverrides(chain builder.ChainNodeConfig, flags chainConfigFlags) builder.ChainNodeConfig {
	if flags.orgOverride != "" {
		chain.GithubOrganization = flags.orgOverride
	}
	if flags.repoOverride != "" {
		chain.GithubRepo = flags.repoOverride
	}
	if flags.repoHostOverride != "" {
		chain.RepoHost = flags.repoHostOverride
	}
	if flags.cloneKeyOverride != "" {
		chain.CloneKey = flags.cloneKeyOverride
	}
	if flags.dockerfileOverride != "" {
		chain.Dockerfile = builder.DockerfileType(flags.dockerfileOverride)
	}
	if flags.buildTargetOverride != "" {
		chain.BuildTarget = flags.buildTargetOverride
	}
	if flags.buildEnvOverride != "" {
		chain.BuildEnv = strings.Split(flags.buildEnvOverride, " ")
	}
	if flags.binariesOverride != "" {
		chain.Binaries = strings.Split(flags.binariesOverride, " ")
	}
	if flags.librariesOverride != "" {
		chain.Libraries = strings.Split(flags.librariesOverride, " ")
	}
	return chain
}
