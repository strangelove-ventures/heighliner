FROM --platform=$BUILDPLATFORM rust:latest AS build-env

# RUN apt-get update && apt-get install -y clang libclang-dev

# RUN curl https://sh.rustup.rs -sSf | sh -s -- --no-modify-path --default-toolchain none -y
RUN rustup component add rustfmt

ARG TARGETARCH
ARG BUILDARCH

RUN if [ "${TARGETARCH}" = "arm64" ]; then \
      rustup target add aarch64-unknown-linux-musl; \
      if [ "${BUILDARCH}" != "arm64" ]; then \
        dpkg --add-architecture arm64; \
        wget -c https://musl.cc/aarch64-linux-musl-cross.tgz -O - | tar -xzvv --strip-components 1 -C /usr; \
      fi; \
      apt update && apt install -y libssl-dev:arm64 openssl:arm64 clang:arm64 libclang-dev:arm64; \
    elif [ "${TARGETARCH}" = "amd64" ]; then \
      rustup target add x86_64-unknown-linux-musl; \
      if [ "${BUILDARCH}" != "amd64" ]; then \
        dpkg --add-architecture amd64; \
        wget -c https://musl.cc/x86_64-linux-musl-cross.tgz -O - | tar -xzvv --strip-components 1 -C /usr; \
      fi; \
      apt update && apt install -y libssl-dev:amd64 openssl:amd64 clang:amd64 libclang-dev:amd64; \
    fi

ARG GITHUB_ORGANIZATION
ARG REPO_HOST

WORKDIR /build

ARG GITHUB_REPO
ARG VERSION
ARG BUILD_TIMESTAMP

RUN git clone -b ${VERSION} --single-branch https://${REPO_HOST}/${GITHUB_ORGANIZATION}/${GITHUB_REPO}.git

WORKDIR /build/${GITHUB_REPO}

ARG BUILD_TARGET

RUN if [ ! -z "$BUILD_TARGET" ]; then \
      if [ "$TARGETARCH" = "arm64" ] && [ "$BUILDARCH" != "arm64" ]; then \
        cargo fetch --target aarch64-unknown-linux-musl; \
      elif [ "$TARGETARCH" = "amd64" ] && [ "$BUILDARCH" != "amd64" ]; then \
        cargo fetch --target x86_64-unknown-linux-musl; \
      else \
        cargo fetch; \
      fi; \
    fi

ARG BUILD_ENV
ARG BUILD_TAGS
ARG PRE_BUILD

RUN [ ! -z "$PRE_BUILD" ] && sh -c "${PRE_BUILD}"; \
    [ ! -z "$BUILD_ENV" ] && export ${BUILD_ENV}; \
    [ ! -z "$BUILD_TAGS" ] && export "${BUILD_TAGS}"; \
    if [ "$TARGETARCH" = "arm64" ] && [ "$BUILDARCH" != "arm64" ]; then \
      export TARGET_CC=aarch64-linux-musl-gcc; \
      cargo ${BUILD_TARGET} --target aarch64-unknown-linux-musl; \
    elif [ "$TARGETARCH" = "amd64" ] && [ "$BUILDARCH" != "amd64" ]; then \
      export TARGET_CC=x86_64-linux-musl-gcc; \
      cargo ${BUILD_TARGET} --target x86_64-unknown-linux-musl; \
    else \
      cargo ${BUILD_TARGET}; \
    fi;

RUN mkdir /root/bin
ARG BINARIES
ENV BINARIES_ENV ${BINARIES}
RUN bash -c 'BINARIES_ARR=($BINARIES_ENV); for BINARY in "${BINARIES_ARR[@]}"; do cp $BINARY /root/bin/ ; done'

# Use minimal busybox from infra-toolkit image for final scratch image
FROM ghcr.io/strangelove-ventures/infra-toolkit:v0.0.6 AS busybox-min
RUN addgroup --gid 1025 -S heighliner && adduser --uid 1025 -S heighliner -G heighliner

# Use ln and rm from full featured busybox for assembling final image
FROM busybox:1.34.1-musl AS busybox-full

# Build final image from scratch
FROM scratch

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

WORKDIR /bin

# Install ln (for making hard links) and rm (for cleanup) from full busybox image (will be deleted, only needed for image assembly)
COPY --from=busybox-full /bin/ln /bin/rm ./

# Install minimal busybox image as shell binary (will create hardlinks for the rest of the binaries to this data)
COPY --from=busybox-min /busybox/busybox /bin/sh

# Add hard links for read-only utils, then remove ln and rm
# Will then only have one copy of the busybox minimal binary file with all utils pointing to the same underlying inode
RUN ln sh pwd && \
    ln sh ls && \
    ln sh cat && \
    ln sh less && \
    ln sh grep && \
    ln sh sleep && \
    ln sh env && \
    ln sh tar && \
    ln sh tee && \
    ln sh du && \
    rm ln rm

# Install chain binaries
COPY --from=build-env /root/bin /bin

# Install libraries
COPY --from=build-env /root/lib /lib

# Install trusted CA certificates
COPY --from=busybox-min /etc/ssl/cert.pem /etc/ssl/cert.pem

# Install heighliner user
COPY --from=busybox-min /etc/passwd /etc/passwd
COPY --from=busybox-min --chown=1025:1025 /home/heighliner /home/heighliner

WORKDIR /home/heighliner
USER heighliner
