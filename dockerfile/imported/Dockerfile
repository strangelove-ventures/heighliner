ARG BASE_IMAGE
ARG VERSION
FROM $BASE_IMAGE:$VERSION AS imported

# Use minimal busybox from infra-toolkit image for final scratch image
FROM ghcr.io/strangelove-ventures/infra-toolkit:v0.1.4 AS infra-toolkit
RUN addgroup --gid 1025 -S heighliner && adduser --uid 1025 -S heighliner -G heighliner

# Use ln and rm from full featured busybox for assembling final image
FROM busybox:1.34.1-musl AS busybox-full

FROM debian:latest AS build-env

COPY --from=imported / /imported

RUN export ARCH=$(uname -m);\
    BIN="$(eval "echo "/imported/nix/store/*-uniond-$(uname -m)-unknown-linux-musl/bin/uniond"")";\
    echo $BIN

# Copy all binaries to /root/bin, for a single place to copy into final image.
# If a colon (:) delimiter is present, binary will be renamed to the text after the delimiter.
RUN mkdir /root/bin
ARG BINARIES
ENV BINARIES_ENV ${BINARIES}
RUN bash -c 'set -eux;\
  export ARCH=$(uname -m);\
  BINARIES_ARR=();\
  IFS=, read -ra BINARIES_ARR <<< "$BINARIES_ENV";\
  for BINARY in "${BINARIES_ARR[@]}"; do\
    BINSPLIT=();\
    IFS=: read -ra BINSPLIT <<< "$BINARY";\
    BINPATH="${BINSPLIT[1]+"${BINSPLIT[1]}"}";\
    BIN="$(eval "echo "/imported${BINSPLIT[0]+"${BINSPLIT[0]}"}"")";\
    if [ ! -z "$BINPATH" ]; then\
      if [[ $BINPATH == *"/"* ]]; then\
        mkdir -p "$(dirname "${BINPATH}")";\
        cp "$BIN" "${BINPATH}";\
      else\
        cp "$BIN" "/root/bin/${BINPATH}";\
      fi;\
    else\
      cp "$BIN" /root/bin/;\
    fi;\
  done'

RUN mkdir -p /root/lib
ARG LIBRARIES
ENV LIBRARIES_ENV ${LIBRARIES}
RUN bash -c 'set -eux;\
  export ARCH=$(uname -m);\
  LIBRARIES_ARR=($LIBRARIES_ENV); for LIBRARY in "${LIBRARIES_ARR[@]}"; do LIB="$(eval "echo "$LIBRARY"")"; cp /imported$LIB /root/lib/; done'

# Copy over directories
RUN mkdir -p /root/dir_abs && touch /root/dir_abs.list
ARG DIRECTORIES
ENV DIRECTORIES_ENV ${DIRECTORIES}
RUN bash -c 'set -eux;\
  DIRECTORIES_ARR=($DIRECTORIES_ENV);\
  i=0;\
  for DIRECTORY in "${DIRECTORIES_ARR[@]}"; do \
    cp -R /imported$DIRECTORY /root/dir_abs/$i;\
    echo $DIRECTORY >> /root/dir_abs.list;\
    ((i = i + 1));\
  done'

