FROM --platform=$BUILDPLATFORM rust:1-bullseye AS build-env

RUN rustup component add rustfmt

ARG TARGETARCH
ARG BUILDARCH
ENV TARGETARCH ${TARGETARCH}

RUN if [ "${TARGETARCH}" = "arm64" ]; then \
      rustup target add aarch64-unknown-linux-gnu; \
      wget https://github.com/protocolbuffers/protobuf/releases/download/v21.8/protoc-21.8-linux-aarch_64.zip; \
      unzip protoc-21.8-linux-aarch_64.zip -d /usr; \
      if [ "${BUILDARCH}" != "arm64" ]; then \
        dpkg --add-architecture arm64; \
        apt update && apt install -y gcc-aarch64-linux-gnu g++-aarch64-linux-gnu; \
        ln -s /usr/aarch64-linux-gnu/include/bits /usr/include/bits; \
        ln -s /usr/aarch64-linux-gnu/include/sys /usr/include/sys; \
        ln -s /usr/aarch64-linux-gnu/include/gnu /usr/include/gnu; \
      else \
        apt update; \
      fi; \
      apt install -y libssl1.1:arm64 libssl-dev:arm64 openssl:arm64 libclang-dev clang cmake libstdc++6:arm64; \
    elif [ "${TARGETARCH}" = "amd64" ]; then \
      rustup target add x86_64-unknown-linux-gnu; \
      wget https://github.com/protocolbuffers/protobuf/releases/download/v21.8/protoc-21.8-linux-x86_64.zip; \
      unzip protoc-21.8-linux-x86_64.zip -d /usr; \
      if [ "${BUILDARCH}" != "amd64" ]; then \
        dpkg --add-architecture amd64; apt update; \
        apt update && apt install -y gcc-x86_64-linux-gnu g++-x86_64-linux-gnu; \
        ln -s /usr/x86_64-linux-gnu/include/bits /usr/include/bits; \
        ln -s /usr/x86_64-linux-gnu/include/sys /usr/include/sys; \
        ln -s /usr/x86_64-linux-gnu/include/gnu /usr/include/gnu; \
      else \
        apt update; \
      fi; \
      apt install -y libssl1.1:amd64 libssl-dev:amd64 openssl:amd64 libclang-dev clang cmake libstdc++6:amd64; \
    fi

ARG GITHUB_ORGANIZATION
ARG REPO_HOST

WORKDIR /build

ARG GITHUB_REPO
ARG VERSION
ARG BUILD_TIMESTAMP

RUN git clone -b ${VERSION} --single-branch https://${REPO_HOST}/${GITHUB_ORGANIZATION}/${GITHUB_REPO}.git --recursive

WORKDIR /build/${GITHUB_REPO}

ARG BUILD_TARGET
ARG BUILD_DIR

RUN if [ ! -z "$BUILD_TARGET" ]; then \
      if [ ! -z "$BUILD_DIR" ]; then cd "${BUILD_DIR}"; fi; \
      if [ "$TARGETARCH" = "arm64" ] && [ "$BUILDARCH" != "arm64" ]; then \
        cargo fetch --target aarch64-unknown-linux-gnu; \
      elif [ "$TARGETARCH" = "amd64" ] && [ "$BUILDARCH" != "amd64" ]; then \
        cargo fetch --target x86_64-unknown-linux-gnu; \
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
    if [ ! -z "$BUILD_DIR" ]; then cd "${BUILD_DIR}"; fi; \
    if [ ! -z "$BUILD_TARGET" ]; then \
      if [ "$TARGETARCH" = "arm64" ] && [ "$BUILDARCH" != "arm64" ]; then \
        export CARGO_TARGET_AARCH64_UNKNOWN_LINUX_GNU_LINKER=aarch64-linux-gnu-gcc \
          CC_aarch64_unknown_linux_gnu=aarch64-linux-gnu-gcc \
          CXX_aarch64_unknown_linux_gnu=aarch64-linux-gnu-g++ \
          PKG_CONFIG_SYSROOT_DIR=/usr/aarch64-linux-gnu; \
        cargo ${BUILD_TARGET} --target aarch64-unknown-linux-gnu; \
      elif [ "$TARGETARCH" = "amd64" ] && [ "$BUILDARCH" != "amd64" ]; then \
        export CARGO_TARGET_x86_64_UNKNOWN_LINUX_GNU_LINKER=x86_64-linux-gnu-gcc \
          CC_x86_64_unknown_linux_gnu=x86_64-linux-gnu-gcc \
          CXX_x86_64_unknown_linux_gnu=x86_64-linux-gnu-g++ \
          PKG_CONFIG_SYSROOT_DIR=/usr/x86_64-linux-gnu; \
        cargo ${BUILD_TARGET} --target x86_64-unknown-linux-gnu; \
      else \
        cargo ${BUILD_TARGET} --target $(uname -m)-unknown-linux-gnu;\
      fi; \
    fi

