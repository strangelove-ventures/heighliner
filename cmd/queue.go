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
	chainConfig chainConfigFlags,
) {
	heighlinerBuilder := builder.NewHeighlinerBuilder(buildConfig, chainConfig.parallel, chainConfig.local, chainConfig.race)

	for _, chainNodeConfig := range chains {
		// If chain is provided, only build images for that chain
		// Chain must be declared in chains.yaml
		if chainConfig.chain != "" && chainNodeConfig.Name != chainConfig.chain {
			continue
		}
		if chainConfig.orgOverride != "" {
			chainNodeConfig.GithubOrganization = chainConfig.orgOverride
		}
		if chainConfig.repoOverride != "" {
			chainNodeConfig.GithubRepo = chainConfig.repoOverride
		}
		if chainConfig.repoHostOverride != "" {
			chainNodeConfig.RepoHost = chainConfig.repoHostOverride
		}
		if chainConfig.cloneKeyOverride != "" {
			chainNodeConfig.CloneKey = chainConfig.cloneKeyOverride
		}
		if chainConfig.buildTargetOverride != "" {
			chainNodeConfig.BuildTarget = chainConfig.buildTargetOverride
		}
		if chainConfig.buildEnvOverride != "" {
			chainNodeConfig.BuildEnv = strings.Split(chainConfig.buildEnvOverride, " ")
		}
		if chainConfig.binariesOverride != "" {
			chainNodeConfig.Binaries = strings.Split(chainConfig.binariesOverride, " ")
		}
		if chainConfig.librariesOverride != "" {
			chainNodeConfig.Libraries = strings.Split(chainConfig.librariesOverride, " ")
		}
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
			Build: builder.ChainNodeConfig{
				Name:               chainConfig.chain,
				RepoHost:           chainConfig.repoHostOverride,
				GithubOrganization: chainConfig.orgOverride,
				GithubRepo:         chainConfig.repoOverride,
				CloneKey:           chainConfig.cloneKeyOverride,
				Dockerfile:         builder.DockerfileType(chainConfig.dockerfileOverride),
				PreBuild:           chainConfig.preBuildOverride,
				BuildTarget:        chainConfig.buildTargetOverride,
				BuildEnv:           strings.Split(chainConfig.buildEnvOverride, " "),
				BuildDir:           chainConfig.buildDirOverride,
				Binaries:           strings.Split(chainConfig.binariesOverride, " "),
				Libraries:          strings.Split(chainConfig.librariesOverride, " "),
			},
			Ref:    chainConfig.ref,
			Tag:    chainConfig.tag,
			Latest: chainConfig.latest,
		}
		chainQueuedBuilds.ChainConfigs = append(chainQueuedBuilds.ChainConfigs, chainConfig)
		heighlinerBuilder.AddToQueue(chainQueuedBuilds)
	}

	heighlinerBuilder.BuildImages()
}
