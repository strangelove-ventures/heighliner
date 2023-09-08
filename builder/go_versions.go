package builder

import (
	"sort"

	"golang.org/x/mod/semver"
)

// To add a new go version, add to `ADD NEW GO VERSION` [1], [2], [3], and [4]

const (
	Go118Version = "1.18.10"
	Go119Version = "1.19.12"
	Go120Version = "1.20.7"
	Go121Version = "1.21.0"
	// ADD NEW GO VERSION [1] - latest patch release for each major/minor

	// When updating alpine image, ensure all golang build image combinations below exist
	AlpineImageVersion = "3.17"

	// golang official dockerhub images to use for cosmos builds
	// Find from https://hub.docker.com/_/golang
	Go118Image = Go118Version + "-alpine" + AlpineImageVersion
	Go119Image = Go119Version + "-alpine" + AlpineImageVersion
	Go120Image = Go120Version + "-alpine" + AlpineImageVersion
	Go121Image = Go121Version + "-alpine" + AlpineImageVersion
	// ADD NEW GO VERSION [2]

	// ADD NEW GO VERSION [3] - update GoDefaultVersion and GoDefaultImage to latest
	GoDefaultVersion = Go121Version
	GoDefaultImage   = Go121Image // default image for cosmos go builds if go.mod parse fails
)

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
func GetImageAndVersionForGoVersion(goVersion string) GoVersion {
	for _, goVer := range goVersionsDesc() {
		if semver.Compare("v"+goVersion, "v"+goVer) >= 0 {
			return GoImageForVersion[goVer]
		}
	}
	return GoVersion{Version: GoDefaultVersion, Image: GoDefaultImage}
}
