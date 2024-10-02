package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/strangelove-ventures/heighliner/builder"
	"github.com/strangelove-ventures/heighliner/docker"
	"gopkg.in/yaml.v2"
)

type chainConfigFlags struct {
	chain    string
	ref      string
	tag      string
	latest   bool
	local    bool
	number   int16
	parallel int16
	race     bool

	// chains.yaml parameter override flags
	orgOverride         string
	repoOverride        string
	repoHostOverride    string
	cloneKeyOverride    string
	dockerfileOverride  string
	buildDirOverride    string
	preBuildOverride    string
	buildTargetOverride string
	buildEnvOverride    string
	binariesOverride    string
	librariesOverride   string
}

const (
	flagFile          = "file"
	flagRegistry      = "registry"
	flagChain         = "chain"
	flagOrg           = "org"
	flagRepo          = "repo"
	flagRepoHost      = "repo-host"
	flagCloneKey      = "clone-key"
	flagGitRef        = "git-ref"
	flagDockerfile    = "dockerfile"
	flagBuildDir      = "build-dir"
	flagPreBuild      = "pre-build"
	flagBuildTarget   = "build-target"
	flagBuildEnv      = "build-env"
	flagBinaries      = "binaries"
	flagLibraries     = "libraries"
	flagTag           = "tag"
	flagVersion       = "version" // DEPRECATED
	flagNumber        = "number"
	flagParallel      = "parallel"
	flagSkip          = "skip"
	flagTarExport     = "tar-export-path"
	flagLatest        = "latest"
	flagLocal         = "local"
	flagUseBuildkit   = "use-buildkit"
	flagBuildkitAddr  = "buildkit-addr"
	flagPlatform      = "platform"
	flagNoCache       = "no-cache"
	flagNoBuildCache  = "no-build-cache"
	flagRace          = "race"
	flagGoVersion     = "go-version"
	flagAlpineVersion = "alpine-version"
)

func loadChainsYaml(configFile string) error {
	if _, err := os.Stat(configFile); err != nil {
		return fmt.Errorf("error checking for file: %s: %w", configFile, err)
	}
	bz, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("error reading file: %s: %w", configFile, err)
	}
	var newChains []builder.ChainNodeConfig
	err = yaml.Unmarshal(bz, &newChains)
	if err != nil {
		return fmt.Errorf("error unmarshalling yaml from file: %s: %w", configFile, err)
	}
	chains = newChains
	return nil
}

