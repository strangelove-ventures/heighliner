FROM --platform=$BUILDPLATFORM rust:latest AS build-env

RUN apt-get update && apt-get install -y clang libclang-dev

RUN curl https://sh.rustup.rs -sSf | sh -s -- --no-modify-path --default-toolchain none -y
RUN rustup component add rustfmt

WORKDIR /build

ARG GITHUB_ORGANIZATION
ARG GITHUB_REPO

RUN git clone https://github.com/${GITHUB_ORGANIZATION}/${GITHUB_REPO}.git

WORKDIR /build/${GITHUB_REPO}

ARG VERSION

RUN git checkout ${VERSION}

ARG BUILD_TARGET

RUN [ ! -z "$BUILD_TARGET" ] && cargo fetch || true

ARG TARGETARCH
ARG TARGETOS
ARG BUILDARCH

ARG BUILD_TARGET
ARG BUILD_ENV
ARG BUILD_TAGS
ARG PRE_BUILD

RUN [ ! -z "$PRE_BUILD" ] && sh -c "${PRE_BUILD}"; \
    [ ! -z "$BUILD_ENV" ] && export ${BUILD_ENV}; \
    [ ! -z "$BUILD_TAGS" ] && export "${BUILD_TAGS}"; \
    if [ "$TARGETARCH" == "arm64" ] && [ "$BUILDARCH" != "arm64" ]; then \
      TARGET=aarch64-unknown-linux-gnu; \
      rustup target add $TARGET; \
      apt install -y gcc-aarch64-linux-gnu; \
      dpkg --add-architecture arm64; \
      apt update && apt install -y libssl-dev:arm64; \
      [ ! -z "$BUILD_TARGET" ] && cargo ${BUILD_TARGET} --target $TARGET; \
    elif [ "$TARGETARCH" == "amd64" ] && [ "$BUILDARCH" != "amd64" ]; then \
      TARGET=x86_64-unknown-linux-gnu; \
      rustup target add $TARGET; \
      apt install -y gcc-x86_64-linux-gnu; \
      dpkg --add-architecture amd64; \
      apt update && apt install -y libssl-dev:amd64; \
      [ ! -z "$BUILD_TARGET" ] && cargo ${BUILD_TARGET} --target $TARGET; \
    else \
      [ ! -z "$BUILD_TARGET" ] && cargo ${BUILD_TARGET}; \
    fi;

RUN mkdir /root/bin
ARG BINARIES
ENV BINARIES_ENV ${BINARIES}
RUN bash -c 'BINARIES_ARR=($BINARIES_ENV); for BINARY in "${BINARIES_ARR[@]}"; do cp $BINARY /root/bin/ ; done'

FROM debian:bullseye

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

RUN apt update && apt install -y ca-certificates jq curl git gcc nano lz4 wget unzip
WORKDIR /root

# Install chain binaries
COPY --from=build-env /root/bin /usr/local/bin

# Install libraries
COPY --from=build-env /usr/local/lib /usr/local/lib
RUN cp -r /usr/local/lib/* /lib/

RUN groupadd -g 1025 -r heighliner && useradd -u 1025 --no-log-init -r -g heighliner heighliner
WORKDIR /home/heighliner
USER heighliner
