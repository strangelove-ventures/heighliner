# Heighliner

Heighliner is a repository of docker images for the node software of Cosmos chains

![Heighliner photo](https://static.wikia.nocookie.net/dune/images/7/72/51mMK0akBOL._AC_SY400_-1.jpg/revision/latest)

## Docker Images

The images are available as packages in the Github Container Registry (ghcr) [here](https://github.com/orgs/strangelove-ventures/packages?tab=packages&q=heighliner)

This repository checks for new tags in the chains in [chains.yaml](./chains.yaml) daily and builds new images if necessary.

## Build Your Own

If you would like to build the images yourself, heighliner is a CLI tool to help you do so.
Download the latest [release](https://github.com/strangelove-ventures/heighliner/releases), or build it yourself with:

```bash
go build
```

#### Example: build the docker image for gaia v6.0.0:

```bash
heighliner build -c gaia -v v6.0.0
```

Docker image `heighliner/gaia:v6.0.0` will now be available in your local docker images

#### Example: build and push the gaia v6.0.0 docker image to ghcr:

```bash
export DOCKER_USER=github_user
export DOCKER_PASSWORD=github_personal_access_token
heighliner build -r ghcr.io/strangelove-ventures/heighliner -c gaia -v v6.0.0
```

Docker image `ghcr.io/strangelove-ventures/heighliner/gaia:v6.0.0` will be built and pushed to ghcr.io

#### Example: build and push last n releases of osmosis chain

```bash
export DOCKER_USER=github_user
export DOCKER_PASSWORD=github_personal_access_token
heighliner build -r ghcr.io/strangelove-ventures/heighliner -c osmosis -n 3
```

heighliner will fetch the last 3 osmosis release tags from github, build docker images, and push them, e.g.:
- `ghcr.io/strangelove-ventures/heighliner/osmosis:v6.1.0`
- `ghcr.io/strangelove-ventures/heighliner/osmosis:v6.0.0`
- `ghcr.io/strangelove-ventures/heighliner/osmosis:v5.0.0`

#### Example: build and push last n releases of all chains

```bash
export DOCKER_USER=github_user
export DOCKER_PASSWORD=github_personal_access_token
heighliner build -r ghcr.io/strangelove-ventures/heighliner -n 3
```

heighliner will fetch the last 3 release tags from github for all chains in [chains.yaml](./chains.yaml), build docker images, and push them.

## Add a new chain

To include a Cosmos based blockchain that does not yet have images, submit a PR adding it to [chains.yaml](./chains.yaml) so it will be included in the daily builds.