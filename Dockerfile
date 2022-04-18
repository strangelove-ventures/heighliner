FROM golang:alpine AS build-env
ARG VERSION
ARG NAME
ARG GITHUB_ORGANIZATION
ARG GITHUB_REPO
ARG BINARY
ARG MAKE_TARGET
ARG BUILD_ENV
ARG BUILD_TAGS
ARG PRE_BUILD

ENV PACKAGES curl make git libc-dev bash gcc linux-headers eudev-dev

RUN apk add --no-cache $PACKAGES

WORKDIR /go/src/github.com/${GITHUB_ORGANIZATION}

RUN git clone https://github.com/${GITHUB_ORGANIZATION}/${GITHUB_REPO}.git

WORKDIR /go/src/github.com/${GITHUB_ORGANIZATION}/${GITHUB_REPO}

ADD https://github.com/CosmWasm/wasmvm/releases/download/v1.0.0-beta10/libwasmvm_muslc.x86_64.a /lib/libwasmvm_muslc.a

RUN git checkout ${VERSION}

RUN if [ ! -z "$PRE_BUILD" ]; then sh -c "${PRE_BUILD}"; fi; \
    if [ ! -z "$BUILD_ENV" ]; then export ${BUILD_ENV}; fi; \
    if [ ! -z "$BUILD_TAGS" ]; then export "${BUILD_TAGS}"; fi; \
    make ${MAKE_TARGET}

RUN cp ${BINARY} /root/cosmos

RUN git clone https://github.com/tendermint/tendermint && \
  cd tendermint && \
  git checkout remotes/origin/callum/app-version && \
  go install ./...

FROM alpine:edge

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

ARG BINARY
ENV BINARY ${BINARY}

RUN apk add --no-cache ca-certificates jq curl git gcc
WORKDIR /root

# Install tendermint
COPY --from=build-env /go/bin/tendermint /usr/bin/

# Install chain binary
COPY --from=build-env /root/cosmos .
RUN mv /root/cosmos /usr/bin/$(basename $BINARY)

EXPOSE 26657

CMD env $(basename $BINARY) start
