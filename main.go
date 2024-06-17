package main

import (
	_ "embed"

	"github.com/p2p-org/heighliner/cmd"
)

//go:embed chains.yaml
var chainsYaml []byte

func main() {
	cmd.Execute(chainsYaml)
}