func BuildCmd() *cobra.Command {
	var chainConfig chainConfigFlags
	var buildConfig builder.HeighlinerDockerBuildConfig

	var buildCmd = &cobra.Command{
		Use:   "build",
		Short: "Build the docker images",
		Long: `By default, fetch the last 5 releases in the repositories specified in chains.yaml.
For each tag that doesn't exist in the specified container repository,
it will be built and pushed`,
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

			version, _ := cmdFlags.GetString(flagVersion)

			// DEPRECATION HANDLING
			if version != "" {
				if chainConfig.ref == "" {
					chainConfig.ref = version
				}
				fmt.Printf(
					`Warning: --version/-v flag is deprecated. Please update to use --git-ref/-g instead. 
An optional flag --tag/-t is now available to override the resulting docker image tag if desirable to differ from the derived tag
`)
			}
			// END DEPRECATION HANDLING

			queueAndBuild(buildConfig, chainConfig)
		},
	}

	buildCmd.PersistentFlags().StringP(flagFile, "f", "", "chains.yaml config file path (searches for chains.yaml in current directory by default)")

	// Chain config options
	buildCmd.PersistentFlags().StringVarP(&chainConfig.chain, flagChain, "c", "", "Cosmos chain to build from chains.yaml")
	buildCmd.PersistentFlags().StringVarP(&chainConfig.ref, flagGitRef, "g", "", "Github short ref to build (branch, tag)")
	buildCmd.PersistentFlags().StringVarP(&chainConfig.tag, flagTag, "t", "", "Resulting docker image tag. If not provided, will derive from ref.")
	buildCmd.PersistentFlags().Int16VarP(&chainConfig.number, flagNumber, "n", 5, "Number of releases to build per chain")
	buildCmd.PersistentFlags().Int16Var(&chainConfig.parallel, flagParallel, 1, "Number of docker builds to run simultaneously")
	buildCmd.PersistentFlags().BoolVarP(&chainConfig.latest, flagLatest, "l", false, "Also push latest tag (for single version build only)")
	buildCmd.PersistentFlags().BoolVar(&chainConfig.local, flagLocal, false, "Use local directory (not git repository)")
	buildCmd.PersistentFlags().BoolVar(&chainConfig.race, flagRace, false, "Enable race detector (go builds only)")

	// Chain config override flags (overwrites chains.yaml params)
	buildCmd.PersistentFlags().StringVarP(&chainConfig.orgOverride, flagOrg, "o", "", "github-organization override for building from a fork")
	buildCmd.PersistentFlags().StringVar(&chainConfig.repoOverride, flagRepo, "", "github-repo override for building from a fork")
	buildCmd.PersistentFlags().StringVar(&chainConfig.repoHostOverride, flagRepoHost, "", "repo-host Git repository host override for building from a fork")
	buildCmd.PersistentFlags().StringVar(&chainConfig.cloneKeyOverride, flagCloneKey, "", "base64 encoded ssh key to authenticate")
	buildCmd.PersistentFlags().StringVar(&chainConfig.dockerfileOverride, flagDockerfile, "", "dockerfile override (cosmos, cargo, imported, none)")
	buildCmd.PersistentFlags().StringVar(&chainConfig.buildDirOverride, flagBuildDir, "", "build-dir override - repo relative directory to run build target")
	buildCmd.PersistentFlags().StringVar(&chainConfig.preBuildOverride, flagPreBuild, "", "pre-build override - command(s) to run prior to build-target")
	buildCmd.PersistentFlags().StringVar(&chainConfig.buildTargetOverride, flagBuildTarget, "", "Build target (build-target) override")
	buildCmd.PersistentFlags().StringVar(&chainConfig.buildEnvOverride, flagBuildEnv, "", "build-env override - Build environment variables")
	buildCmd.PersistentFlags().StringVar(&chainConfig.binariesOverride, flagBinaries, "", "binaries override - Binaries after build phase to package into final image")
	buildCmd.PersistentFlags().StringVar(&chainConfig.librariesOverride, flagLibraries, "", "libraries override - Libraries after build phase to package into final image")

	// Docker specific flags
	buildCmd.PersistentFlags().StringVarP(&buildConfig.ContainerRegistry, flagRegistry, "r", "", "Docker Container Registry for pushing images")
	buildCmd.PersistentFlags().BoolVarP(&buildConfig.SkipPush, flagSkip, "s", false, "Skip pushing images to registry")
	buildCmd.PersistentFlags().StringVar(&buildConfig.TarExportPath, flagTarExport, "", "File path to export built image as docker tarball")
	buildCmd.PersistentFlags().BoolVarP(&buildConfig.UseBuildKit, flagUseBuildkit, "b", false, "Use buildkit to build multi-arch images")
	buildCmd.PersistentFlags().StringVar(&buildConfig.BuildKitAddr, flagBuildkitAddr, docker.BuildKitSock, "Address of the buildkit socket, can be unix, tcp, ssl")
	buildCmd.PersistentFlags().StringVarP(&buildConfig.Platform, flagPlatform, "p", docker.DefaultPlatforms, "Platforms to build (only applies to buildkit builds with -b)")
	buildCmd.PersistentFlags().BoolVar(&buildConfig.NoCache, flagNoCache, false, "Don't use docker cache for building")
	buildCmd.PersistentFlags().BoolVar(&buildConfig.NoBuildCache, flagNoBuildCache, false, "Invalidate caches for clone and build.")
	buildCmd.PersistentFlags().StringVar(&buildConfig.GoVersion, flagGoVersion, "", "Go version override to use for building (go builds only)")
	buildCmd.PersistentFlags().StringVar(&buildConfig.AlpineVersion, flagAlpineVersion, "", "Alpine version override to use for building (go builds only)")

	// DEPRECATED
	buildCmd.PersistentFlags().StringP(flagVersion, "v", "", "DEPRECATED, use --git-ref/-g instead")

	return buildCmd
}
