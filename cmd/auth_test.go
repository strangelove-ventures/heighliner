package cmd

import (
	"strings"
	"testing"

	"github.com/strangelove-ventures/heighliner/builder"
	"github.com/stretchr/testify/require"
)

func TestBuildCommandRegistersPrivateGitAuthenticationFlags(t *testing.T) {
	command := BuildCmd()
	for _, flag := range []string{flagSSH, flagSSHKnownHosts, flagGitHubAuth, flagCloneKeyEnv, flagCloneKey} {
		require.NotNilf(t, command.PersistentFlags().Lookup(flag), "missing --%s", flag)
	}
	require.Nil(t, command.PersistentFlags().Lookup("github-token-env"))
}

func TestAuthenticationValidationRunsBeforeQueueing(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	originalChains := chains
	chains = []builder.ChainNodeConfig{{
		Name:       "private-chain",
		Dockerfile: builder.DockerfileTypeCosmos,
	}}
	t.Cleanup(func() { chains = originalChains })

	var warnings strings.Builder
	err := validateAuthenticationBeforeQueue(builder.HeighlinerDockerBuildConfig{
		UseBuildKit:   true,
		UseGitHubAuth: true,
	}, chainConfigFlags{chain: "private-chain", ref: "private-ref"}, &warnings)
	require.ErrorContains(t, err, "requires GH_TOKEN or GITHUB_TOKEN")
	require.Empty(t, warnings.String())
}
