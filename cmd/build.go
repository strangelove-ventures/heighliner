package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/strangelove-ventures/heighliner/docker"
	"github.com/strangelove-ventures/heighliner/dockerfile"
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
	NoBuildCache      bool
}

type HeighlinerQueuedChainBuilds struct {
	ChainConfigs []ChainNodeDockerBuildConfig
}

// tagFromVersion returns a sanitized docker image tag from a version string.
func tagFromVersion(version string) string {
	return strings.ReplaceAll(version, "/", "-")
}

// getDockerfile attempts to find Dockerfile within current working directory.
// Returns embedded Dockerfile if local file is not found or cannot be read.
func getDockerfile(dockerfile string, embedded []byte) []byte {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Using embedded %s due to working directory not found\n", dockerfile)
		return embedded
	}

	absDockerfile := filepath.Join(cwd, "dockerfile", dockerfile)
	if _, err := os.Stat(absDockerfile); err != nil {
		fmt.Printf("Using embedded %s due to local dockerfile not found\n", dockerfile)
		return embedded
	}

	df, err := os.ReadFile(absDockerfile)
	if err != nil {
		fmt.Printf("Using embedded %s due to failure to read local dockerfile\n", dockerfile)
		return embedded
	}

	fmt.Printf("Using local %s", dockerfile)
	return df
}

// dockerfileAndTag returns the appropriate dockerfile as bytes and the docker image tag
// based on the input configuration.
func dockerfileAndTag(
	buildConfig *HeighlinerDockerBuildConfig,
	chainConfig *ChainNodeDockerBuildConfig,
	local bool,
) ([]byte, string) {
	switch chainConfig.Build.Language {
	case "imported":
		return getDockerfile("imported/Dockerfile", dockerfile.Imported), tagFromVersion(chainConfig.Version)
	case "rust":
		return getDockerfile("rust/Dockerfile", dockerfile.Rust), tagFromVersion(chainConfig.Version)
	case "nix":
		if buildConfig.UseBuildKit {
			return getDockerfile("nix/Dockerfile", dockerfile.Nix), tagFromVersion(chainConfig.Version)
		}
		return getDockerfile("nix/native.Dockerfile", dockerfile.NixNative), tagFromVersion(chainConfig.Version)
	case "go":
		if local {
			// local builds always use embedded Dockerfile.
			if chainConfig.Version == "" {
				return dockerfile.SDKLocal, "local"
			}
			return dockerfile.SDKLocal, tagFromVersion(chainConfig.Version)
		} else {
			if buildConfig.UseBuildKit {
				return getDockerfile("sdk/Dockerfile", dockerfile.SDK), tagFromVersion(chainConfig.Version)
			}
			return getDockerfile("sdk/native.Dockerfile", dockerfile.SDKNative), tagFromVersion(chainConfig.Version)
		}
	default:
		return getDockerfile("none/Dockerfile", dockerfile.None), tagFromVersion(chainConfig.Version)
	}
}

