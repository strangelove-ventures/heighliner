FROM --platform=$BUILDPLATFORM golang:alpine AS build-env

RUN apk add --update --no-cache curl make git libc-dev bash gcc linux-headers eudev-dev

ARG TARGETARCH
ARG BUILDARCH

RUN LIBDIR=/lib; \
    if [ "${TARGETARCH}" = "arm64" ]; then \
      ARCH=aarch64; \
      if [ "${BUILDARCH}" != "arm64" ]; then \
        wget -c https://musl.cc/aarch64-linux-musl-cross.tgz -O - | tar -xzvv --strip-components 1 -C /usr; \
        LIBDIR=/usr/aarch64-linux-musl/lib; \
        mkdir $LIBDIR; \
      fi; \
    elif [ "${TARGETARCH}" = "amd64" ]; then \
      ARCH=x86_64; \
      if [ "${BUILDARCH}" != "amd64" ]; then \
        wget -c https://musl.cc/x86_64-linux-musl-cross.tgz -O - | tar -xzvv --strip-components 1 -C /usr; \
        LIBDIR=/usr/x86_64-linux-musl/lib; \
        mkdir $LIBDIR; \
      fi; \
    fi; \
    wget -O $LIBDIR/libwasmvm_muslc.a https://github.com/CosmWasm/wasmvm/releases/download/v1.0.0-beta10/libwasmvm_muslc.$ARCH.a

RUN git clone https://github.com/tendermint/tendermint; \
  cd tendermint; \
  git checkout remotes/origin/callum/app-version; \
  if [ "${TARGETARCH}" = "arm64" ] && [ "${BUILDARCH}" != "arm64" ]; then \
    export CGO_ENABLED=1 CC=aarch64-linux-musl-gcc; \
  elif [ "${TARGETARCH}" = "amd64" ] && [ "${BUILDARCH}" != "amd64" ]; then \
    export CGO_ENABLED=1 CC=x86_64-linux-musl-gcc; \
  fi; \
  export GOOS=linux GOARCH=$TARGETARCH; \
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

RUN if [ "${TARGETARCH}" = "arm64" ] && [ "${BUILDARCH}" != "arm64" ]; then \
      export CGO_ENABLED=1 CC=aarch64-linux-musl-gcc CXX=aarch64-linux-musl-g++;\
    elif [ "${TARGETARCH}" = "amd64" ] && [ "${BUILDARCH}" != "amd64" ]; then \
      export CGO_ENABLED=1 CC=x86_64-linux-musl-gcc CXX=x86_64-linux-musl-g++; \
    fi; \
    export GOOS=linux GOARCH=$TARGETARCH; \
    [ ! -z "$PRE_BUILD" ] && sh -c "${PRE_BUILD}"; \
    [ ! -z "$BUILD_ENV" ] && export ${BUILD_ENV}; \
    [ ! -z "$BUILD_TAGS" ] && export "${BUILD_TAGS}"; \
    [ ! -z "$BUILD_DIR" ] && cd "${BUILD_DIR}"; \
    make ${BUILD_TARGET}

RUN [ -d "/go/bin/linux_${TARGETARCH}" ] && mv /go/bin/linux_${TARGETARCH}/* /go/bin/ || true

RUN mkdir /root/bin
ARG BINARIES
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

# Install libraries
COPY --from=build-env /usr/local/lib /usr/local/lib

EXPOSE 26657
