package cmd

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type DockerImageBuildErrorDetail struct {
	Message string `json:"message"`
}

type DockerImageBuildLogAux struct {
	ID string `json:"ID"`
}

type DockerImageBuildLog struct {
	Stream      string                       `json:"stream"`
	Aux         *DockerImageBuildLogAux      `json:"aux"`
	Error       string                       `json:"error"`
	ErrorDetail *DockerImageBuildErrorDetail `json:"errorDetail"`
}

type ChainNodeConfig struct {
	Name               string            `yaml:"name"`
	GithubOrganization string            `yaml:"github-organization"`
	GithubRepo         string            `yaml:"github-repo"`
	MakeTarget         string            `yaml:"make-target"`
	BinaryPath         string            `yaml:"binary-path"`
	BuildEnv           []string          `yaml:"build-env"`
	RocksDBVersion     map[string]string `yaml:"rocksdb-version"`
}

type GithubRelease struct {
	TagName string `json:"tag_name"`
}

func buildChainNodeDockerImage(containerRegistry string, chainNodeConfig ChainNodeConfig, version string, latest bool, skip bool, containerAuthentication string, rocksDbVersion *string) error {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	var dockerfile string
	var makeTarget string
	var imageTag string
	if rocksDbVersion != nil {
		dockerfile = "rocksdb.Dockerfile"
		makeTarget = fmt.Sprintf("%s BUILD_TAGS=rocksdb", chainNodeConfig.MakeTarget)
		imageTag = fmt.Sprintf("%s-rocks", version)
	} else {
		dockerfile = "Dockerfile"
		makeTarget = chainNodeConfig.MakeTarget
		imageTag = version
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
	for _, envVar := range chainNodeConfig.BuildEnv {
		buildEnv += envVar + " "
	}

	opts := types.ImageBuildOptions{
		Dockerfile:  dockerfile,
		Tags:        imageTags,
		NetworkMode: "host",
		Remove:      true,
		BuildArgs: map[string]*string{
			"VERSION":             &version,
			"NAME":                &chainNodeConfig.Name,
			"GITHUB_ORGANIZATION": &chainNodeConfig.GithubOrganization,
			"GITHUB_REPO":         &chainNodeConfig.GithubRepo,
			"MAKE_TARGET":         &makeTarget,
			"BINARY":              &chainNodeConfig.BinaryPath,
			"BUILD_ENV":           &buildEnv,
			"ROCKSDB_VERSION":     rocksDbVersion,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3000)
	defer cancel()

	tar, err := archive.TarWithOptions("./", &archive.TarOptions{})
	if err != nil {
		log.Fatalf("Error archiving project for docker: %v", err)
	}

	res, err := dockerClient.ImageBuild(ctx, tar, opts)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(res.Body)

	for scanner.Scan() {
		dockerLogLine := &DockerImageBuildLog{}
		logLineText := scanner.Text()
		err = json.Unmarshal([]byte(logLineText), dockerLogLine)
		if err != nil {
			return err
		}
		if dockerLogLine.Stream != "" {
			fmt.Printf(dockerLogLine.Stream)
		}
		if dockerLogLine.Aux != nil {
			fmt.Printf("Image ID: %s\n", dockerLogLine.Aux.ID)
		}
		if dockerLogLine.Error != "" {
			return errors.New(dockerLogLine.Error)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Only continue to push images if registry is provided
	if containerRegistry == "" || skip {
		return nil
	}

	// push all image tags to container registry using provided auth
	for _, imageTag := range imageTags {
		rd, err := dockerClient.ImagePush(ctx, imageTag, types.ImagePushOptions{
			All:          true,
			RegistryAuth: containerAuthentication,
		})
		if err != nil {
			return err
		}

		defer rd.Close()

		buf := new(strings.Builder)
		_, err = io.Copy(buf, rd)

		if err != nil {
			return err
		}

		fmt.Println(buf.String())
	}

	return nil
}

func buildMostRecentReleasesForChain(chainNodeConfig ChainNodeConfig, number int16, containerRegistry string, skip bool, containerAuthentication string) error {
	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=%d&page=1",
		chainNodeConfig.GithubOrganization, chainNodeConfig.GithubRepo, number))
	if err != nil {
		return err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	releases := []GithubRelease{}
	err = json.Unmarshal(body, &releases)
	if err != nil {
		return err
	}
	for i, release := range releases {
		fmt.Printf("Building tag: %s\n", release.TagName)
		err := buildChainNodeDockerImage(containerRegistry, chainNodeConfig, release.TagName, i == 0, skip, containerAuthentication, nil)
		if err != nil {
			fmt.Printf("Error building docker image: %v\n", err)
			continue
		}
		if rocksDbVersion, ok := chainNodeConfig.RocksDBVersion[release.TagName]; ok {
			err = buildChainNodeDockerImage(containerRegistry, chainNodeConfig, release.TagName, i == 0, skip, containerAuthentication, &rocksDbVersion)
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

		authConfig := types.AuthConfig{
			Username: os.Getenv("DOCKER_USER"),
			Password: os.Getenv("DOCKER_PASSWORD"),
		}
		encodedJSON, err := json.Marshal(authConfig)
		if err != nil {
			log.Fatalf("Error assembling docker registry authentication string: %v", err)
		}
		authStr := base64.URLEncoding.EncodeToString(encodedJSON)

		for _, chainNodeConfig := range chains {
			// If chain is provided, only build images for that chain
			// Chain must be declared in chains.yaml
			if chain != "" && chainNodeConfig.Name != chain {
				continue
			}
			fmt.Printf("Chain: %s\n", chainNodeConfig.Name)
			if version != "" {
				fmt.Printf("Building tag: %s\n", version)
				err := buildChainNodeDockerImage(containerRegistry, chainNodeConfig, version, false, skip, authStr, nil)
				if err != nil {
					log.Fatalf("Error building docker image: %v", err)
				}
				if rocksDbVersion, ok := chainNodeConfig.RocksDBVersion[version]; ok {
					err = buildChainNodeDockerImage(containerRegistry, chainNodeConfig, version, false, skip, authStr, &rocksDbVersion)
					if err != nil {
						log.Fatalf("Error building rocksdb docker image: %v", err)
					}
				}
				return
			}
			// If specific version not provided, build images for the last n releases from the chain
			err := buildMostRecentReleasesForChain(chainNodeConfig, number, containerRegistry, skip, authStr)
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
}
