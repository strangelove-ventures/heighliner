package builder

const (
	Go118Version = "1.18.10"
	Go119Version = "1.19.6"
	Go120Version = "1.20.1"

	AlpineImageVersion = "3.17"

	// golang official dockerhub images to use for cosmos builds
	Go118Image = Go118Version + "-alpine" + AlpineImageVersion
	Go119Image = Go119Version + "-alpine" + AlpineImageVersion
	Go120Image = Go120Version + "-alpine" + AlpineImageVersion

	GoDefaultImage = Go120Image // default image for cosmos go builds if go.mod parse fails
)

// Map of go version to the builder image
var GoVersionsDesc = map[string]string{
	Go120Version: Go120Image,
	Go119Version: Go119Image,
	Go118Version: Go118Image,
}