# Copy all binaries to /root/bin, for a single place to copy into final image.
# If a colon (:) delimiter is present, binary will be renamed to the text after the delimiter.
RUN mkdir /root/bin
ARG BINARIES
ENV BINARIES_ENV ${BINARIES}
RUN bash -c \
  'if [ "${TARGETARCH}" = "arm64" ]; then export ARCH=aarch64; \
  elif [ "${TARGETARCH}" = "amd64" ]; then export ARCH=x86_64; fi; \
  IFS=, read -ra BINARIES_ARR <<< "$BINARIES_ENV"; \
  for BINARY in "${BINARIES_ARR[@]}"; do \
    IFS=: read -ra BINSPLIT <<< "$BINARY"; \
    BINPATH=${BINSPLIT[1]} ;\
    BIN="$(eval "echo "${BINSPLIT[0]}"")"; \
    if [ ! -z "$BINPATH" ]; then \
      if [[ $BINPATH == *"/"* ]]; then \
        mkdir -p "$(dirname "${BINPATH}")" ; \
        cp "$BIN" "${BINPATH}"; \
      else \
        cp "$BIN" "/root/bin/${BINPATH}"; \
      fi;\
    else \
      cp "$BIN" /root/bin/ ; \
    fi; \
  done'

RUN mkdir -p /root/lib
ARG LIBRARIES
ENV LIBRARIES_ENV ${LIBRARIES}
RUN bash -c 'LIBRARIES_ARR=($LIBRARIES_ENV); for LIBRARY in "${LIBRARIES_ARR[@]}"; do cp $LIBRARY /root/lib/; done'

# Use minimal busybox from infra-toolkit image for final scratch image
FROM ghcr.io/strangelove-ventures/infra-toolkit:v0.0.6 AS busybox-min
RUN addgroup --gid 1025 -S heighliner && adduser --uid 1025 -S heighliner -G heighliner

# Use ln and rm from full featured busybox for assembling final image
FROM busybox:1.34.1-musl AS busybox-full

# Use TARGETARCH image for determining necessary libs
FROM rust:1-bullseye as target-arch-libs
RUN apt update && apt install -y libssl1.1 openssl clang libstdc++6

COPY --from=build-env /root/bin /root/bin
RUN mkdir -p /root/lib_abs && touch /root/lib_abs.list
RUN bash -c \
  'ls /root/bin; \
  for BIN in /root/bin/*; do \
    echo "Getting $(uname -m) libs for bin: $BIN"; \
    readarray -t LIBS < <(ldd "$BIN"); \
    i=0; for LIB in "${LIBS[@]}"; do \
      PATH1=$(echo $LIB | awk "{print \$1}") ; \
      if [ "$PATH1" = "linux-vdso.so.1" ]; then continue; fi; \
      PATH2=$(echo $LIB | awk "{print \$3}") ; \
      if [ ! -z "$PATH2" ]; then \
        echo "Copying $(uname -m) lib2: $PATH2"; \
        cp $PATH2 /root/lib_abs/$i ; \
        echo $PATH2 >> /root/lib_abs.list; \
      else \
        echo "Copying $(uname -m) lib1: $PATH1"; \
        cp $PATH1 /root/lib_abs/$i ; \
        echo $PATH1 >> /root/lib_abs.list; \
      fi; \
      ((i = i + 1)) ;\
    done; \
  done'

# Build final image from scratch
FROM scratch

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

WORKDIR /bin

# Install ln (for making hard links), rm (for cleanup), mv, mkdir, and dirname from full busybox image (will be deleted, only needed for image assembly)
COPY --from=busybox-full /bin/ln /bin/rm /bin/mv /bin/mkdir /bin/dirname ./

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
    ln sh du

# Install chain binaries
COPY --from=build-env /root/bin /bin

# Install libraries that don't need absolute path
COPY --from=build-env /root/lib /lib

# Copy over absolute path libraries
COPY --from=target-arch-libs /root/lib_abs /root/lib_abs
COPY --from=target-arch-libs /root/lib_abs.list /root/lib_abs.list

# Move absolute path libraries to their absolute locations.
RUN sh -c 'i=0; while read FILE; do \
      echo "$i: $FILE"; \
      DIR="$(dirname "$FILE")"; \
      mkdir -p "$DIR"; \
      mv /root/lib_abs/$i $FILE; \
      i=$((i+1)); \
    done < /root/lib_abs.list'

# Remove write utils used to construct image and tmp dir/file for lib copy.
RUN rm -rf ln rm mv mkdir dirname /root/lib_abs /root/lib_abs.list

# Install trusted CA certificates
COPY --from=busybox-min /etc/ssl/cert.pem /etc/ssl/cert.pem

# Install heighliner user
COPY --from=busybox-min /etc/passwd /etc/passwd
COPY --from=busybox-min --chown=1025:1025 /home/heighliner /home/heighliner

WORKDIR /home/heighliner
USER heighliner
