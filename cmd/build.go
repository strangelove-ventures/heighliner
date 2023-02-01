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

const (
	flagFile         = "file"
	flagRegistry     = "registry"
	flagChain        = "chain"
	flagOrg          = "org"
	flagGitRef       = "git-ref"
	flagTag          = "tag"
	flagVersion      = "version" // DEPRECATED
	flagNumber       = "number"
	flagParallel     = "parallel"
	flagSkip         = "skip"
	flagLatest       = "latest"
	flagLocal        = "local"
	flagUseBuildkit  = "use-buildkit"
	flagBuildkitAddr = "buildkit-addr"
	flagPlatform     = "platform"
	flagNoCache      = "no-cache"
	flagNoBuildCache = "no-build-cache"
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

		containerRegistry, _ := cmdFlags.GetString(flagRegistry)
		chain, _ := cmdFlags.GetString(flagChain)
		version, _ := cmdFlags.GetString(flagVersion)
		ref, _ := cmdFlags.GetString(flagGitRef)
		tag, _ := cmdFlags.GetString(flagTag)
		org, _ := cmdFlags.GetString(flagOrg)
		number, _ := cmdFlags.GetInt16(flagNumber)
		skip, _ := cmdFlags.GetBool(flagSkip)

		useBuildKit, _ := cmdFlags.GetBool(flagUseBuildkit)
		buildKitAddr, _ := cmdFlags.GetString(flagBuildkitAddr)
		platform, _ := cmdFlags.GetString(flagPlatform)
		noCache, _ := cmdFlags.GetBool(flagNoCache)
		noBuildCache, _ := cmdFlags.GetBool(flagNoBuildCache)
		latest, _ := cmdFlags.GetBool(flagLatest)
		local, _ := cmdFlags.GetBool(flagLocal)
		parallel, _ := cmdFlags.GetInt16(flagParallel)

		buildConfig := builder.HeighlinerDockerBuildConfig{
			ContainerRegistry: containerRegistry,
			SkipPush:          skip,
			UseBuildKit:       useBuildKit,
			BuildKitAddr:      buildKitAddr,
			Platform:          platform,
			NoCache:           noCache,
			NoBuildCache:      noBuildCache,
		}

		// DEPRECATION HANDLING
		if version != "" {
			if ref == "" {
				ref = version
			}
			fmt.Printf(
				`Warning: --version/-v flag is deprecated. Please update to use --ref/-r instead. 
An optional flag --tag/-t is now available to override the resulting docker image tag if desirable to differ from the derived tag
`)
		}
		// END DEPRECATION HANDLING

		queueAndBuild(buildConfig, chain, org, ref, tag, latest, local, number, parallel)
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)

	buildCmd.PersistentFlags().StringP(flagFile, "f", "", "chains.yaml config file path")
	buildCmd.PersistentFlags().StringP(flagRegistry, "r", "", "Docker Container Registry for pushing images")
	buildCmd.PersistentFlags().StringP(flagChain, "c", "", "Cosmos chain to build from chains.yaml")
	buildCmd.PersistentFlags().StringP(flagOrg, "o", "", "Github organization override for building from a fork")
	buildCmd.PersistentFlags().StringP(flagGitRef, "g", "", "Github short ref to build (branch, tag)")
	buildCmd.PersistentFlags().StringP(flagTag, "t", "", "Resulting docker image tag. If not provided, will derive from ref.")
	buildCmd.PersistentFlags().Int16P(flagNumber, "n", 5, "Number of releases to build per chain")
	buildCmd.PersistentFlags().Int16(flagParallel, 1, "Number of docker builds to run simultaneously")
	buildCmd.PersistentFlags().BoolP(flagSkip, "s", false, "Skip pushing images to registry")
	buildCmd.PersistentFlags().BoolP(flagLatest, "l", false, "Also push latest tag (for single version build only)")
	buildCmd.PersistentFlags().Bool(flagLocal, false, "Use local directory (not git repository)")

	buildCmd.PersistentFlags().BoolP(flagUseBuildkit, "b", false, "Use buildkit to build multi-arch images")
	buildCmd.PersistentFlags().String(flagBuildkitAddr, docker.BuildKitSock, "Address of the buildkit socket, can be unix, tcp, ssl")
	buildCmd.PersistentFlags().StringP(flagPlatform, "p", docker.DefaultPlatforms, "Platforms to build (only applies to buildkit builds with -b)")
	buildCmd.PersistentFlags().Bool(flagNoCache, false, "Don't use docker cache for building")
	buildCmd.PersistentFlags().Bool(flagNoBuildCache, false, "Invalidate caches for clone and build.")

	// DEPRECATED
	buildCmd.PersistentFlags().StringP(flagVersion, "v", "", "DEPRECATED, use --ref/-r instead")
}
