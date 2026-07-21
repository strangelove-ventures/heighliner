package builder

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func testCloneKey(t *testing.T) string {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKey(privateKey, "heighliner-test")
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(pem.EncodeToMemory(block))
}

func testKnownHosts(t *testing.T) string {
	t.Helper()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(path, append([]byte("github.com "), ssh.MarshalAuthorizedKey(sshPublicKey)...), 0600))
	return path
}

type socketFileInfo struct{}

func (socketFileInfo) Name() string       { return "agent.sock" }
func (socketFileInfo) Size() int64        { return 0 }
func (socketFileInfo) Mode() os.FileMode  { return os.ModeSocket }
func (socketFileInfo) ModTime() time.Time { return time.Time{} }
func (socketFileInfo) IsDir() bool        { return false }
func (socketFileInfo) Sys() any           { return nil }

func socketStat(string) (os.FileInfo, error) {
	return socketFileInfo{}, nil
}

func TestResolveGitAuthNone(t *testing.T) {
	auth, err := ResolveGitAuth(HeighlinerDockerBuildConfig{}, ChainNodeConfig{})
	require.NoError(t, err)
	require.Equal(t, GitAuthModeNone, auth.Mode)
}

func TestResolveGitAuthRejectsConflictingSources(t *testing.T) {
	t.Setenv("GH_TOKEN", "secret")
	_, err := ResolveGitAuth(HeighlinerDockerBuildConfig{
		UseBuildKit:   true,
		UseSSH:        true,
		UseGitHubAuth: true,
	}, ChainNodeConfig{Dockerfile: DockerfileTypeCosmos})
	require.ErrorContains(t, err, "mutually exclusive")
}

func TestResolveStandardGitHubToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "primary-token")
	t.Setenv("GITHUB_TOKEN", "fallback-token")

	auth, err := ResolveGitAuth(HeighlinerDockerBuildConfig{
		UseBuildKit:   true,
		UseGitHubAuth: true,
	}, ChainNodeConfig{Dockerfile: DockerfileTypeCosmos})
	require.NoError(t, err)
	require.Equal(t, GitAuthModeGitHubToken, auth.Mode)
	require.Equal(t, "primary-token", auth.GitHubToken)
	require.Contains(t, string(auth.Netrc()), "password primary-token")
	cloneOptions, err := gitCloneOptions("github.com", "example", "private", auth)
	require.NoError(t, err)
	require.NotContains(t, cloneOptions.URL, "primary-token")
	require.Equal(t, "https://github.com/example/private", cloneOptions.URL)
	require.Equal(t, "primary-token", cloneOptions.Auth.(*githttp.BasicAuth).Password)

	t.Setenv("GH_TOKEN", "")
	auth, err = ResolveGitAuth(HeighlinerDockerBuildConfig{
		UseBuildKit:   true,
		UseGitHubAuth: true,
	}, ChainNodeConfig{Dockerfile: DockerfileTypeCosmos})
	require.NoError(t, err)
	require.Equal(t, "fallback-token", auth.GitHubToken)

	t.Setenv("GITHUB_TOKEN", "")
	_, err = ResolveGitAuth(HeighlinerDockerBuildConfig{
		UseBuildKit:   true,
		UseGitHubAuth: true,
	}, ChainNodeConfig{Dockerfile: DockerfileTypeCosmos})
	require.ErrorContains(t, err, "requires GH_TOKEN or GITHUB_TOKEN")

	_, err = ResolveGitAuth(HeighlinerDockerBuildConfig{
		UseBuildKit:   true,
		UseGitHubAuth: true,
	}, ChainNodeConfig{RepoHost: "git.example.com", Dockerfile: DockerfileTypeCosmos})
	require.ErrorContains(t, err, "only supports github.com")

	_, err = ResolveGitAuth(HeighlinerDockerBuildConfig{
		UseBuildKit:       true,
		UseGitHubAuth:     true,
		SSHKnownHostsPath: testKnownHosts(t),
	}, ChainNodeConfig{Dockerfile: DockerfileTypeCosmos})
	require.ErrorContains(t, err, "requires SSH authentication")
}

func TestNewAuthOptionsRequireBuildKit(t *testing.T) {
	t.Setenv("GH_TOKEN", "github-token-value")
	_, err := ResolveGitAuth(HeighlinerDockerBuildConfig{
		UseGitHubAuth: true,
	}, ChainNodeConfig{Dockerfile: DockerfileTypeCosmos})
	require.ErrorContains(t, err, "require --use-buildkit")

	_, err = ResolveGitAuth(HeighlinerDockerBuildConfig{
		SSHKnownHostsPath: testKnownHosts(t),
	}, ChainNodeConfig{Dockerfile: DockerfileTypeCosmos, CloneKey: testCloneKey(t)})
	require.ErrorContains(t, err, "require --use-buildkit")
}

func TestNewAuthOptionsAreLimitedToCosmosDockerfiles(t *testing.T) {
	t.Setenv("GH_TOKEN", "github-token-value")
	_, err := ResolveGitAuth(HeighlinerDockerBuildConfig{
		UseBuildKit:   true,
		UseGitHubAuth: true,
	}, ChainNodeConfig{Dockerfile: DockerfileTypeCargo})
	require.ErrorContains(t, err, "only Cosmos Dockerfiles")
}

