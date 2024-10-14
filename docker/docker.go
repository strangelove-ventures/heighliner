package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
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

func BuildDockerImage(ctx context.Context, dockerfile string, tags []string, push bool, args map[string]string, noCache bool) error {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	buildArgs := map[string]*string{}

	for arg, value := range args {
		thisValue := value
		buildArgs[arg] = &thisValue
	}

	opts := types.ImageBuildOptions{
		NoCache:     noCache,
		Dockerfile:  dockerfile,
		Tags:        tags,
		NetworkMode: "host",
		Remove:      true,
		BuildArgs:   buildArgs,
	}

	tar, err := archive.TarWithOptions("./", &archive.TarOptions{})
	if err != nil {
		panic(fmt.Errorf("error archiving project for docker: %v", err))
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
			fmt.Printf("%s", dockerLogLine.Stream)
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
	if !push {
		return nil
	}

	// push all image tags to container registry using provided auth
	for _, imageTag := range tags {
		rd, err := dockerClient.ImagePush(ctx, imageTag, image.PushOptions{
			All: true,
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
