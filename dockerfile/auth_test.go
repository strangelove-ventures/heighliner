package dockerfile

import (
	"bytes"
	"strings"
	"testing"

	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/stretchr/testify/require"
)

func TestCosmosBuildKitDockerfilesUseSessionMounts(t *testing.T) {
	for name, dockerfile := range map[string][]byte{
		"remote":      Cosmos,
		"local-cross": CosmosLocalCross,
	} {
		t.Run(name, func(t *testing.T) {
			contents := string(dockerfile)
			_, err := parser.Parse(bytes.NewReader(dockerfile))
			require.NoError(t, err)
			require.Contains(t, contents, "ARG GIT_AUTH_MODE=none")
			require.Contains(t, contents, "--mount=type=ssh")
			require.Contains(t, contents, "id=git_known_hosts")
			require.Contains(t, contents, "id=git_netrc")
			require.Contains(t, contents, "StrictHostKeyChecking=yes")
			require.NotContains(t, contents, "ARG CLONE_KEY")
			require.NotContains(t, contents, "base64 -d")
			require.NotContains(t, contents, "id_ed25519")
		})
	}

	require.GreaterOrEqual(t, strings.Count(string(Cosmos), "--mount=type=ssh"), 2, "remote clone and build must both mount SSH")
}