func TestResolveSSHAgentAndKnownHosts(t *testing.T) {
	socketPath := "/tmp/test-agent.sock"
	t.Setenv("SSH_AUTH_SOCK", socketPath)
	t.Setenv("SSH_KNOWN_HOSTS", testKnownHosts(t))

	auth, err := resolveGitAuth(HeighlinerDockerBuildConfig{
		UseBuildKit: true,
		UseSSH:      true,
	}, ChainNodeConfig{Dockerfile: DockerfileTypeCosmos}, socketStat)
	require.NoError(t, err)
	require.Equal(t, GitAuthModeSSH, auth.Mode)
	require.Equal(t, socketPath, auth.SSHAgentSocket)
	require.NotEmpty(t, auth.SSHKnownHosts)
}

func TestResolveSSHRequiresAgentAndKnownHosts(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	_, err := ResolveGitAuth(HeighlinerDockerBuildConfig{UseBuildKit: true, UseSSH: true}, ChainNodeConfig{Dockerfile: DockerfileTypeCosmos})
	require.ErrorContains(t, err, "SSH_AUTH_SOCK")

	socketPath := "/tmp/test-agent.sock"
	t.Setenv("SSH_AUTH_SOCK", socketPath)
	t.Setenv("SSH_KNOWN_HOSTS", "")
	t.Setenv("HOME", t.TempDir())
	_, err = resolveGitAuth(HeighlinerDockerBuildConfig{UseBuildKit: true, UseSSH: true}, ChainNodeConfig{Dockerfile: DockerfileTypeCosmos}, socketStat)
	require.ErrorContains(t, err, "known_hosts")
}

func TestResolveCloneKeyEnvAndLegacyCloneKey(t *testing.T) {
	encodedKey := testCloneKey(t)
	knownHosts := testKnownHosts(t)
	t.Setenv("PRIVATE_CLONE_KEY", encodedKey)

	auth, err := ResolveGitAuth(HeighlinerDockerBuildConfig{
		UseBuildKit:       true,
		CloneKeyEnv:       "PRIVATE_CLONE_KEY",
		SSHKnownHostsPath: knownHosts,
	}, ChainNodeConfig{Dockerfile: DockerfileTypeCosmos})
	require.NoError(t, err)
	require.NotEmpty(t, auth.SSHPrivateKey)
	require.False(t, auth.InsecureLegacyClone)

	legacyAuth, err := ResolveGitAuth(HeighlinerDockerBuildConfig{}, ChainNodeConfig{CloneKey: encodedKey})
	require.NoError(t, err)
	require.True(t, legacyAuth.InsecureLegacyClone)
	cloneOptions, err := gitCloneOptions("github.com", "example", "private", legacyAuth)
	require.NoError(t, err)
	require.Equal(t, "git@github.com:example/private.git", cloneOptions.URL)
	require.IsType(t, &gitssh.PublicKeys{}, cloneOptions.Auth)
}

func TestGitAuthBuildConfigDoesNotPutBuildKitSecretsInArgs(t *testing.T) {
	token := "token-must-not-be-an-arg"
	args, sessionOptions := gitAuthBuildConfig(true, "legacy-key-must-not-be-an-arg", GitAuthConfig{
		Mode:        GitAuthModeGitHubToken,
		GitHubToken: token,
	})
	require.Equal(t, map[string]string{"GIT_AUTH_MODE": "github-token"}, args)
	for key, value := range args {
		require.NotContains(t, key, token)
		require.NotContains(t, value, token)
	}
	require.Contains(t, string(sessionOptions.Secrets["git_netrc"]), token)

	key := []byte("private-key-must-not-be-an-arg")
	args, sessionOptions = gitAuthBuildConfig(true, string(key), GitAuthConfig{Mode: GitAuthModeSSH, SSHPrivateKey: key})
	require.NotContains(t, strings.Join([]string{args["GIT_AUTH_MODE"], args["CLONE_KEY"]}, ""), string(key))
	require.Equal(t, key, sessionOptions.SSHPrivateKey)

	legacyArgs, _ := gitAuthBuildConfig(false, "legacy-key", GitAuthConfig{Mode: GitAuthModeSSH})
	require.Equal(t, "legacy-key", legacyArgs["CLONE_KEY"])
}

func TestCloneKeyDeprecationWarnings(t *testing.T) {
	warnings := CloneKeyDeprecationWarnings(HeighlinerDockerBuildConfig{}, ChainNodeConfig{CloneKey: "configured"})
	require.Len(t, warnings, 2)
	require.Contains(t, warnings[0], "deprecated")
	require.Contains(t, warnings[1], "insecure legacy")

	warnings = CloneKeyDeprecationWarnings(HeighlinerDockerBuildConfig{UseBuildKit: true}, ChainNodeConfig{CloneKey: "configured"})
	require.Len(t, warnings, 1)
}
