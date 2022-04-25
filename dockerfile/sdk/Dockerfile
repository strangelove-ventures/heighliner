FROM --platform=$BUILDPLATFORM golang:alpine AS build-env

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
ARG TARGETOS

RUN git clone https://github.com/tendermint/tendermint && \
  cd tendermint && \
  git checkout remotes/origin/callum/app-version && \
  GOARCH=$TARGETARCH go install ./...

WORKDIR /go/src/github.com/${GITHUB_ORGANIZATION}

RUN git clone https://github.com/${GITHUB_ORGANIZATION}/${GITHUB_REPO}.git

WORKDIR /go/src/github.com/${GITHUB_ORGANIZATION}/${GITHUB_REPO}

RUN git checkout ${VERSION}

RUN [ ! -z "$PRE_BUILD" ] && sh -c "${PRE_BUILD}"; \
    [ ! -z "$BUILD_ENV" ] && export ${BUILD_ENV}; \
    [ ! -z "$BUILD_TAGS" ] && export "${BUILD_TAGS}"; \
    export GOARCH=$TARGETARCH; \
    make ${BUILD_TARGET}

RUN [ -d "/go/bin/${TARGETOS}_${TARGETARCH}" ] && mv /go/bin/${TARGETOS}_${TARGETARCH}/* /go/bin/ || true

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
