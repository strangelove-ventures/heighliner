# Adding a New Chain

To add a new chain, add relevant chain info to `chains.yaml` file.

Please keep chains in alphabetical order.


## Fields:

`name` -> The name used to direct heighliner on what chain to build. Must be lowercase.

`repo-host` -> By default, this is "github.com", but use this field to override. For example "gitlab.com"

`github-organization` -> The organization name of the location of the chain binary.

`github-repo` -> The repo name of the location of the chain binary.

`dockerfile` -> Which dockerfile strategy to use (folder names under dockerfile/). OPTIONS: `cosmos`, `cargo`, `imported`, or `none`. Use `imported` if you are importing an existing public docker image as a base for the heighliner image. Use `none` if you are not able to build the chain binary from source and need to download binaries into the image instead.

`build-env` -> Environment variables to be created during the build.

`pre-build` -> Any extra arguments needed to build the chain binary. 

`build-target` -> The build command specific to the chosen `dockerfile`. For `cosmos`, likely `make install`. For `cargo`, likely `build --release`.

`binaries` -> The location of the binary(ies) in the build environment after the build is complete. Adding a ":" after the path allows for the ability to rename the binary.

`libraries` -> Any extra libraries from the build environment needed in the final image.

`target-libraries` -> Any extra libraries from the target image needed in the final image (cargo Dockerfiles only).


## Verify Build:


Please check the image builds successfully before submitting PR:

`./heighliner build -c <CHAIN-NAME> -g <BRANCH OR TAG>`

Ensure binary runs in image:

`docker run -it --rm --entrypoint sh <name>:<tag>`