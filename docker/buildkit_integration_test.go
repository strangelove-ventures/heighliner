package docker

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildKitAuthenticationSessionMounts(t *testing.T) {
	buildKitAddress := os.Getenv("BUILDKIT_ADDR")
	if buildKitAddress == "" {
		t.Skip("BUILDKIT_ADDR is not set")
	}

	placeholder, err := os.CreateTemp("/tmp", "hl-buildkit-agent-")
	require.NoError(t, err)
	sshSocket := placeholder.Name()
	require.NoError(t, placeholder.Close())
	require.NoError(t, os.Remove(sshSocket))
	listener, err := net.Listen("unix", sshSocket)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = listener.Close()
		_ = os.Remove(sshSocket)
	})

	dockerfileDir := t.TempDir()
	dockerfile := `FROM busybox:1.36.1-musl
RUN mkdir -p /root/.ssh
RUN --mount=type=ssh,required=true \
    --mount=type=secret,id=git_netrc,target=/root/.netrc,required=true \
    --mount=type=secret,id=git_known_hosts,target=/root/.ssh/known_hosts,required=true \
    test -S /run/buildkit/ssh_agent.0 && \
    grep -q "dummy-netrc-value" /root/.netrc && \
    grep -q "dummy-known-host-value" /root/.ssh/known_hosts
RUN test ! -e /root/.netrc && \
    test ! -e /root/.ssh/known_hosts && \
    test ! -e /run/buildkit/ssh_agent.0
`
	require.NoError(t, os.WriteFile(filepath.Join(dockerfileDir, "Dockerfile"), []byte(dockerfile), 0600))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	err = BuildDockerImageWithBuildKit(
		ctx,
		dockerfileDir,
		[]string{"heighliner-buildkit-auth-integration:session-mounts"},
		false,
		"",
		map[string]string{"GIT_AUTH_MODE": "ssh"},
		BuildKitOptions{
			Address:          buildKitAddress,
			Platform:         "linux/amd64",
			NoCache:          true,
			LogBuildProgress: "plain",
			Session: BuildKitSessionOptions{
				SSHAgentSocket: sshSocket,
				Secrets: map[string][]byte{
					GitNetrcSecretID:      []byte("dummy-netrc-value"),
					GitKnownHostsSecretID: []byte("dummy-known-host-value"),
				},
			},
		},
	)
	require.NoError(t, err)
}
