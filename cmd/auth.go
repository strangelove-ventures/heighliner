package cmd

import (
	"fmt"
	"io"

	"github.com/strangelove-ventures/heighliner/builder"
)

func validateAuthenticationBeforeQueue(
	buildConfig builder.HeighlinerDockerBuildConfig,
	flags chainConfigFlags,
	warnings io.Writer,
) error {
	var selected []builder.ChainNodeConfig
	for _, chain := range chains {
		if flags.chain != "" && chain.Name != flags.chain {
			continue
		}
		selected = append(selected, applyChainOverrides(chain, flags))
		if flags.ref != "" || flags.local {
			break
		}
	}

	if len(selected) == 0 {
		selected = append(selected, applyChainOverrides(builder.ChainNodeConfig{Name: flags.chain}, flags))
	}

	for _, chain := range selected {
		if _, err := builder.ResolveGitAuth(buildConfig, chain); err != nil {
			return fmt.Errorf("invalid private Git authentication for chain %q: %w", chain.Name, err)
		}
		for _, warning := range builder.CloneKeyDeprecationWarnings(buildConfig, chain) {
			if _, err := fmt.Fprintln(warnings, warning); err != nil {
				return fmt.Errorf("write clone-key deprecation warning: %w", err)
			}
		}
	}
	return nil
}
