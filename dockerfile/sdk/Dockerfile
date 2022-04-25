FROM golang:alpine AS build-env

# Can't do this because of libwasm_muslc.a static library necessary for target arch. 
# Would be great to just have go cross compile the go binaries using host arch, but not possible because of the static muslc lib
#FROM --platform=$BUILDPLATFORM golang:alpine AS build-env

ARG VERSION
ARG NAME
ARG GITHUB_ORGANIZATION
ARG GITHUB_REPO
ARG BINARIES
ARG BUILD_TARGET
ARG BUILD_ENV
ARG BUILD_TAGS
ARG PRE_BUILD

RUN apk add --no-cache curl make git libc-dev bash gcc linux-headers eudev-dev

ARG TARGETARCH

RUN git clone https://github.com/tendermint/tendermint && \
  cd tendermint && \
  git checkout remotes/origin/callum/app-version && \
  GOARCH=$TARGETARCH go install ./...

RUN wget -O /lib/libwasmvm_muslc.a https://github.com/CosmWasm/wasmvm/releases/download/v1.0.0-beta10/libwasmvm_muslc.$(uname -m).a

WORKDIR /go/src/github.com/${GITHUB_ORGANIZATION}

RUN git clone https://github.com/${GITHUB_ORGANIZATION}/${GITHUB_REPO}.git

WORKDIR /go/src/github.com/${GITHUB_ORGANIZATION}/${GITHUB_REPO}

RUN git checkout ${VERSION}

RUN if [ ! -z "$PRE_BUILD" ]; then sh -c "${PRE_BUILD}"; fi; \
    if [ ! -z "$BUILD_ENV" ]; then export ${BUILD_ENV}; fi; \
    if [ ! -z "$BUILD_TAGS" ]; then export "${BUILD_TAGS}"; fi; \
    export GOARCH=$TARGETARCH; \
    make ${BUILD_TARGET}

RUN mkdir /root/bin
ENV BINARIES_ENV ${BINARIES}
RUN bash -c 'BINARIES_ARR=($BINARIES_ENV); for BINARY in "${BINARIES_ARR[@]}"; do cp $BINARY /root/bin/ ; done'

FROM alpine:edge

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

RUN apk add --no-cache ca-certificates jq curl git gcc
WORKDIR /root

# Install tendermint
COPY --from=build-env /go/bin/tendermint /usr/bin/

# Install chain binaries
COPY --from=build-env /root/bin /usr/local/bin

EXPOSE 26657
