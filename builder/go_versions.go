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
	Go121Version = "1.21.13"
	Go122Version = "1.22.12"
	Go123Version = "1.23.12"
	Go124Version = "1.24.13"
	Go125Version = "1.25.7"
	// ADD NEW GO VERSION [1] - latest patch release for each major/minor

	// When updating alpine image, ensure all golang build image combinations below exist
	LatestAlpineImageVersion = "3.23"
)

var (
	// ADD NEW GO VERSION [3] - update GoDefaultVersion to latest; GoDefaultImage is set from map in init()
	GoDefaultVersion = Go125Version
	GoDefaultImage   string // default image for cosmos go builds if go.mod parse fails
)

func GolangAlpineImage(goVersion, alpineVersion string) string {
	return goVersion + "-alpine" + alpineVersion
}

type GoVersion struct {
	Version       string
	Image         string
	AlpineVersion string // Alpine version for this Go major.minor (e.g. "3.22")
}

// withImage returns a copy with Image set from Version+AlpineVersion if Image is empty.
func (v GoVersion) withImage() GoVersion {
	if v.Image == "" && v.AlpineVersion != "" {
		v.Image = GolangAlpineImage(v.Version, v.AlpineVersion)
	}
	return v
}

// GoImageForVersion maps major.minor (e.g. "1.23") to version info. Store Version+AlpineVersion only; use withImage() when Image is needed.
var GoImageForVersion = map[string]GoVersion{
	"1.18": {Version: Go118Version, AlpineVersion: "3.17"},
	"1.19": {Version: Go119Version, AlpineVersion: "3.18"},
	"1.20": {Version: Go120Version, AlpineVersion: "3.19"},
	"1.21": {Version: Go121Version, AlpineVersion: "3.20"},
	"1.22": {Version: Go122Version, AlpineVersion: "3.21"},
	"1.23": {Version: Go123Version, AlpineVersion: "3.22"},
	"1.24": {Version: Go124Version, AlpineVersion: LatestAlpineImageVersion},
	"1.25": {Version: Go125Version, AlpineVersion: LatestAlpineImageVersion},
	// ADD NEW GO VERSION [4]
}

func init() {
	GoDefaultImage = GoImageForVersion["1.25"].withImage().Image
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
	// If alpine version is not provided, but go version is provided to the patch version (e.g. 1.23.10),
	// use the Alpine version for that major.minor from the map so the image exists on Docker Hub.
	if len(strings.Split(goVersion, ".")) == 3 {
		alpineVer := LatestAlpineImageVersion
		for _, goVer := range goVersionsDesc() {
			if semver.Compare("v"+goVersion, "v"+goVer) >= 0 {
				if entry := GoImageForVersion[goVer]; entry.AlpineVersion != "" {
					alpineVer = entry.AlpineVersion
				}
				break
			}
		}
		return GoVersion{Version: goVersion, Image: GolangAlpineImage(goVersion, alpineVer)}
	}
	// If alpine version is not provided, and go version is not provided to the patch version, use the latest alpine version with the latest go patch version.
	for _, goVer := range goVersionsDesc() {
		if semver.Compare("v"+goVersion, "v"+goVer) >= 0 {
			return GoImageForVersion[goVer].withImage()
		}
	}
	// If unable to find go version in mapping, return default
	return GoVersion{Version: GoDefaultVersion, Image: GoDefaultImage, AlpineVersion: LatestAlpineImageVersion}
}
