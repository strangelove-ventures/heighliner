package builder

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/strangelove-ventures/heighliner/docker"
)

type GitAuthMode string

const (
	GitAuthModeNone        GitAuthMode = "none"
	GitAuthModeSSH         GitAuthMode = "ssh"
	GitAuthModeGitHubToken GitAuthMode = "github-token"
	githubTokenEnv         string      = "GH_TOKEN"
	githubActionsTokenEnv  string      = "GITHUB_TOKEN"
)

type GitAuthConfig struct {
	Mode                GitAuthMode
	SSHAgentSocket      string
	SSHPrivateKey       []byte
	SSHKnownHosts       []byte
	SSHKnownHostsPaths  []string
	GitHubToken         string
	InsecureLegacyClone bool
}

func (a GitAuthConfig) Netrc() []byte {
	if a.Mode != GitAuthModeGitHubToken {
		return nil
	}
	return []byte(fmt.Sprintf("machine github.com\nlogin x-access-token\npassword %s\n", a.GitHubToken))
}

func ResolveGitAuth(buildConfig HeighlinerDockerBuildConfig, chainConfig ChainNodeConfig) (GitAuthConfig, error) {
	return resolveGitAuth(buildConfig, chainConfig, os.Stat)
}

func resolveGitAuth(
	buildConfig HeighlinerDockerBuildConfig,
	chainConfig ChainNodeConfig,
	stat func(string) (os.FileInfo, error),
) (GitAuthConfig, error) {
	repoHost := chainConfig.RepoHost
	if repoHost == "" {
		repoHost = "github.com"
	}

	sources := 0
	if buildConfig.UseSSH {
		sources++
	}
	if buildConfig.UseGitHubAuth {
		sources++
	}
	if buildConfig.CloneKeyEnv != "" {
		sources++
	}
	if chainConfig.CloneKey != "" {
		sources++
	}
	if sources > 1 {
		return GitAuthConfig{}, errors.New("private Git authentication options are mutually exclusive")
	}
	usesSSH := buildConfig.UseSSH || buildConfig.CloneKeyEnv != "" || chainConfig.CloneKey != ""
	if buildConfig.SSHKnownHostsPath != "" && !usesSSH {
		return GitAuthConfig{}, errors.New("--ssh-known-hosts requires SSH authentication")
	}
	if sources == 0 {
		return GitAuthConfig{Mode: GitAuthModeNone}, nil
	}

	usesNewOption := buildConfig.UseSSH || buildConfig.SSHKnownHostsPath != "" || buildConfig.UseGitHubAuth || buildConfig.CloneKeyEnv != ""
	if usesNewOption && !buildConfig.UseBuildKit {
		return GitAuthConfig{}, errors.New("--ssh, --ssh-known-hosts, --github-auth, and --clone-key-env require --use-buildkit")
	}
	if usesNewOption && !isCosmosDockerfile(chainConfig) {
		return GitAuthConfig{}, errors.New("new private Git authentication options currently support only Cosmos Dockerfiles")
	}

	if buildConfig.UseGitHubAuth {
		if repoHost != "github.com" {
			return GitAuthConfig{}, fmt.Errorf("GitHub token authentication only supports github.com repositories, got %q", repoHost)
		}

		token := os.Getenv(githubTokenEnv)
		if token == "" {
			token = os.Getenv(githubActionsTokenEnv)
		}
		if token == "" {
			return GitAuthConfig{}, errors.New("--github-auth requires GH_TOKEN or GITHUB_TOKEN to be set and non-empty")
		}
		if strings.ContainsAny(token, "\r\n") {
			return GitAuthConfig{}, errors.New("GitHub token environment variable contains a newline")
		}
		return GitAuthConfig{Mode: GitAuthModeGitHubToken, GitHubToken: token}, nil
	}

	auth := GitAuthConfig{Mode: GitAuthModeSSH}
	if buildConfig.UseSSH {
		auth.SSHAgentSocket = os.Getenv("SSH_AUTH_SOCK")
		if auth.SSHAgentSocket == "" {
			return GitAuthConfig{}, errors.New("--ssh requires SSH_AUTH_SOCK to be set")
		}
		info, err := stat(auth.SSHAgentSocket)
		if err != nil {
			return GitAuthConfig{}, fmt.Errorf("cannot access SSH agent socket %q: %w", auth.SSHAgentSocket, err)
		}
		if info.Mode()&os.ModeSocket == 0 {
			return GitAuthConfig{}, fmt.Errorf("SSH_AUTH_SOCK %q is not a Unix socket", auth.SSHAgentSocket)
		}
	} else {
		encodedKey := chainConfig.CloneKey
		if buildConfig.CloneKeyEnv != "" {
			encodedKey = os.Getenv(buildConfig.CloneKeyEnv)
			if encodedKey == "" {
				return GitAuthConfig{}, fmt.Errorf("clone key environment variable %q is not set or empty", buildConfig.CloneKeyEnv)
			}
		}

		privateKey, err := base64.StdEncoding.DecodeString(encodedKey)
		if err != nil {
			return GitAuthConfig{}, errors.New("failed to decode clone key")
		}
		if _, err := gitssh.NewPublicKeys("git", privateKey, ""); err != nil {
			return GitAuthConfig{}, errors.New("failed to parse clone key")
		}
		auth.SSHPrivateKey = privateKey
	}

	if !buildConfig.UseBuildKit && chainConfig.CloneKey != "" {
		auth.InsecureLegacyClone = true
		return auth, nil
	}

	paths, contents, err := resolveKnownHosts(buildConfig.SSHKnownHostsPath)
	if err != nil {
		return GitAuthConfig{}, err
	}
	auth.SSHKnownHostsPaths = paths
	auth.SSHKnownHosts = contents
	return auth, nil
}

