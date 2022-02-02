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
	Name               string `yaml:"name"`
	GithubOrganization string `yaml:"github-organization"`
	GithubRepo         string `yaml:"github-repo"`
	MakeTarget         string `yaml:"make-target"`
	BinaryPath         string `yaml:"binary-path"`
}

type GithubRelease struct {
	TagName string `json:"tag_name"`
}

func buildChainNodeDockerImage(containerRegistry string, chainNodeConfig ChainNodeConfig, version string, latest bool, containerAuthentication string) error {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	var imageName string
	if containerRegistry == "" {
		imageName = chainNodeConfig.Name
	} else {
		imageName = fmt.Sprintf("%s/%s", containerRegistry, chainNodeConfig.Name)
	}

	imageTags := []string{fmt.Sprintf("%s:%s", imageName, version)}
	if latest {
		imageTags = append(imageTags, fmt.Sprintf("%s:latest", imageName))
	}

	opts := types.ImageBuildOptions{
		Dockerfile: "Dockerfile",
		Tags:       imageTags,
		Remove:     true,
		BuildArgs: map[string]*string{
			"VERSION":             &version,
			"NAME":                &chainNodeConfig.Name,
			"GITHUB_ORGANIZATION": &chainNodeConfig.GithubOrganization,
			"GITHUB_REPO":         &chainNodeConfig.GithubRepo,
			"MAKE_TARGET":         &chainNodeConfig.MakeTarget,
			"BINARY":              &chainNodeConfig.BinaryPath,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*300)
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

func buildLastFiveReleasesForChain(chainNodeConfig ChainNodeConfig, containerRegistry string, containerAuthentication string) error {
	resp, err := http.Get(fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=5&page=1",
		chainNodeConfig.GithubOrganization, chainNodeConfig.GithubRepo))
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
		err := buildChainNodeDockerImage(containerRegistry, chainNodeConfig, release.TagName, i == 0, containerAuthentication)
		if err != nil {
			return err
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

		chain, _ := cmd.Flags().GetString("chain")
		version, _ := cmd.Flags().GetString("version")

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
			panic(err)
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
				err := buildChainNodeDockerImage(containerRegistry, chainNodeConfig, version, false, authStr)
				if err != nil {
					log.Fatalf("Error building docker image: %v", err)
				}
				return
			}
			// If specific version not provided, build images for the last 5 releases from the chain
			err := buildLastFiveReleasesForChain(chainNodeConfig, containerRegistry, authStr)
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
}
