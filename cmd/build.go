package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/strangelove-ventures/heighliner/docker"
	"gopkg.in/yaml.v2"
)

type ChainNodeConfig struct {
	Name               string   `yaml:"name"`
	RepoHost           string   `yaml:"repo-host"`
	GithubOrganization string   `yaml:"github-organization"`
	GithubRepo         string   `yaml:"github-repo"`
	Language           string   `yaml:"language"`
	BuildTarget        string   `yaml:"build-target"`
	BuildDir           string   `yaml:"build-dir"`
	Binaries           []string `yaml:"binaries"`
	Libraries          []string `yaml:"libraries"`
	PreBuild           string   `yaml:"pre-build"`
	Platforms          []string `yaml:"platforms"`
	BuildEnv           []string `yaml:"build-env"`
	BaseImage          string   `yaml:"base-image"`
}

type GithubRelease struct {
	TagName string `json:"tag_name"`
}

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if c := s[len(s)-1]; s[0] == c && (c == '"' || c == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

type ChainNodeDockerBuildConfig struct {
	Build   ChainNodeConfig
	Version string
	Latest  bool
}

type HeighlinerDockerBuildConfig struct {
	ContainerRegistry string
	SkipPush          bool
	UseBuildKit       bool
	BuildKitAddr      string
	Platform          string
	NoCache           bool
}

type HeighlinerQueuedChainBuilds struct {
	ChainConfigs []ChainNodeDockerBuildConfig
}

func buildChainNodeDockerImage(
	buildConfig *HeighlinerDockerBuildConfig,
	chainConfig *ChainNodeDockerBuildConfig,
) error {
	var dockerfile string
	var imageTag string
	switch chainConfig.Build.Language {
	case "imported":
		dockerfile = "./dockerfile/imported"
		imageTag = strings.ReplaceAll(chainConfig.Version, "/", "-")
	case "rust":
		dockerfile = "./dockerfile/rust"
		imageTag = strings.ReplaceAll(chainConfig.Version, "/", "-")
	case "nix":
		dockerfile = "./dockerfile/nix"
		imageTag = strings.ReplaceAll(chainConfig.Version, "/", "-")
	case "go":
		dockerfile = "./dockerfile/sdk"
		imageTag = strings.ReplaceAll(chainConfig.Version, "/", "-")

	default:
		dockerfile = "./dockerfile/none"
		imageTag = strings.ReplaceAll(chainConfig.Version, "/", "-")
	}

	var imageName string
	if buildConfig.ContainerRegistry == "" {
		imageName = chainConfig.Build.Name
	} else {
		imageName = fmt.Sprintf("%s/%s", buildConfig.ContainerRegistry, chainConfig.Build.Name)
	}

	imageTags := []string{fmt.Sprintf("%s:%s", imageName, imageTag)}
	if chainConfig.Latest {
		imageTags = append(imageTags, fmt.Sprintf("%s:latest", imageName))
	}

	fmt.Printf("Image Tags: +%v\n", imageTags)

	buildEnv := ""

	buildTagsEnvVar := ""
	for _, envVar := range chainConfig.Build.BuildEnv {
		envVarSplit := strings.Split(envVar, "=")
		if envVarSplit[0] == "BUILD_TAGS" {
			buildTagsEnvVar = envVar
		} else {
			buildEnv += envVar + " "
		}
	}

	binaries := strings.Join(chainConfig.Build.Binaries, " ")
	libraries := strings.Join(chainConfig.Build.Libraries, " ")

	repoHost := chainConfig.Build.RepoHost
	if repoHost == "" {
		repoHost = "github.com"
	}

	buildArgs := map[string]string{
		"VERSION":             chainConfig.Version,
		"NAME":                chainConfig.Build.Name,
		"BASE_IMAGE":          chainConfig.Build.BaseImage,
		"REPO_HOST":           repoHost,
		"GITHUB_ORGANIZATION": chainConfig.Build.GithubOrganization,
		"GITHUB_REPO":         chainConfig.Build.GithubRepo,
		"BUILD_TARGET":        chainConfig.Build.BuildTarget,
		"BINARIES":            binaries,
		"LIBRARIES":           libraries,
		"PRE_BUILD":           chainConfig.Build.PreBuild,
		"BUILD_ENV":           buildEnv,
		"BUILD_TAGS":          buildTagsEnvVar,
		"BUILD_DIR":           chainConfig.Build.BuildDir,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Minute*180))
	defer cancel()

	push := buildConfig.ContainerRegistry != "" && !buildConfig.SkipPush

	if buildConfig.UseBuildKit {
		buildKitOptions := docker.GetDefaultBuildKitOptions()
		buildKitOptions.Address = buildConfig.BuildKitAddr
		supportedPlatforms := chainConfig.Build.Platforms

		if len(supportedPlatforms) > 0 {
			platforms := []string{}
			requestedPlatforms := strings.Split(buildConfig.Platform, ",")
			for _, supportedPlatform := range supportedPlatforms {
				for _, requestedPlatform := range requestedPlatforms {
					if supportedPlatform == requestedPlatform {
						platforms = append(platforms, requestedPlatform)
					}
				}
			}
			if len(platforms) == 0 {
				return fmt.Errorf("no requested platforms are supported for this chain: %s. requested: %s, supported: %s", chainConfig.Build.Name, buildConfig.Platform, strings.Join(supportedPlatforms, ","))
			}
			buildKitOptions.Platform = strings.Join(platforms, ",")
		} else {
			buildKitOptions.Platform = buildConfig.Platform
		}
		buildKitOptions.NoCache = buildConfig.NoCache
		return docker.BuildDockerImageWithBuildKit(ctx, dockerfile, imageTags, push, buildArgs, buildKitOptions)
	} else {
		return docker.BuildDockerImage(ctx, dockerfile, imageTags, push, buildArgs, buildConfig.NoCache)
	}
}