func isCosmosDockerfile(chainConfig ChainNodeConfig) bool {
	dockerfile := chainConfig.Dockerfile
	if dockerfile == "" {
		dockerfile = chainConfig.Language
	}
	return dockerfile == DockerfileTypeCosmos || dockerfile == DockerfileTypeGo
}

func resolveKnownHosts(explicitPath string) ([]string, []byte, error) {
	knownHostsPath := explicitPath
	if knownHostsPath == "" {
		knownHostsPath = os.Getenv("SSH_KNOWN_HOSTS")
	}
	if knownHostsPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, nil, fmt.Errorf("cannot locate ~/.ssh/known_hosts: %w", err)
		}
		knownHostsPath = filepath.Join(home, ".ssh", "known_hosts")
	}

	paths := filepath.SplitList(knownHostsPath)
	var contents []byte
	for _, knownHostsFile := range paths {
		if knownHostsFile == "" {
			continue
		}
		entry, err := os.ReadFile(knownHostsFile)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot read SSH known_hosts file %q: %w", knownHostsFile, err)
		}
		contents = append(contents, entry...)
		if len(contents) > 0 && contents[len(contents)-1] != '\n' {
			contents = append(contents, '\n')
		}
	}
	if len(paths) == 0 || len(contents) == 0 {
		return nil, nil, errors.New("SSH authentication requires a non-empty known_hosts file")
	}
	return paths, contents, nil
}

func CloneKeyDeprecationWarnings(buildConfig HeighlinerDockerBuildConfig, chainConfig ChainNodeConfig) []string {
	if chainConfig.CloneKey == "" {
		return nil
	}
	warnings := []string{"Warning: --clone-key and YAML clone-key are deprecated; use --ssh or --clone-key-env instead."}
	if !buildConfig.UseBuildKit {
		warnings = append(warnings, "Warning: non-BuildKit clone-key authentication uses the insecure legacy image-layer path; use --use-buildkit.")
	}
	return warnings
}

func gitAuthBuildConfig(useBuildKit bool, legacyCloneKey string, auth GitAuthConfig) (map[string]string, docker.BuildKitSessionOptions) {
	args := map[string]string{"GIT_AUTH_MODE": string(auth.Mode)}
	if !useBuildKit {
		args["CLONE_KEY"] = legacyCloneKey
	}

	sessionOptions := docker.BuildKitSessionOptions{
		SSHAgentSocket: auth.SSHAgentSocket,
		SSHPrivateKey:  auth.SSHPrivateKey,
		Secrets:        map[string][]byte{},
	}
	if len(auth.SSHKnownHosts) != 0 {
		sessionOptions.Secrets[docker.GitKnownHostsSecretID] = auth.SSHKnownHosts
	}
	if netrc := auth.Netrc(); len(netrc) != 0 {
		sessionOptions.Secrets[docker.GitNetrcSecretID] = netrc
	}
	return args, sessionOptions
}
