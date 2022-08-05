# Adding a New Chain

To add a new chain, add relevant chain info to `chains.yaml` file.

Please keep chains in alphabetical order.


## Fields:

`name` -> The name used to direct heighliner on what chain to build. Must be lowercase.

`language` -> Used in the Docker `FROM` argument to create a a docker base image. OPTIONS: "go, rust, imported". Use "imported" if you are not able to build the chain binary from source and are importing a pre-made docker container.

`repo-host` -> By default, this is "github.com", but use this field to override. For example "gitlab.com"

`github-organization` -> The organization name of the location of the chain binary.

`github-repo` -> The repo name of the location of the chain binary.

`build-target` -> The argument to call after `make`.

`binaries` -> The location of where the the build target places the binarie(s).

`pre-build` -> Any extra arguments needed to build the chain binary. 

`build-env` -> 


Please check your addition builds successfully before submitting PR:

`./heighliner build -c <CHAIN-NAME> -v <VERSION>