// buildChainNodeDockerImage builds the requested chain node docker image
// based on the input configuration.
func buildChainNodeDockerImage(
	buildConfig *HeighlinerDockerBuildConfig,
	chainConfig *ChainNodeDockerBuildConfig,
	local bool,
	queueTmpDirRemoval func(tmpDir string, start bool),
) error {
	df, imageTag := dockerfileAndTag(buildConfig, chainConfig, local)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting working directory: %w", err)
	}

	dir, err := os.MkdirTemp(cwd, "heighliner")
	if err != nil {
		return fmt.Errorf("error making temporary directory for dockerfile: %w", err)
	}

	// queue removal on ctrl+c
	queueTmpDirRemoval(dir, true)
	defer func() {
		// this build is done, so don't need removal on ctrl+c anymore since we are removing now.
		queueTmpDirRemoval(dir, false)
		_ = os.RemoveAll(dir)
	}()

	reldir, err := filepath.Rel(cwd, dir)
	if err != nil {
		return fmt.Errorf("error finding relative path for dockerfile working directory: %w", err)
	}

	dfilepath := filepath.Join(reldir, "Dockerfile")
	if err := os.WriteFile(dfilepath, df, 0644); err != nil {
		return fmt.Errorf("error writing temporary dockerfile: %w", err)
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

	buildTimestamp := ""
	if buildConfig.NoBuildCache {
		buildTimestamp = strconv.FormatInt(time.Now().Unix(), 10)
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
		"BUILD_TIMESTAMP":     buildTimestamp,
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
		if err := docker.BuildDockerImageWithBuildKit(ctx, reldir, imageTags, push, buildArgs, buildKitOptions); err != nil {
			return err
		}
	} else {
		if err := docker.BuildDockerImage(ctx, dfilepath, imageTags, push, buildArgs, buildConfig.NoCache); err != nil {
			return err
		}
	}
	return nil
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

func loadChainsYaml(configFile string) error {
	if _, err := os.Stat(configFile); err != nil {
		return fmt.Errorf("error checking for file: %s: %w", configFile, err)
	}
	bz, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("error reading file: %s: %w", configFile, err)
	}
	var newChains []ChainNodeConfig
	err = yaml.Unmarshal(bz, &newChains)
	if err != nil {
		return fmt.Errorf("error unmarshalling yaml from file: %s: %w", configFile, err)
	}
	chains = newChains
	return nil
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the docker images",
	Long: `By default, fetch the last 5 releases in the repositories specified in chains.yaml.
For each tag that doesn't exist in the specified container repository,
it will be built and pushed`,
	Run: func(cmd *cobra.Command, args []string) {
		cmdFlags := cmd.Flags()

		configFile, _ := cmdFlags.GetString("file")
		if configFile == "" {
			// try to load a local chains.yaml, but do not panic for any error, will fall back to embedded chains.
			cwd, err := os.Getwd()
			if err == nil {
				chainsYamlSearchPath := filepath.Join(cwd, "chains.yaml")
				if err := loadChainsYaml(chainsYamlSearchPath); err != nil {
					fmt.Printf("No config found at %s, using embedded chains. pass -f to configure config.yaml path.\n", chainsYamlSearchPath)
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

		containerRegistry, _ := cmdFlags.GetString("registry")
		chain, _ := cmdFlags.GetString("chain")
		version, _ := cmdFlags.GetString("version")
		org, _ := cmdFlags.GetString("org")
		number, _ := cmdFlags.GetInt16("number")
		skip, _ := cmdFlags.GetBool("skip")

		useBuildKit, _ := cmdFlags.GetBool("use-buildkit")
		buildKitAddr, _ := cmdFlags.GetString("buildkit-addr")
		platform, _ := cmdFlags.GetString("platform")
		noCache, _ := cmdFlags.GetBool("no-cache")
		noBuildCache, _ := cmdFlags.GetBool("no-build-cache")
		latest, _ := cmdFlags.GetBool("latest")
		local, _ := cmdFlags.GetBool("local")
		parallel, _ := cmdFlags.GetInt16("parallel")

		buildQueue := []*HeighlinerQueuedChainBuilds{}
		buildConfig := HeighlinerDockerBuildConfig{
			ContainerRegistry: containerRegistry,
			SkipPush:          skip,
			UseBuildKit:       useBuildKit,
			BuildKitAddr:      buildKitAddr,
			Platform:          platform,
			NoCache:           noCache,
			NoBuildCache:      noBuildCache,
		}

		for _, chainNodeConfig := range chains {
			// If chain is provided, only build images for that chain
			// Chain must be declared in chains.yaml
			if chain != "" && chainNodeConfig.Name != chain {
				continue
			}
			if org != "" {
				chainNodeConfig.GithubOrganization = org
			}
			chainQueuedBuilds := HeighlinerQueuedChainBuilds{ChainConfigs: []ChainNodeDockerBuildConfig{}}
			if version != "" || local {
				chainConfig := ChainNodeDockerBuildConfig{
					Build:   chainNodeConfig,
					Version: version,
					Latest:  latest,
				}
				chainQueuedBuilds.ChainConfigs = append(chainQueuedBuilds.ChainConfigs, chainConfig)
				buildQueue = append(buildQueue, &chainQueuedBuilds)
				buildImages(&buildConfig, buildQueue, parallel, local)
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
		buildImages(&buildConfig, buildQueue, parallel, false)
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

func buildNextImage(
	buildConfig *HeighlinerDockerBuildConfig,
	queue []*HeighlinerQueuedChainBuilds,
	buildIndex *int,
	buildIndexLock *sync.Mutex,
	wg *sync.WaitGroup,
	errors *[]error,
	errorsLock *sync.Mutex,
	local bool,
	queueTmpDirRemoval func(tmpDir string, start bool),
) {
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
		if err := buildChainNodeDockerImage(buildConfig, chainConfig, local, queueTmpDirRemoval); err != nil {
			errorsLock.Lock()
			*errors = append(*errors, fmt.Errorf("error building docker image for %s:%s - %v\n", chainConfig.Build.Name, chainConfig.Version, err))
			errorsLock.Unlock()
		}
		buildNextImage(buildConfig, queue, buildIndex, buildIndexLock, wg, errors, errorsLock, local, queueTmpDirRemoval)
	}()
}

func buildImages(buildConfig *HeighlinerDockerBuildConfig, queue []*HeighlinerQueuedChainBuilds, parallel int16, local bool) {
	buildIndex := 0
	buildIndexLock := sync.Mutex{}
	errors := []error{}
	errorsLock := sync.Mutex{}

	tmpDirsToRemove := make(map[string]bool)
	var tmpDirMapMu sync.Mutex
	queueTmpDirRemoval := func(tmpDir string, start bool) {
		tmpDirMapMu.Lock()
		defer tmpDirMapMu.Unlock()
		if start {
			tmpDirsToRemove[tmpDir] = true
		} else {
			delete(tmpDirsToRemove, tmpDir)
		}
	}

	// Delete tmp dirs on ctrl+c
	c := make(chan os.Signal)
	//nolint:govet
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		tmpDirMapMu.Lock()
		defer tmpDirMapMu.Unlock()
		for dir := range tmpDirsToRemove {
			_ = os.RemoveAll(dir)
		}

		os.Exit(1)
	}()

	wg := sync.WaitGroup{}
	for i := int16(0); i < parallel; i++ {
		wg.Add(1)
		buildNextImage(buildConfig, queue, &buildIndex, &buildIndexLock, &wg, &errors, &errorsLock, local, queueTmpDirRemoval)
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

	buildCmd.PersistentFlags().StringP("file", "f", "", "chains.yaml config file path")
	buildCmd.PersistentFlags().StringP("registry", "r", "", "Docker Container Registry for pushing images")
	buildCmd.PersistentFlags().StringP("chain", "c", "", "Cosmos chain to build from chains.yaml")
	buildCmd.PersistentFlags().StringP("org", "o", "", "Github organization override for building from a fork")
	buildCmd.PersistentFlags().StringP("version", "v", "", "Github tag to build")
	buildCmd.PersistentFlags().Int16P("number", "n", 5, "Number of releases to build per chain")
	buildCmd.PersistentFlags().Int16("parallel", 1, "Number of docker builds to run simultaneously")
	buildCmd.PersistentFlags().BoolP("skip", "s", false, "Skip pushing images to registry")
	buildCmd.PersistentFlags().BoolP("latest", "l", false, "Also push latest tag (for single version build only)")
	buildCmd.PersistentFlags().Bool("local", false, "Use local directory (not git repository)")

	buildCmd.PersistentFlags().BoolP("use-buildkit", "b", false, "Use buildkit to build multi-arch images")
	buildCmd.PersistentFlags().String("buildkit-addr", docker.BuildKitSock, "Address of the buildkit socket, can be unix, tcp, ssl")
	buildCmd.PersistentFlags().StringP("platform", "p", docker.DefaultPlatforms, "Platforms to build")
	buildCmd.PersistentFlags().Bool("no-cache", false, "Don't use docker cache for building")
	buildCmd.PersistentFlags().Bool("no-build-cache", false, "Invalidate caches for clone and build.")
}
