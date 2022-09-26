FROM ghcr.io/strangelove-ventures/heighliner/busybox:v0.0.1 AS busybox-min

RUN apk add --update --no-cache curl make git libc-dev bash gcc linux-headers eudev-dev

# Build minimal busybox
WORKDIR /
# busybox v1.34.1 stable
RUN git clone -b 1_34_1 --single-branch https://git.busybox.net/busybox
WORKDIR /busybox
ADD busybox.min.config .config
RUN make

RUN addgroup --gid 1025 -S heighliner && adduser --uid 1025 -S heighliner -G heighliner

FROM nixos/nix:latest AS build-env

WORKDIR /build

ARG GITHUB_ORGANIZATION
ARG GITHUB_REPO
ARG VERSION
ARG REPO_HOST
ARG BUILD_TIMESTAMP

RUN git clone -b ${VERSION} --single-branch https://${REPO_HOST}/${GITHUB_ORGANIZATION}/${GITHUB_REPO}.git

WORKDIR /build/${GITHUB_REPO}

ARG BUILD_TARGET

ARG BUILD_ENV
ARG PRE_BUILD

RUN if [ ! -z "$PRE_BUILD" ]; then sh -c "${PRE_BUILD}"; fi; \
    if [ ! -z "$BUILD_TARGET" ]; then \
      if [ ! -z "$BUILD_ENV" ]; then export ${BUILD_ENV}; fi; \
      if [ ! -z "$BUILD_DIR" ]; then cd "${BUILD_DIR}"; fi; \
      nix ${BUILD_TARGET}; \
    fi

# Commented out, but very useful for figuring out which libs are needed to copy over.
# Example: ldd $BINARY to find out which libraries it is looking for.
# COPY --from=busybox-min /lib/ld-musl-x86_64.so.1 /lib/ld-musl-x86_64.so.1
# COPY --from=busybox-min /usr/bin/ldd /bin/ldd

# Copy all binaries to /root/bin, for a single place to copy into final image.
# If a colon (:) delimiter is present, binary will be renamed to the text after the delimiter.
RUN mkdir /root/bin
ARG BINARIES
ENV BINARIES_ENV ${BINARIES}
RUN bash -c \
  'BINARIES_ARR=($BINARIES_ENV); \
  cd /root/bin ; \
  for BINARY in "${BINARIES_ARR[@]}"; do \
    BINSPLIT=(${BINARY//:/ }) ; \
    BINPATH=${BINSPLIT[1]} ; \
    if [ ! -z "$BINPATH" ]; then \
      mkdir -p "$(dirname "${BINPATH}")" ; \
      cp ${BINSPLIT[0]} "${BINPATH}"; \
    else \
      cp ${BINSPLIT[0]} . ; \
    fi; \
  done'

# Copy all libraries to an indexed filepath in /root/lib for a single place to copy into final image.
# Maintain their original filepath in /root/lib.list since they need to be in the same place in the final image for nix.
RUN mkdir -p /root/lib && touch /root/lib.list
ARG LIBRARIES
ENV FILES_ENV ${LIBRARIES}
RUN bash -c 'FILES_ARR=($FILES_ENV); i=0; for FILE in "${FILES_ARR[@]}"; do \
    FILES_NESTED=($(ls -d $FILE)); \
    for FILE_NESTED in "${FILES_NESTED[@]}"; do \
      cp $FILE_NESTED /root/lib/$i; \
      echo "COPYING $i: $FILE_NESTED"; \
      echo $FILE_NESTED >> /root/lib.list; \
      ((i = i + 1)) ;\
    done; \
  done'

# Use utils from full featured busybox for assembling final image
FROM busybox:1.34.1-musl AS busybox-full

# Build final image from scratch
FROM scratch

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

WORKDIR /bin

# Install ln (for making hard links), rm (for cleanup), mv, mkdir, and dirname from full busybox image (will be deleted, only needed for image assembly)
COPY --from=busybox-full /bin/ln /bin/rm /bin/mv /bin/mkdir /bin/dirname ./

# Commented out, but very useful for figuring out which libs are missing in final image
# Example: /bin/ldd $BINARY to find out which libraries it is looking for.
# COPY --from=busybox-min /lib/ld-musl-x86_64.so.1 /lib/ld-musl-x86_64.so.1
# COPY --from=busybox-min /usr/bin/ldd /bin/ldd

# Install minimal busybox image as shell binary (will create hardlinks for the rest of the binaries to this data)
COPY --from=busybox-min /busybox/busybox /bin/sh

# Add hard links for read-only utils
# Will then only have one copy of the busybox minimal binary file with all utils pointing to the same underlying inode
RUN ln sh pwd && \
    ln sh ls && \
    ln sh cat && \
    ln sh less && \
    ln sh grep && \
    ln sh sleep && \
    ln sh du 

# Install chain binaries
COPY --from=build-env /root/bin /bin

# Copy over libraries
COPY --from=build-env /root/lib /root/lib
COPY --from=build-env /root/lib.list /root/lib.list

# Move libraries to their exact locations.
RUN sh -c 'i=0; while read FILE; do \
      echo "$i: $FILE"; \
      DIR="$(dirname "$FILE")"; \
      mkdir -p "$DIR"; \
      mv /root/lib/$i $FILE; \
      i=$((i+1)); \
    done < /root/lib.list'

# Remove write utils used to construct image and tmp dir/file for lib copy.
RUN rm -rf ln rm mv mkdir dirname /root/lib /root/lib.list

# Install trusted CA certificates
COPY --from=busybox-min /etc/ssl/cert.pem /etc/ssl/cert.pem

# Install heighliner user
COPY --from=busybox-min /etc/passwd /etc/passwd
COPY --from=busybox-min --chown=1025:1025 /home/heighliner /home/heighliner

WORKDIR /home/heighliner
USER heighliner
