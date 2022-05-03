FROM rust:latest AS build-env

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

RUN cargo fetch

ARG BUILD_TARGET
ARG BUILD_ENV
ARG BUILD_TAGS
ARG PRE_BUILD

RUN [ ! -z "$PRE_BUILD" ] && sh -c "${PRE_BUILD}"; \
    [ ! -z "$BUILD_ENV" ] && export ${BUILD_ENV}; \
    [ ! -z "$BUILD_TAGS" ] && export "${BUILD_TAGS}"; \
    cargo ${BUILD_TARGET}

RUN mkdir /root/bin
ARG BINARIES
ENV BINARIES_ENV ${BINARIES}
RUN bash -c 'BINARIES_ARR=($BINARIES_ENV); for BINARY in "${BINARIES_ARR[@]}"; do cp $BINARY /root/bin/ ; done'

FROM debian:bullseye

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

RUN apt update && apt install -y ca-certificates jq curl git gcc nano lz4 wget
WORKDIR /root

# Install chain binaries
COPY --from=build-env /root/bin /usr/local/bin

RUN groupadd -r heighliner && useradd --no-log-init -r -g heighliner heighliner
WORKDIR /home/heighliner
USER heighliner
