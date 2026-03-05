package builder_test

import (
	"testing"

	"github.com/strangelove-ventures/heighliner/builder"
	"github.com/stretchr/testify/require"
)

func TestGoVersions(t *testing.T) {
	goVer := builder.GetImageAndVersionForGoVersion("1.18", "")
	require.Equal(t, "1.18.10", goVer.Version)
	require.Equal(t, "1.18.10-alpine3.17", goVer.Image)
	require.Equal(t, "3.17", goVer.AlpineVersion)

	goVer = builder.GetImageAndVersionForGoVersion("1.19", "")
	require.Equal(t, "1.19.13", goVer.Version)
	require.Equal(t, "1.19.13-alpine3.18", goVer.Image)
	require.Equal(t, "3.18", goVer.AlpineVersion)

	goVer = builder.GetImageAndVersionForGoVersion("1.20", "")
	require.Equal(t, "1.20.14", goVer.Version)
	require.Equal(t, "1.20.14-alpine3.19", goVer.Image)
	require.Equal(t, "3.19", goVer.AlpineVersion)

	goVer = builder.GetImageAndVersionForGoVersion("1.21", "")
	require.Equal(t, "1.21.13", goVer.Version)
	require.Equal(t, "1.21.13-alpine3.20", goVer.Image)
	require.Equal(t, "3.20", goVer.AlpineVersion)

	goVer = builder.GetImageAndVersionForGoVersion("unknown", "")
	require.Equal(t, builder.GoDefaultVersion, goVer.Version)
	require.Equal(t, builder.GoDefaultImage, goVer.Image)
	require.Equal(t, builder.LatestAlpineImageVersion, goVer.AlpineVersion)

	goVer = builder.GetImageAndVersionForGoVersion("1.19.7", "3.17")
	require.Equal(t, "1.19.7", goVer.Version)
	require.Equal(t, "1.19.7-alpine3.17", goVer.Image)
	require.Equal(t, "", goVer.AlpineVersion)

	// Patch version without alpine: use major.minor's Alpine from map (1.19 -> 3.18)
	goVer = builder.GetImageAndVersionForGoVersion("1.19.10", "")
	require.Equal(t, "1.19.10", goVer.Version)
	require.Equal(t, "1.19.10-alpine3.18", goVer.Image)
	require.Equal(t, "", goVer.AlpineVersion)

	// Patch version 1.23.10 -> alpine3.22 (1.23's Alpine), not LatestAlpineImageVersion
	goVer = builder.GetImageAndVersionForGoVersion("1.23.10", "")
	require.Equal(t, "1.23.10", goVer.Version)
	require.Equal(t, "1.23.10-alpine3.22", goVer.Image)
	require.Equal(t, "", goVer.AlpineVersion)

	// Patch version 1.22.x and 1.24.x use their major.minor Alpine
	goVer = builder.GetImageAndVersionForGoVersion("1.22.5", "")
	require.Equal(t, "1.22.5", goVer.Version)
	require.Equal(t, "1.22.5-alpine3.21", goVer.Image)
	require.Equal(t, "", goVer.AlpineVersion)

	goVer = builder.GetImageAndVersionForGoVersion("1.24.1", "")
	require.Equal(t, "1.24.1", goVer.Version)
	require.Equal(t, "1.24.1-alpine3.23", goVer.Image)
	require.Equal(t, "", goVer.AlpineVersion)
}
