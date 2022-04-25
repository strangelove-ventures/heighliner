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
	"time"

	"github.com/spf13/cobra"
	"github.com/strangelove-ventures/heighliner/docker"
	"gopkg.in/yaml.v2"
)

type ChainNodeConfig struct {
	Name               string            `yaml:"name"`
	GithubOrganization string            `yaml:"github-organization"`
	GithubRepo         string            `yaml:"github-repo"`
	Language           string            `yaml:"language"`
	BuildTarget        string            `yaml:"build-target"`
	Binaries           []string          `yaml:"binaries"`
	PreBuild           string            `yaml:"pre-build"`
	BuildEnv           []string          `yaml:"build-env"`
	RocksDBVersion     map[string]string `yaml:"rocksdb-version"`
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

func buildChainNodeDockerImage(
	containerRegistry string,
	chainNodeConfig ChainNodeConfig,
	version string,
	latest bool,
	skip bool,
	rocksDbVersion string,
	useBuildKit bool,
	buildKitAddr string,
	platform string,
) error {
	var dockerfile string
	var imageTag string
	switch chainNodeConfig.Language {
	case "rust":
		dockerfile = "./dockerfile/rust"
		imageTag = strings.ReplaceAll(version, "/", "-")
	case "go":
		fallthrough
	default:
		if rocksDbVersion != "" {
			dockerfile = "./dockerfile/sdk-rocksdb"
			imageTag = fmt.Sprintf("%s-rocks", strings.ReplaceAll(version, "/", "-"))
		} else {
			dockerfile = "./dockerfile/sdk"
			imageTag = strings.ReplaceAll(version, "/", "-")
		}
	}

	var imageName string
	if containerRegistry == "" {
		imageName = chainNodeConfig.Name
	} else {
		imageName = fmt.Sprintf("%s/%s", containerRegistry, chainNodeConfig.Name)
	}

	imageTags := []string{fmt.Sprintf("%s:%s", imageName, imageTag)}
	if latest {
		imageTags = append(imageTags, fmt.Sprintf("%s:latest", imageName))
	}

	buildEnv := ""

	buildTagsEnvVar := ""
	for _, envVar := range chainNodeConfig.BuildEnv {
		envVarSplit := strings.Split(envVar, "=")
		if envVarSplit[0] == "BUILD_TAGS" && rocksDbVersion != "" {
			buildTagsEnvVar = fmt.Sprintf("BUILD_TAGS=%s rocksdb", trimQuotes(envVarSplit[1]))
		} else {
			buildEnv += envVar + " "
		}
	}
	if buildTagsEnvVar == "" && rocksDbVersion != "" {
		buildTagsEnvVar = "BUILD_TAGS=rocksdb"
	}

	binaries := strings.Join(chainNodeConfig.Binaries, " ")

	fmt.Printf("Building with dockerfile: %s\n", dockerfile)

	buildArgs := map[string]string{
		"VERSION":             version,
		"NAME":                chainNodeConfig.Name,
		"GITHUB_ORGANIZATION": chainNodeConfig.GithubOrganization,
		"GITHUB_REPO":         chainNodeConfig.GithubRepo,
		"BUILD_TARGET":        chainNodeConfig.BuildTarget,
		"BINARIES":            binaries,
		"PRE_BUILD":           chainNodeConfig.PreBuild,
		"BUILD_ENV":           buildEnv,
		"BUILD_TAGS":          buildTagsEnvVar,
		"ROCKSDB_VERSION":     rocksDbVersion,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Minute*15))
	defer cancel()

	if useBuildKit {
		buildKitOptions := docker.GetDefaultBuildKitOptions()
		buildKitOptions.Address = buildKitAddr
		buildKitOptions.Platform = platform
		return docker.BuildDockerImageWithBuildKit(ctx, dockerfile, imageTags, containerRegistry != "" && !skip, buildArgs, buildKitOptions)
	} else {
		return docker.BuildDockerImage(ctx, dockerfile, imageTags, containerRegistry != "" && !skip, buildArgs)
	}
}

func buildMostRecentReleasesForChain(chainNodeConfig ChainNodeConfig, number int16, containerRegistry string, skip bool, useBuildKit bool, buildKitAddr string, platform string) error {
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
		return fmt.Errorf("error parsing github releases response: %v", err)
	}
	for i, release := range releases {
		fmt.Printf("Building tag: %s\n", release.TagName)
		err := buildChainNodeDockerImage(containerRegistry, chainNodeConfig, release.TagName, i == 0, skip, "", useBuildKit, buildKitAddr, platform)
		if err != nil {
			fmt.Printf("Error building docker image: %v\n", err)
			continue
		}
		if rocksDbVersion, ok := chainNodeConfig.RocksDBVersion[release.TagName]; ok {
			err = buildChainNodeDockerImage(containerRegistry, chainNodeConfig, release.TagName, i == 0, skip, rocksDbVersion, useBuildKit, buildKitAddr, platform)
			if err != nil {
				fmt.Printf("Error building rocksdb docker image: %v\n", err)
			}
		}
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

		for _, chainNodeConfig := range chains {
			// If chain is provided, only build images for that chain
			// Chain must be declared in chains.yaml
			if chain != "" && chainNodeConfig.Name != chain {
				continue
			}
			fmt.Printf("Chain: %s\n", chainNodeConfig.Name)
			if version != "" {
				fmt.Printf("Building tag: %s\n", version)
				err := buildChainNodeDockerImage(containerRegistry, chainNodeConfig, version, false, skip, "", useBuildKit, buildKitAddr, platform)
				if err != nil {
					log.Fatalf("Error building docker image: %v", err)
				}
				if rocksDbVersion, ok := chainNodeConfig.RocksDBVersion[version]; ok {
					err = buildChainNodeDockerImage(containerRegistry, chainNodeConfig, version, false, skip, rocksDbVersion, useBuildKit, buildKitAddr, platform)
					if err != nil {
						log.Fatalf("Error building rocksdb docker image: %v", err)
					}
				}
				return
			}
			// If specific version not provided, build images for the last n releases from the chain
			err := buildMostRecentReleasesForChain(chainNodeConfig, number, containerRegistry, skip, useBuildKit, buildKitAddr, platform)
			if err != nil {
				log.Fatalf("Error building docker images: %v", err)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)

	buildCmd.PersistentFlags().StringP("registry", "r", "", "Docker Container Registry for pushing images")
	buildCmd.PersistentFlags().StringP("chain", "c", "", "Cosmos chain to build from chains.yaml")
	buildCmd.PersistentFlags().StringP("version", "v", "", "Github tag to build")
	buildCmd.PersistentFlags().Int16P("number", "n", 5, "Number of releases to build per chain")
	buildCmd.PersistentFlags().BoolP("skip", "s", false, "Skip pushing images to registry")

	buildCmd.PersistentFlags().BoolP("use-buildkit", "b", false, "Use buildkit to build multi-arch images")
	buildCmd.PersistentFlags().String("buildkit-addr", docker.BuildKitSock, "Address of the buildkit socket, can be unix, tcp, ssl")
	buildCmd.PersistentFlags().StringP("platform", "p", docker.DefaultPlatforms, "Platforms to build")
}
