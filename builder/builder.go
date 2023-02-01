package builder

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/strangelove-ventures/heighliner/docker"
	"github.com/strangelove-ventures/heighliner/dockerfile"
)

type HeighlinerBuilder struct {
	buildConfig HeighlinerDockerBuildConfig
	queue       []HeighlinerQueuedChainBuilds
	parallel    int16
	local       bool

	buildIndex   int
	buildIndexMu sync.Mutex

	errors     []error
	errorsLock sync.Mutex

	tmpDirsToRemove map[string]bool
	tmpDirMapMu     sync.Mutex
}

func NewHeighlinerBuilder(
	buildConfig HeighlinerDockerBuildConfig,
	parallel int16,
	local bool,
) *HeighlinerBuilder {
	return &HeighlinerBuilder{
		buildConfig: buildConfig,
		parallel:    parallel,
		local:       local,

		tmpDirsToRemove: make(map[string]bool),
	}
}

func (h *HeighlinerBuilder) AddToQueue(chainBuilds ...HeighlinerQueuedChainBuilds) {
	h.queue = append(h.queue, chainBuilds...)
}

// imageTag determines which docker image tag to use based on inputs.
func imageTag(ref string, tag string, local bool) string {
	if tag != "" {
		return tag
	}

	tag = deriveTagFromRef(ref)

	if local && tag == "" {
		return "local"
	}

	return tag
}

// deriveTagFromRef returns a sanitized docker image tag from a git ref (branch/tag).
func deriveTagFromRef(version string) string {
	return strings.ReplaceAll(version, "/", "-")
}

// dockerfileEmbeddedOrLocal attempts to find Dockerfile within current working directory.
// Returns embedded Dockerfile if local file is not found or cannot be read.
func dockerfileEmbeddedOrLocal(dockerfile string, embedded []byte) []byte {
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

	fmt.Printf("Using local %s\n", dockerfile)
	return df
}

// dockerfileAndTag returns the appropriate dockerfile as bytes and the docker image tag
// based on the input configuration.
func rawDockerfile(
	dockerfileType DockerfileType,
	useBuildKit bool,
	local bool,
) []byte {
	switch dockerfileType {
	case DockerfileTypeImported:
		return dockerfileEmbeddedOrLocal("imported/Dockerfile", dockerfile.Imported)

	case DockerfileTypeRust:
		// DEPRECATED
		fallthrough
	case DockerfileTypeCargo:
		if useBuildKit {
			return dockerfileEmbeddedOrLocal("cargo/Dockerfile", dockerfile.Cargo)
		}
		return dockerfileEmbeddedOrLocal("cargo/native.Dockerfile", dockerfile.CargoNative)

	case DockerfileTypeGo:
		// DEPRECATED
		fallthrough
	case DockerfileTypeCosmos:
		if local {
			// local builds always use embedded Dockerfile.
			return dockerfile.CosmosLocal
		}
		if useBuildKit {
			return dockerfileEmbeddedOrLocal("cosmos/Dockerfile", dockerfile.Cosmos)
		}
		return dockerfileEmbeddedOrLocal("cosmos/native.Dockerfile", dockerfile.CosmosNative)

	default:
		return dockerfileEmbeddedOrLocal("none/Dockerfile", dockerfile.None)
	}
}

