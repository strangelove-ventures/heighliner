FROM rust:latest AS build-env

RUN apt-get update && apt-get install -y clang libclang-dev

RUN curl https://sh.rustup.rs -sSf | sh -s -- --no-modify-path --default-toolchain none -y
RUN rustup component add rustfmt

WORKDIR /build

ARG GITHUB_ORGANIZATION
ARG GITHUB_REPO
ARG VERSION
ARG REPO_HOST

RUN git clone https://${REPO_HOST}/${GITHUB_ORGANIZATION}/${GITHUB_REPO}.git

WORKDIR /build/${GITHUB_REPO}

RUN git checkout ${VERSION}

ARG BUILD_TARGET

RUN if [ ! -z "$BUILD_TARGET" ]; then cargo fetch; fi

ARG BUILD_ENV
ARG BUILD_TAGS
ARG PRE_BUILD

RUN if [ ! -z "$PRE_BUILD" ]; then sh -c "${PRE_BUILD}"; fi; \
    if [ ! -z "$BUILD_TARGET" ]; then \
      if [ ! -z "$BUILD_ENV" ]; then export ${BUILD_ENV}; fi; \
      if [ ! -z "$BUILD_TAGS" ]; then export "${BUILD_TAGS}"; fi; \
      if [ ! -z "$BUILD_DIR" ]; then cd "${BUILD_DIR}"; fi; \
      cargo ${BUILD_TARGET}; \
    fi

# Copy all binaries to /root/bin, for a single place to copy into final image.
# If a colon (:) delimiter is present, binary will be renamed to the text after the delimiter.
RUN mkdir /root/bin
ARG BINARIES
ENV BINARIES_ENV ${BINARIES}
RUN bash -c \
  'BINARIES_ARR=($BINARIES_ENV); \
  for BINARY in "${BINARIES_ARR[@]}"; do \
    BINSPLIT=(${BINARY//:/ }) ; \
    BINPATH=${BINSPLIT[1]} ; \
    if [ ! -z "$BINPATH" ]; then \
      if [[ $BINPATH == *"/"* ]]; then \
        mkdir -p "$(dirname "${BINPATH}")" ; \
        cp ${BINSPLIT[0]} "${BINPATH}"; \
      else \
        cp ${BINSPLIT[0]} "/root/bin/${BINPATH}"; \
      fi ;\
    else \
      cp ${BINSPLIT[0]} /root/bin/ ; \
    fi; \
  done'

RUN mkdir -p /root/lib
ARG LIBRARIES
ENV LIBRARIES_ENV ${LIBRARIES}
RUN bash -c 'LIBRARIES_ARR=($LIBRARIES_ENV); for LIBRARY in "${LIBRARIES_ARR[@]}"; do cp $LIBRARY /root/lib/; done'

FROM debian:bullseye

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

RUN apt update && apt install -y ca-certificates jq curl git gcc nano lz4 wget unzip
WORKDIR /root

# Install chain binaries
COPY --from=build-env /root/bin /usr/local/bin

# Install libraries
COPY --from=build-env /root/lib /lib

RUN groupadd -g 1025 -r heighliner && useradd -u 1025 --no-log-init -r -g heighliner heighliner
WORKDIR /home/heighliner
RUN chown -R heighliner:heighliner /home/heighliner
USER heighliner
