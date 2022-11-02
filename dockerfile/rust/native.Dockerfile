FROM rust:1-bullseye AS build-env

RUN rustup component add rustfmt

RUN apt update && apt install -y libssl1.1 libssl-dev openssl libclang-dev clang cmake libstdc++6
RUN if [ "$(uname -m)" = "aarch64" ]; then \
      wget https://github.com/protocolbuffers/protobuf/releases/download/v21.8/protoc-21.8-linux-aarch_64.zip; \
      unzip protoc-21.8-linux-aarch_64.zip -d /usr; \
    elif [ "${TARGETARCH}" = "amd64" ]; then \
      wget https://github.com/protocolbuffers/protobuf/releases/download/v21.8/protoc-21.8-linux-x86_64.zip; \
      unzip protoc-21.8-linux-x86_64.zip -d /usr; \
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
ARG BUILD_DIR

RUN if [ ! -z "$BUILD_TARGET" ]; then \
      if [ ! -z "$BUILD_DIR" ]; then cd "${BUILD_DIR}"; fi; \
      cargo fetch; \
    fi

ARG BUILD_ENV
ARG BUILD_TAGS
ARG PRE_BUILD

RUN [ ! -z "$PRE_BUILD" ] && sh -c "${PRE_BUILD}"; \
    [ ! -z "$BUILD_ENV" ] && export ${BUILD_ENV}; \
    [ ! -z "$BUILD_TAGS" ] && export "${BUILD_TAGS}"; \
    if [ ! -z "$BUILD_DIR" ]; then cd "${BUILD_DIR}"; fi; \
    if [ ! -z "$BUILD_TARGET" ]; then \
      cargo ${BUILD_TARGET} --target $(uname -m)-unknown-linux-gnu; \
    fi;

# Copy all binaries to /root/bin, for a single place to copy into final image.
# If a colon (:) delimiter is present, binary will be renamed to the text after the delimiter.
# Copy all linked shared libraries for each binary to an indexed filepath in /root/lib_abs for a single place to copy into final image.
# Maintain their original filepath in /root/lib_abs.list since they need to be in the same place in the final image.
RUN mkdir /root/bin
RUN mkdir -p /root/lib_abs && touch /root/lib_abs.list
ARG BINARIES
ENV BINARIES_ENV ${BINARIES}
RUN bash -c \
  'export ARCH=$(uname -m); \
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
    readarray -t LIBS < <(ldd "$BIN"); \
    i=0; for LIB in "${LIBS[@]}"; do \
      PATH1=$(echo $LIB | awk "{print \$1}") ; \
      if [ "$PATH1" = "linux-vdso.so.1" ]; then continue; fi; \
      PATH2=$(echo $LIB | awk "{print \$3}") ; \
      if [ ! -z "$PATH2" ]; then \
        cp $PATH2 /root/lib_abs/$i ; \
        echo $PATH2 >> /root/lib_abs.list; \
      else \
        cp $PATH1 /root/lib_abs/$i ; \
        echo $PATH1 >> /root/lib_abs.list; \
      fi; \
      ((i = i + 1)) ;\
    done; \
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
COPY --from=build-env /root/lib_abs /root/lib_abs
COPY --from=build-env /root/lib_abs.list /root/lib_abs.list

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
