FROM golang:alpine AS build-env

RUN apk add --update --no-cache curl make git libc-dev bash gcc linux-headers eudev-dev

ARG TARGETARCH
ARG BUILDARCH

RUN wget -O /lib/libwasmvm_muslc.a https://github.com/CosmWasm/wasmvm/releases/download/v1.0.0-beta10/libwasmvm_muslc.$(uname -m).a

RUN git clone https://github.com/tendermint/tendermint; \
  cd tendermint; \
  git checkout remotes/origin/callum/app-version; \
  go install ./...

ARG GITHUB_ORGANIZATION
WORKDIR /go/src/github.com/${GITHUB_ORGANIZATION}

ARG GITHUB_REPO
ARG VERSION

RUN git clone https://github.com/${GITHUB_ORGANIZATION}/${GITHUB_REPO}.git
WORKDIR /go/src/github.com/${GITHUB_ORGANIZATION}/${GITHUB_REPO}
RUN git checkout ${VERSION}

ARG BUILD_TARGET
ARG BUILD_ENV
ARG BUILD_TAGS
ARG PRE_BUILD
ARG BUILD_DIR

RUN [ ! -z "$PRE_BUILD" ] && sh -c "${PRE_BUILD}"; \
    [ ! -z "$BUILD_ENV" ] && export ${BUILD_ENV}; \
    [ ! -z "$BUILD_TAGS" ] && export "${BUILD_TAGS}"; \
    [ ! -z "$BUILD_DIR" ] && cd "${BUILD_DIR}"; \
    make ${BUILD_TARGET}

RUN mkdir /root/bin
ARG BINARIES
ENV BINARIES_ENV ${BINARIES}
RUN bash -c 'BINARIES_ARR=($BINARIES_ENV); for BINARY in "${BINARIES_ARR[@]}"; do cp $BINARY /root/bin/ ; done'

FROM alpine:edge

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

RUN apk add --no-cache ca-certificates jq curl git gcc nano lz4 wget unzip

# Install tendermint
COPY --from=build-env /go/bin/tendermint /usr/bin/

# Install chain binaries
COPY --from=build-env /root/bin /usr/local/bin

# Install libraries
COPY --from=build-env /usr/local/lib /usr/local/lib

RUN addgroup -S heighliner && adduser -S heighliner -G heighliner
WORKDIR /home/heighliner
USER heighliner
