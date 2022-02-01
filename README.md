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

Example: build the docker image for gaia v6.0.0:

```bash
heighliner build -r test -c gaia -v v6.0.0
```

## Add a new chain

To include a Cosmos based blockchain that does not yet have images, submit a PR adding it to [chains.yaml](./chains.yaml) so it will be included in the daily builds.