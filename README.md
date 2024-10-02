
🌌 Why use Heighliner?
=============================

![Heighliner photo](https://static.wikia.nocookie.net/dune/images/7/72/51mMK0akBOL._AC_SY400_-1.jpg/revision/latest)

We built Heighliner to streamline the management and building of production-grade container images for blockchain node software, Cosmos and beyond.

🌌🌌 Who benefits from Heighliner?
=============================

Heighliner container images are useful for anyone who's responsible for infrastructure deployments like

- validator + [horcrux](https://github.com/strangelove-ventures/horcrux),
- full nodes,
- archive nodes,
- end-to-end testing ([interchaintest](https://github.com/strangelove-ventures/interchaintest)),
- and more.

🌌🌌🌌 What does Heighliner do?
=============================

Heighliner images are minimally constructed from scratch images, packaging only the chain binary and a useful reduced set of busybox utilities.

This repository has a CI/CD pipeline to automatically build images when new git tags are detected on the chain repos in [chains/](chains). These images are hosted as packages in the Github Container Registry (ghcr) [here](https://github.com/orgs/strangelove-ventures/packages?repo_name=heighliner)



🌌🌌🌌🌌 How do I use Heighliner?
=============================

## Add a New Chain

To add a chain to the heighliner built-in configuration and have your chain images available on our repository's [ghcr](https://github.com/orgs/strangelove-ventures/packages?repo_name=heighliner), submit a PR adding it to the [chains directory](chains) so it will be included in the automatic builds.

For further instructions see: [addChain.md](./addChain.md)

## Add Heighliner Images to your Chain Repo

To publish heighliner images on your chain repository and/or incorporate heighliner images into your chain repo's CI/CD flow, see [heighliner-build-action](https://github.com/strangelove-ventures/heighliner-build-action)

## Build Your Own

If you would like to build the images yourself, heighliner is a CLI tool to help you do so.
Download the latest [release](https://github.com/strangelove-ventures/heighliner/releases), or build it yourself with:

```shell
make build
```

## Build and put binary in path. 

If you would like to build an image and put it in your local path , you can do the following.

```shell
make install
```



#### Example: build the docker image for gaia v6.0.0:

```shell
heighliner build --chain gaia --git-ref v6.0.0
```

Docker image `heighliner/gaia:v6.0.0` will now be available in your local docker images

#### Example: Cosmos SDK chain development cycle, build a local repository

```shell
cd ~/gaia-fork
heighliner build -c gaia --local
```

Docker image `gaia:local` will be built and stored in your local docker images.

#### Example: Build from a Github fork

```shell
cd ~/gaia-fork
heighliner build -c gaia -o strangelove-ventures -g working_branch -t image_tag
```

Heighliner will build the `working_branch` branch from the `strangelove-ventures/gaia` github organization/repository. Docker image `gaia:image_tag` will be stored in your local docker images.

#### Example: build with all overrides.

Heighliner can build without a chain present in chains.yaml if the necessary flags are provided.

```shell
heighliner build -c somegaia -o somefork --repo gaia --dockerfile cosmos --build-target "make install" --build-env "LEDGER_ENABLED=false BUILD_TAGS=muslc" --binaries "/go/bin/gaiad" -g v8.0.0 -t v8.0.0-somefork
```

Docker image `somegaia:v8.0.0-somefork` will be built and stored in your local docker images.

#### Example: build and push the gaia v6.0.0 docker image to ghcr:

```shell
# docker login ...
heighliner build -r ghcr.io/strangelove-ventures/heighliner -c gaia -g v6.0.0
```

Docker image `ghcr.io/strangelove-ventures/heighliner/gaia:v6.0.0` will be built and pushed to ghcr.io

#### Example: build and push last n releases of osmosis chain

```shell
# docker login ...
heighliner build -r ghcr.io/strangelove-ventures/heighliner -c osmosis -n 3
```

heighliner will fetch the last 3 osmosis release tags from github, build docker images, and push them, e.g.:
- `ghcr.io/strangelove-ventures/heighliner/osmosis:v6.1.0`
- `ghcr.io/strangelove-ventures/heighliner/osmosis:v6.0.0`
- `ghcr.io/strangelove-ventures/heighliner/osmosis:v5.0.0`

#### Example: build and push last n releases of all chains

This will make a request to each chain's Github repository to fetch all recent releases. Github rate-limits unauthenticated requests to 60 requests per hour. Authenticated requests have either 1000 (personal) or 15000 (enterprise) per hour. To add Github API authentication, set the `GH_USER` and `GH_PAT` environment variables with your Github username and Github Personal Access Token (PAT), respectively.

```shell
# docker login ...
export GH_USER=github_username GH_PAT=github_personal_access_token
heighliner build -r ghcr.io/strangelove-ventures/heighliner -n 3
```

heighliner will fetch the last 3 release tags from github for all chains in [chains](chains), build docker images, and push them.



🌌🌌🌌🌌🌌 Extras
=============================

## Cross compiling
Depends on docker [buildkit](https://github.com/moby/buildkit). Requires `buildkitd` server to be running.
Pass `-b` flag to use buildkit.

The build will look for the local buildkit unix socket by default. Change address with `--buildkit-addr` flag.

Customize the platform(s) to be built with the `--platform` flag.

#### Example: build x64 and arm64 docker images for gaia v7.0.1:

```shell
heighliner build -b -c gaia -g v7.0.1
```

Docker images for `heighliner/gaia:v7.0.1` will now be available in your local docker. The manifest for the tag will contain both amd64 and arm64 images.

#### Example: Use custom buildkit server, build x64 and arm64 docker images for gaia v7.0.1, and push:

```shell
heighliner build -b --buildkit-addr tcp://192.168.1.5:8125 -c gaia -g v7.0.1 -r ghcr.io/strangelove-ventures/heighliner
```

Docker images for `heighliner/gaia:v7.0.1` will be built on the remote buildkit server and then pushed to the container repository. The manifest for the tag will contain both amd64 and arm64 images.