// buildChainNodeDockerImage builds the requested chain node docker image
// based on the input configuration.
func (h *HeighlinerBuilder) buildChainNodeDockerImage(
	chainConfig *ChainNodeDockerBuildConfig,
) error {
	buildCfg := h.buildConfig
	dockerfile := chainConfig.Build.Dockerfile

	// DEPRECATION HANDLING
	if chainConfig.Build.Language != "" {
		fmt.Printf("'language' chain config property is deprecated, please use 'dockerfile' instead\n")
		if dockerfile == "" {
			dockerfile = chainConfig.Build.Language
		}
	}

	for _, rep := range deprecationReplacements {
		if dockerfile == rep[0] {
			fmt.Printf("'dockerfile' value of '%s' is deprecated, please use '%s' instead\n", rep[0], rep[1])
		}
	}
	// END DEPRECATION HANDLING

	df := rawDockerfile(dockerfile, buildCfg.UseBuildKit, h.local)

	tag := imageTag(chainConfig.Ref, chainConfig.Tag, h.local)

	fmt.Printf("Building docker image: %s:%s from ref: %s\n", chainConfig.Build.Name, tag, chainConfig.Ref)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting working directory: %w", err)
	}

	dir, err := os.MkdirTemp(cwd, "heighliner")
	if err != nil {
		return fmt.Errorf("error making temporary directory for dockerfile: %w", err)
	}

	// queue removal on ctrl+c
	h.queueTmpDirRemoval(dir, true)
	defer func() {
		// this build is done, so don't need removal on ctrl+c anymore since we are removing now.
		h.queueTmpDirRemoval(dir, false)
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
	if buildCfg.ContainerRegistry == "" {
		imageName = chainConfig.Build.Name
	} else {
		imageName = fmt.Sprintf("%s/%s", buildCfg.ContainerRegistry, chainConfig.Build.Name)
	}

	imageTags := []string{fmt.Sprintf("%s:%s", imageName, tag)}
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

	binaries := strings.Join(chainConfig.Build.Binaries, ",")

	libraries := strings.Join(chainConfig.Build.Libraries, " ")

	repoHost := chainConfig.Build.RepoHost
	if repoHost == "" {
		repoHost = "github.com"
	}

	buildTimestamp := ""
	if buildCfg.NoBuildCache {
		buildTimestamp = strconv.FormatInt(time.Now().Unix(), 10)
	}

	buildArgs := map[string]string{
		"VERSION":             chainConfig.Ref,
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

	push := buildCfg.ContainerRegistry != "" && !buildCfg.SkipPush

	if buildCfg.UseBuildKit {
		buildKitOptions := docker.GetDefaultBuildKitOptions()
		buildKitOptions.Address = buildCfg.BuildKitAddr
		supportedPlatforms := chainConfig.Build.Platforms

		if len(supportedPlatforms) > 0 {
			platforms := []string{}
			requestedPlatforms := strings.Split(buildCfg.Platform, ",")
			for _, supportedPlatform := range supportedPlatforms {
				for _, requestedPlatform := range requestedPlatforms {
					if supportedPlatform == requestedPlatform {
						platforms = append(platforms, requestedPlatform)
					}
				}
			}
			if len(platforms) == 0 {
				return fmt.Errorf("no requested platforms are supported for this chain: %s. requested: %s, supported: %s", chainConfig.Build.Name, buildCfg.Platform, strings.Join(supportedPlatforms, ","))
			}
			buildKitOptions.Platform = strings.Join(platforms, ",")
		} else {
			buildKitOptions.Platform = buildCfg.Platform
		}
		buildKitOptions.NoCache = buildCfg.NoCache
		if err := docker.BuildDockerImageWithBuildKit(ctx, reldir, imageTags, push, buildArgs, buildKitOptions); err != nil {
			return err
		}
	} else {
		if err := docker.BuildDockerImage(ctx, dfilepath, imageTags, push, buildArgs, buildCfg.NoCache); err != nil {
			return err
		}
	}
	return nil
}

// returns queue items, starting with latest for each chain
func (h *HeighlinerBuilder) getNextQueueItem() *ChainNodeDockerBuildConfig {
	h.buildIndexMu.Lock()
	defer h.buildIndexMu.Unlock()
	j := 0
	for i := 0; true; i++ {
		foundForThisIndex := false
		for _, queuedChainBuilds := range h.queue {
			if i < len(queuedChainBuilds.ChainConfigs) {
				if j == h.buildIndex {
					h.buildIndex++
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

func (h *HeighlinerBuilder) buildNextImage(wg *sync.WaitGroup) {
	chainConfig := h.getNextQueueItem()
	if chainConfig == nil {
		wg.Done()
		return
	}

	go func() {
		if err := h.buildChainNodeDockerImage(chainConfig); err != nil {
			h.errorsLock.Lock()
			h.errors = append(h.errors, fmt.Errorf("error building docker image for %s from ref: %s - %v\n", chainConfig.Build.Name, chainConfig.Ref, err))
			h.errorsLock.Unlock()
		}
		h.buildNextImage(wg)
	}()
}

func (h *HeighlinerBuilder) queueTmpDirRemoval(tmpDir string, start bool) {
	h.tmpDirMapMu.Lock()
	defer h.tmpDirMapMu.Unlock()
	if start {
		h.tmpDirsToRemove[tmpDir] = true
	} else {
		delete(h.tmpDirsToRemove, tmpDir)
	}
}

// registerSigIntHandler will delete tmp dirs on ctrl+c
func (h *HeighlinerBuilder) registerSigIntHandler() {
	c := make(chan os.Signal)
	//nolint:govet
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		h.tmpDirMapMu.Lock()
		defer h.tmpDirMapMu.Unlock()
		for dir := range h.tmpDirsToRemove {
			_ = os.RemoveAll(dir)
		}

		os.Exit(1)
	}()
}

func (h *HeighlinerBuilder) BuildImages() {
	h.registerSigIntHandler()

	wg := new(sync.WaitGroup)
	for i := int16(0); i < h.parallel; i++ {
		wg.Add(1)
		h.buildNextImage(wg)
	}
	wg.Wait()
	if len(h.errors) > 0 {
		for _, err := range h.errors {
			fmt.Println(err)
		}
		panic("Some images failed to build")
	}
}
