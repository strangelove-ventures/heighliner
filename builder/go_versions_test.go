package builder_test

import (
	"testing"

	"github.com/strangelove-ventures/heighliner/builder"
	"github.com/stretchr/testify/require"
)

func TestGoVersions(t *testing.T) {
	goVer := builder.GetImageAndVersionForGoVersion("1.18")
	require.Equal(t, builder.Go118Image, goVer.Image)
	require.Equal(t, builder.Go118Version, goVer.Version)

	goVer = builder.GetImageAndVersionForGoVersion("1.19")
	require.Equal(t, builder.Go119Image, goVer.Image)
	require.Equal(t, builder.Go119Version, goVer.Version)

	goVer = builder.GetImageAndVersionForGoVersion("1.20")
	require.Equal(t, builder.Go120Image, goVer.Image)
	require.Equal(t, builder.Go120Version, goVer.Version)

	goVer = builder.GetImageAndVersionForGoVersion("1.21")
	require.Equal(t, builder.Go121Image, goVer.Image)
	require.Equal(t, builder.Go121Version, goVer.Version)

	goVer = builder.GetImageAndVersionForGoVersion("unknown")
	require.Equal(t, builder.GoDefaultImage, goVer.Image)
	require.Equal(t, builder.GoDefaultVersion, goVer.Version)
}
