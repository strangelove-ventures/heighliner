package builder

import (
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

// To add a new go version, add to `ADD NEW GO VERSION` [1], [2], [3], and [4]
const (
	Go118Version = "1.18.10"
	Go119Version = "1.19.13"
	Go120Version = "1.20.14"
	Go121Version = "1.21.7"
	Go122Version = "1.22.0"
	// ADD NEW GO VERSION [1] - latest patch release for each major/minor

	// When updating alpine image, ensure all golang build image combinations below exist
	LatestAlpineImageVersion = "3.19"
)

var (
	// ADD NEW GO VERSION [2]
	// golang official dockerhub images to use for cosmos builds
	// Find from https://hub.docker.com/_/golang
	Go118Image = GolangAlpineImage(Go118Version, "3.17") // Go 1.18 is now deprecated, pinning to 3.17
	Go119Image = GolangAlpineImage(Go119Version, "3.18") // Go 1.19 is now deprecated, pinning to 3.18
	Go120Image = GolangAlpineImage(Go120Version, LatestAlpineImageVersion)
	Go121Image = GolangAlpineImage(Go121Version, LatestAlpineImageVersion)
	Go122Image = GolangAlpineImage(Go122Version, LatestAlpineImageVersion)

	// ADD NEW GO VERSION [3] - update GoDefaultVersion and GoDefaultImage to latest
	GoDefaultVersion = Go122Version
	GoDefaultImage   = Go122Image // default image for cosmos go builds if go.mod parse fails
)

func GolangAlpineImage(goVersion, alpineVersion string) string {
	return goVersion + "-alpine" + alpineVersion
}

type GoVersion struct {
	Version string
	Image   string
}

// GoImageForVersion is a map of go version to the builder image. Add new go versions here
var GoImageForVersion = map[string]GoVersion{
	"1.18": GoVersion{Version: Go118Version, Image: Go118Image},
	"1.19": GoVersion{Version: Go119Version, Image: Go119Image},
	"1.20": GoVersion{Version: Go120Version, Image: Go120Image},
	"1.21": GoVersion{Version: Go121Version, Image: Go121Image},
	"1.22": GoVersion{Version: Go122Version, Image: Go122Image},
	// ADD NEW GO VERSION [4]
}

// goVersionsDesc returns the go major versions in GoImageForVersion in descending order.
func goVersionsDesc() []string {
	goVersionsDesc := make([]string, len(GoImageForVersion))
	i := 0
	for goVer := range GoImageForVersion {
		goVersionsDesc[i] = goVer
		i++
	}
	sort.SliceStable(goVersionsDesc, func(i, j int) bool {
		return semver.Compare("v"+goVersionsDesc[i], "v"+goVersionsDesc[j]) >= 0
	})
	return goVersionsDesc
}

// GetImageForGoVersion will return the build docker image for the provided go version
func GetImageAndVersionForGoVersion(goVersion string, alpineVersion string) GoVersion {
	// If alpine version is provided, use that image explicitly
	if alpineVersion != "" {
		return GoVersion{Version: goVersion, Image: GolangAlpineImage(goVersion, alpineVersion)}
	}
	// If alpine version is not provided, but go version is provided to the patch version, use the latest alpine version with that go version.
	if len(strings.Split(goVersion, ".")) == 3 {
		return GoVersion{Version: goVersion, Image: GolangAlpineImage(goVersion, LatestAlpineImageVersion)}
	}
	// If alpine version is not provided, and go version is not provided to the patch version, use the latest alpine version with the latest go patch version.
	for _, goVer := range goVersionsDesc() {
		if semver.Compare("v"+goVersion, "v"+goVer) >= 0 {
			return GoImageForVersion[goVer]
		}
	}
	// If unable to find go version in mapping, return default
	return GoVersion{Version: GoDefaultVersion, Image: GoDefaultImage}
}