# Determine shared library dependencies for both bins and libs
RUN cp -R /root/lib/* /root/bin/ || true
RUN mkdir -p /root/lib_abs && touch /root/lib_abs.list
RUN bash -c 'set -eux;\
    export ARCH=$(uname -m);\
    i=0; for BIN in /root/bin/*; do\
    echo "Getting $(uname -m) libs for bin: $BIN";\
    readarray -t LIBS < <(ldd "$BIN");\
    for LIB in "${LIBS[@]}"; do\
      PATH1=$(echo $LIB | awk "{print \$1}");\
      if [ "$PATH1" = "linux-vdso.so.1" ]; then continue; fi;\
      PATH2=$(echo $LIB | awk "{print \$3}");\
      PATH3=$(echo $LIB | awk "{print \$4}");\
      if [ "$PATH2" == "not" ] && [ "$PATH3" == "found" ]; then continue; fi;\
      if [ ! -z "$PATH2" ]; then\
        if cat /root/lib_abs.list | grep -x "$PATH2"; then\
          echo "Skipping $PATH2, already accounted for";\
          continue;\
        else\
          echo "Copying lib2: $PATH2";\
          cp -L $PATH2 /root/lib_abs/$i;\
          echo $PATH2 >> /root/lib_abs.list;\
        fi;\
      else\
        if cat /root/lib_abs.list | grep -x "$PATH1"; then\
          echo "Skipping $PATH1, already accounted for";\
          continue;\
        else\
          echo "Copying lib1: $PATH1";\
          cp -L $PATH1 /root/lib_abs/$i;\
          echo $PATH1 >> /root/lib_abs.list;\
        fi;\
      fi;\
      ((i = i + 1));\
    done;\
  done'

ARG TARGET_LIBRARIES
ENV TARGET_LIBRARIES_ENV ${TARGET_LIBRARIES}
RUN bash -c 'set -eux;\
  export ARCH=$(uname -m);\
  i=$(wc -l < /root/lib_abs.list);\
  LIBRARIES_ARR=($TARGET_LIBRARIES_ENV); for LIBRARY in "${LIBRARIES_ARR[@]}"; do LIB="$(eval "echo "$LIBRARY"")";\
    if cat /root/lib_abs.list | grep -x "$LIB"; then\
      echo "Skipping $LIB, already accounted for";\
      continue;\
    else\
      echo "Copying lib2: $LIB";\
      cp -L $LIB /root/lib_abs/$i;\
      echo $LIB >> /root/lib_abs.list;\
      ((i = i + 1));\
    fi;\
  done'

# Build final image from scratch
FROM scratch

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

WORKDIR /bin

# Install ln (for making hard links), rm (for cleanup), mv, mkdir, and dirname from full busybox image (will be deleted, only needed for image assembly)
COPY --from=busybox-full /bin/ln /bin/rm /bin/mv /bin/mkdir /bin/dirname ./

# Install minimal busybox image as shell binary (will create hardlinks for the rest of the binaries to this data)
COPY --from=infra-toolkit /busybox/busybox /bin/sh

# Install jq
COPY --from=infra-toolkit /usr/local/bin/jq /bin/

# Add hard links for read-only utils
# Will then only have one copy of the busybox minimal binary file with all utils pointing to the same underlying inode
RUN for b in \
  cat \
  date \
  df \
  du \
  env \
  grep \
  head \
  less \
  ls \
  md5sum \
  pwd \
  sha1sum \
  sha256sum \
  sha3sum \
  sha512sum \
  sleep \
  stty \
  tail \
  tar \
  tee \
  tr \
  watch \
  which \
  ; do ln sh $b; done

# Install chain binaries
COPY --from=build-env /root/bin /bin

# Install libraries that don't need absolute path
COPY --from=build-env /root/lib /lib

# Copy over absolute path libraries
COPY --from=build-env /root/lib_abs /root/lib_abs
COPY --from=build-env /root/lib_abs.list /root/lib_abs.list

# Move absolute path libraries to their absolute locations.
RUN sh -c 'i=0; while read FILE; do\
      echo "$i: $FILE";\
      DIR="$(dirname "$FILE")";\
      mkdir -p "$DIR";\
      mv /root/lib_abs/$i $FILE;\
      i=$((i+1));\
    done < /root/lib_abs.list'

# Copy over absolute path directories
COPY --from=build-env /root/dir_abs /root/dir_abs
COPY --from=build-env /root/dir_abs.list /root/dir_abs.list

# Move absolute path directories to their absolute locations.
RUN sh -c 'i=0; while read DIR; do\
      echo "$i: $DIR";\
      PLACEDIR="$(dirname "$DIR")";\
      mkdir -p "$PLACEDIR";\
      mv /root/dir_abs/$i $DIR;\
      i=$((i+1));\
    done < /root/dir_abs.list'

RUN mkdir -p /usr/bin && ln -s /bin/env /usr/bin/env

ARG FINAL_IMAGE
RUN if [ ! -z "$FINAL_IMAGE" ]; then sh -c "$FINAL_IMAGE"; fi

# Remove write utils used to construct image and tmp dir/file for lib copy.
RUN rm -rf ln rm mv mkdir dirname /root/lib_abs /root/lib_abs.list

# Install trusted CA certificates
COPY --from=infra-toolkit /etc/ssl/cert.pem /etc/ssl/cert.pem

# Install heighliner user
COPY --from=infra-toolkit /etc/passwd /etc/passwd
COPY --from=infra-toolkit --chown=1025:1025 /home/heighliner /home/heighliner
COPY --from=infra-toolkit --chown=1025:1025 /tmp /tmp

WORKDIR /home/heighliner
USER heighliner
