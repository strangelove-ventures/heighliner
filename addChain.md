# Adding a New Chain

To add a new chain, add relevant chain info to `chains.yaml` file.

Please keep chains in alphabetical order.


## Fields:

`name` -> The name used to direct heighliner on what chain to build. Must be lowercase.

`repo-host` -> By default, this is "github.com", but use this field to override. For example "gitlab.com"

`github-organization` -> The organization name of the location of the chain binary.

`github-repo` -> The repo name of the location of the chain binary.

`language` -> Used in the Docker `FROM` argument to create a a docker base image. OPTIONS: "go, rust, nix, imported". Use "imported" if you are not able to build the chain binary from source and are importing a pre-made docker container.

`build-env` -> Environment variables to be created during the build.

`pre-build` -> Any extra arguments needed to build the chain binary. 

`build-target` -> The argument to call after `make` (language=golang),  `cargo` (language=rust) or `nix` (language=nix).

`binaries` -> The location of where the the build target places the binarie(s). Adding a ":" after the path allows for the ability to rename the binary.

`libraries` -> Any extra libraries need to run the binary. In additon to the binary itself, these will be copied over to the final image


## Verify Build:


Please check the image builds successfully before submitting PR:

`./heighliner build -c <CHAIN-NAME> -v <VERSION>

Ensure binary runs in image:

`docker run -it --rm --entrypoint sh <name>:<tag>`