FROM rust:latest AS build-env
ARG VERSION
ARG GITHUB_ORGANIZATION
ARG GITHUB_REPO
ARG BINARY
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

RUN cp ${BINARY} /root/binary

FROM debian:bullseye

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

ARG BINARY
ENV BINARY ${BINARY}

RUN apt update && apt install -y ca-certificates jq curl git gcc
WORKDIR /root

# Install chain binary
COPY --from=build-env /root/binary .
RUN mv /root/binary /usr/bin/$(basename $BINARY)

EXPOSE 26657

CMD env $(basename $BINARY) start
