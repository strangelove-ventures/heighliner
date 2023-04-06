package builder

type DockerfileType string

const (
	DockerfileTypeCosmos   DockerfileType = "cosmos"
	DockerfileTypeCargo    DockerfileType = "cargo"
	DockerfileTypeImported DockerfileType = "imported"

	DockerfileTypeGo   DockerfileType = "go"   // DEPRECATED, use "cosmos" instead
	DockerfileTypeRust DockerfileType = "rust" // DEPRECATED, use "cargo" instead
)

// The first values for `dockerfile` are deprecated. Their recommended replacement is the second value.
var deprecationReplacements = [][2]DockerfileType{
	{DockerfileTypeGo, DockerfileTypeCosmos},
	{DockerfileTypeRust, DockerfileTypeCargo},
}

type ChainNodeConfig struct {
	Name               string         `yaml:"name"`
	RepoHost           string         `yaml:"repo-host"`
	GithubOrganization string         `yaml:"github-organization"`
	GithubRepo         string         `yaml:"github-repo"`
	Language           DockerfileType `yaml:"language"` // DEPRECATED, use "dockerfile" instead
	Dockerfile         DockerfileType `yaml:"dockerfile"`
	BuildTarget        string         `yaml:"build-target"`
	BuildDir           string         `yaml:"build-dir"`
	Binaries           []string       `yaml:"binaries"`
	Libraries          []string       `yaml:"libraries"`
	TargetLibraries    []string       `yaml:"target-libraries"`
	PreBuild           string         `yaml:"pre-build"`
	Platforms          []string       `yaml:"platforms"`
	BuildEnv           []string       `yaml:"build-env"`
	BaseImage          string         `yaml:"base-image"`
}

type ChainNodeDockerBuildConfig struct {
	Build  ChainNodeConfig
	Ref    string
	Tag    string
	Latest bool
}

type HeighlinerDockerBuildConfig struct {
	ContainerRegistry string
	SkipPush          bool
	UseBuildKit       bool
	BuildKitAddr      string
	Platform          string
	NoCache           bool
	NoBuildCache      bool
}

type HeighlinerQueuedChainBuilds struct {
	ChainConfigs []ChainNodeDockerBuildConfig
}
