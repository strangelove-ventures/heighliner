package docker

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func testSSHPrivateKey(t *testing.T) []byte {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKey(privateKey, "heighliner-test")
	require.NoError(t, err)
	return pem.EncodeToMemory(block)
}

func TestBuildKitFrontendAttrsContainOnlyNonSecretAuthMode(t *testing.T) {
	secret := "must-not-appear"
	options := BuildKitOptions{
		Platform: "linux/amd64",
		NoCache:  true,
		Session: BuildKitSessionOptions{Secrets: map[string][]byte{
			GitNetrcSecretID: []byte(secret),
		}},
	}
	attrs := buildKitFrontendAttrs(map[string]string{
		"GIT_AUTH_MODE": "github-token",
		"NAME":          "example",
	}, options)

	require.Equal(t, "github-token", attrs["build-arg:GIT_AUTH_MODE"])
	require.Contains(t, attrs, "no-cache")
	for key, value := range attrs {
		require.NotContains(t, key, secret)
		require.NotContains(t, value, secret)
	}
}

func TestBuildKitSessionAttachables(t *testing.T) {
	attachables, err := buildKitSessionAttachables(BuildKitSessionOptions{
		SSHPrivateKey: testSSHPrivateKey(t),
		Secrets: map[string][]byte{
			GitKnownHostsSecretID: []byte("github.com ssh-ed25519 test"),
			GitNetrcSecretID:      []byte("machine github.com"),
		},
	})
	require.NoError(t, err)
	require.Len(t, attachables, 2)

	_, err = buildKitSessionAttachables(BuildKitSessionOptions{
		SSHAgentSocket: "/tmp/agent.sock",
		SSHPrivateKey:  testSSHPrivateKey(t),
	})
	require.ErrorContains(t, err, "both an SSH agent and a private key")
}
