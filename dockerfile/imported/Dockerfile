ARG BASE_IMAGE
ARG VERSION
FROM $BASE_IMAGE:$VERSION AS build-env

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
      cp -d ${BINSPLIT[0]} "${BINPATH}"; \
    else \
      cp -d ${BINSPLIT[0]} . ; \
    fi; \
  done'

# Copy all libraries to an indexed filepath in /root/lib for a single place to copy into final image.
# Maintain their original filepath in /root/lib.list since they need to be in the same place in the final image for nix.
RUN mkdir -p /root/lib && touch /root/lib.list
ARG LIBRARIES
ENV FILES_ENV ${LIBRARIES}
RUN bash -c 'shopt -s globstar; FILES_ARR=($FILES_ENV); i=0; for FILE in "${FILES_ARR[@]}"; do \
    FILES_NESTED=($(ls -d $FILE)); \
    for FILE_NESTED in "${FILES_NESTED[@]}"; do \
      if [ -d "$FILE_NESTED" ]; then \
        continue; \
      fi; \
      cp -d $FILE_NESTED /root/lib/$i; \
      echo "COPYING $i: $FILE_NESTED"; \
      echo $FILE_NESTED >> /root/lib.list; \
      ((i = i + 1)) ;\
    done; \
  done'

# Use minimal busybox from infra-toolkit image for final scratch image
FROM ghcr.io/strangelove-ventures/infra-toolkit:v0.0.5 AS busybox-min
RUN addgroup --gid 1025 -S heighliner && adduser --uid 1025 -S heighliner -G heighliner

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
# COPY --from=busybox-min /lib /lib
# COPY --from=busybox-min /usr/bin/ldd /bin/ldd

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

# Install binaries
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
