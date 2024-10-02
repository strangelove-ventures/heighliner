package main

import (
	_ "embed"

	"github.com/strangelove-ventures/heighliner/cmd"
)

//go:embed chains/01_chains.yaml
var chainsYaml []byte

func main() {
	cmd.Execute(chainsYaml)
}
