package main

import (
	_ "embed"

	"github.com/strangelove-ventures/heighliner/cmd"
)

//go:chains chains.yaml
var chainsYaml []byte

func main() {
	cmd.Execute(chainsYaml)
}
