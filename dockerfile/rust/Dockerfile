FROM rust:latest AS build-env
ARG VERSION
ARG GITHUB_ORGANIZATION
ARG GITHUB_REPO
ARG BINARIES
ARG BUILD_TARGET
ARG BUILD_ENV
ARG BUILD_TAGS
ARG PRE_BUILD

RUN apt-get update && apt-get install -y clang libclang-dev

RUN curl https://sh.rustup.rs -sSf | sh -s -- --no-modify-path --default-toolchain none -y
RUN rustup component add rustfmt

WORKDIR /build

RUN git clone https://github.com/${GITHUB_ORGANIZATION}/${GITHUB_REPO}.git

WORKDIR /build/${GITHUB_REPO}

RUN git checkout ${VERSION}

RUN cargo fetch

RUN if [ ! -z "$PRE_BUILD" ]; then sh -c "${PRE_BUILD}"; fi; \
    if [ ! -z "$BUILD_ENV" ]; then export ${BUILD_ENV}; fi; \
    if [ ! -z "$BUILD_TAGS" ]; then export "${BUILD_TAGS}"; fi; \
    cargo ${BUILD_TARGET}

RUN mkdir /root/bin
ENV BINARIES_ENV ${BINARIES}
RUN bash -c 'BINARIES_ARR=($BINARIES_ENV); for BINARY in "${BINARIES_ARR[@]}"; do cp $BINARY /root/bin/ ; done'

FROM debian:bullseye

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

RUN apt update && apt install -y ca-certificates jq curl git gcc
WORKDIR /root

# Install chain binaries
COPY --from=build-env /root/bin /usr/local/bin

EXPOSE 26657