func queueMostRecentReleasesForChain(
	chainQueuedBuilds *HeighlinerQueuedChainBuilds,
	chainNodeConfig ChainNodeConfig,
	number int16,
) error {
	if chainNodeConfig.GithubOrganization == "" || chainNodeConfig.GithubRepo == "" {
		return fmt.Errorf("github organization: %s and/or repo: %s not provided for chain: %s\n", chainNodeConfig.GithubOrganization, chainNodeConfig.GithubRepo, chainNodeConfig.Name)
	}
	client := http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=%d&page=1",
		chainNodeConfig.GithubOrganization, chainNodeConfig.GithubRepo, number), http.NoBody)
	if err != nil {
		return fmt.Errorf("error building github releases request: %v", err)
	}

	basicAuthUser := os.Getenv("GITHUB_USER")
	basicAuthPassword := os.Getenv("GITHUB_PASSWORD")

	req.SetBasicAuth(basicAuthUser, basicAuthPassword)

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error performing github releases request: %v", err)
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error reading body from github releases request: %v", err)
	}

	releases := []GithubRelease{}
	err = json.Unmarshal(body, &releases)
	if err != nil {
		return fmt.Errorf("error parsing github releases response: %s, error: %v", body, err)
	}
	for i, release := range releases {
		chainQueuedBuilds.ChainConfigs = append(chainQueuedBuilds.ChainConfigs, ChainNodeDockerBuildConfig{
			Build:   chainNodeConfig,
			Version: release.TagName,
			Latest:  i == 0,
		})
	}
	return nil
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the docker images",
	Long: `By default, fetch the last 5 releases in the repositories specified in chains.yaml.
For each tag that doesn't exist in the specified container repository,
it will be built and pushed`,
	Run: func(cmd *cobra.Command, args []string) {
		containerRegistry, _ := cmd.Flags().GetString("registry")
		fmt.Printf("Container registry: %s\n", containerRegistry)

		cmdFlags := cmd.Flags()

		chain, _ := cmdFlags.GetString("chain")
		version, _ := cmdFlags.GetString("version")
		number, _ := cmdFlags.GetInt16("number")
		skip, _ := cmdFlags.GetBool("skip")

		useBuildKit, _ := cmdFlags.GetBool("use-buildkit")
		buildKitAddr, _ := cmdFlags.GetString("buildkit-addr")
		platform, _ := cmdFlags.GetString("platform")
		noCache, _ := cmdFlags.GetBool("no-cache")
		latest, _ := cmdFlags.GetBool("latest")
		parallel, _ := cmdFlags.GetInt16("parallel")

		// Parse chains.yaml
		dat, err := os.ReadFile("./chains.yaml")
		if err != nil {
			log.Fatalf("Error reading chains.yaml: %v", err)
		}
		chains := []ChainNodeConfig{}
		err = yaml.Unmarshal(dat, &chains)
		if err != nil {
			log.Fatalf("Error parsing chains.yaml: %v", err)
		}

		buildQueue := []*HeighlinerQueuedChainBuilds{}
		buildConfig := HeighlinerDockerBuildConfig{
			ContainerRegistry: containerRegistry,
			SkipPush:          skip,
			UseBuildKit:       useBuildKit,
			BuildKitAddr:      buildKitAddr,
			Platform:          platform,
			NoCache:           noCache,
		}

		for _, chainNodeConfig := range chains {
			// If chain is provided, only build images for that chain
			// Chain must be declared in chains.yaml
			if chain != "" && chainNodeConfig.Name != chain {
				continue
			}
			chainQueuedBuilds := HeighlinerQueuedChainBuilds{ChainConfigs: []ChainNodeDockerBuildConfig{}}
			if version != "" {
				chainConfig := ChainNodeDockerBuildConfig{
					Build:   chainNodeConfig,
					Version: version,
					Latest:  latest,
				}
				chainQueuedBuilds.ChainConfigs = append(chainQueuedBuilds.ChainConfigs, chainConfig)
				buildQueue = append(buildQueue, &chainQueuedBuilds)
				buildImages(&buildConfig, buildQueue, parallel)
				return
			}
			// If specific version not provided, build images for the last n releases from the chain
			err := queueMostRecentReleasesForChain(&chainQueuedBuilds, chainNodeConfig, number)
			if err != nil {
				log.Printf("Error queueing docker image builds for chain %s: %v", chainNodeConfig.Name, err)
				continue
			}
			if len(chainQueuedBuilds.ChainConfigs) > 0 {
				buildQueue = append(buildQueue, &chainQueuedBuilds)
			}
		}
		buildImages(&buildConfig, buildQueue, parallel)
	},
}

// returns queue items, starting with latest for each chain
func getQueueItem(queue []*HeighlinerQueuedChainBuilds, index int) *ChainNodeDockerBuildConfig {
	j := 0
	for i := 0; true; i++ {
		foundForThisIndex := false
		for _, queuedChainBuilds := range queue {
			if i < len(queuedChainBuilds.ChainConfigs) {
				if j == index {
					return &queuedChainBuilds.ChainConfigs[i]
				}
				j++
				foundForThisIndex = true
			}
		}
		if !foundForThisIndex {
			// all done
			return nil
		}
	}
	return nil
}

func buildNextImage(buildConfig *HeighlinerDockerBuildConfig, queue []*HeighlinerQueuedChainBuilds, buildIndex *int, buildIndexLock *sync.Mutex, wg *sync.WaitGroup, errors *[]error, errorsLock *sync.Mutex) {
	buildIndexLock.Lock()
	defer buildIndexLock.Unlock()
	chainConfig := getQueueItem(queue, *buildIndex)
	*buildIndex++
	if chainConfig == nil {
		wg.Done()
		return
	}
	go func() {
		log.Printf("Building docker image: %s:%s\n", chainConfig.Build.Name, chainConfig.Version)
		if err := buildChainNodeDockerImage(buildConfig, chainConfig); err != nil {
			errorsLock.Lock()
			*errors = append(*errors, fmt.Errorf("error building docker image for %s:%s - %v\n", chainConfig.Build.Name, chainConfig.Version, err))
			errorsLock.Unlock()
		}
		buildNextImage(buildConfig, queue, buildIndex, buildIndexLock, wg, errors, errorsLock)
	}()

}

func buildImages(buildConfig *HeighlinerDockerBuildConfig, queue []*HeighlinerQueuedChainBuilds, parallel int16) {
	buildIndex := 0
	buildIndexLock := sync.Mutex{}
	errors := []error{}
	errorsLock := sync.Mutex{}

	wg := sync.WaitGroup{}
	for i := int16(0); i < parallel; i++ {
		wg.Add(1)
		buildNextImage(buildConfig, queue, &buildIndex, &buildIndexLock, &wg, &errors, &errorsLock)
	}
	wg.Wait()
	if len(errors) > 0 {
		for _, err := range errors {
			log.Println(err)
		}
		panic("Some images failed to build")
	}
}

func init() {
	rootCmd.AddCommand(buildCmd)

	buildCmd.PersistentFlags().StringP("registry", "r", "", "Docker Container Registry for pushing images")
	buildCmd.PersistentFlags().StringP("chain", "c", "", "Cosmos chain to build from chains.yaml")
	buildCmd.PersistentFlags().StringP("version", "v", "", "Github tag to build")
	buildCmd.PersistentFlags().Int16P("number", "n", 5, "Number of releases to build per chain")
	buildCmd.PersistentFlags().Int16("parallel", 1, "Number of docker builds to run simultaneously")
	buildCmd.PersistentFlags().BoolP("skip", "s", false, "Skip pushing images to registry")
	buildCmd.PersistentFlags().BoolP("latest", "l", false, "Also push latest tag (for single version build only)")

	buildCmd.PersistentFlags().BoolP("use-buildkit", "b", false, "Use buildkit to build multi-arch images")
	buildCmd.PersistentFlags().String("buildkit-addr", docker.BuildKitSock, "Address of the buildkit socket, can be unix, tcp, ssl")
	buildCmd.PersistentFlags().StringP("platform", "p", docker.DefaultPlatforms, "Platforms to build")
	buildCmd.PersistentFlags().Bool("no-cache", false, "Don't use docker cache for building")
}
