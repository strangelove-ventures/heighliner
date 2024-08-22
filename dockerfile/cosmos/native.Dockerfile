ARG BASE_VERSION
FROM golang:${BASE_VERSION} AS build-env

RUN apk add --update --no-cache curl make git libc-dev bash gcc linux-headers eudev-dev ncurses-dev

ARG CLONE_KEY

RUN if [ ! -z "${CLONE_KEY}" ]; then\
        mkdir -p ~/.ssh;\
        echo "${CLONE_KEY}" | base64 -d > ~/.ssh/id_ed25519;\
        chmod 600 ~/.ssh/id_ed25519;\
        apk add openssh;\
        git config --global --add url."ssh://git@github.com/".insteadOf "https://github.com/";\
        ssh-keyscan github.com >> ~/.ssh/known_hosts;\
    fi

ARG TARGETARCH
ARG BUILDARCH
ARG GITHUB_ORGANIZATION
ARG REPO_HOST

WORKDIR /go/src/${REPO_HOST}/${GITHUB_ORGANIZATION}

ARG GITHUB_REPO
ARG VERSION
ARG BUILD_TIMESTAMP

RUN git clone -b ${VERSION} --single-branch https://${REPO_HOST}/${GITHUB_ORGANIZATION}/${GITHUB_REPO}.git --recursive

WORKDIR /go/src/${REPO_HOST}/${GITHUB_ORGANIZATION}/${GITHUB_REPO}

ARG BUILD_TARGET
ARG BUILD_ENV
ARG BUILD_TAGS
ARG PRE_BUILD
ARG BUILD_DIR
ARG WASMVM_VERSION

RUN set -eux;\
    export ARCH=$(uname -m);\
    if [ ! -z "${WASMVM_VERSION}" ]; then\
      WASMVM_REPO=$(echo $WASMVM_VERSION | awk '{print $1}');\
      WASMVM_VERS=$(echo $WASMVM_VERSION | awk '{print $2}');\
      wget -O /lib/libwasmvm_muslc.a https://${WASMVM_REPO}/releases/download/${WASMVM_VERS}/libwasmvm_muslc.$(uname -m).a;\
      cp /lib/libwasmvm_muslc.a /lib/libwasmvm_muslc.$(uname -m).a;\
      cp /lib/libwasmvm_muslc.a /lib/libwasmvm.$(uname -m).a;\
    fi;\
    export CGO_ENABLED=1 LDFLAGS='-linkmode external -extldflags "-static"';\
    if [ ! -z "$PRE_BUILD" ]; then sh -c "${PRE_BUILD}"; fi;\
    if [ ! -z "$BUILD_TARGET" ]; then\
      if [ ! -z "$BUILD_ENV" ]; then export ${BUILD_ENV}; fi;\
      if [ ! -z "$BUILD_TAGS" ]; then export "${BUILD_TAGS}"; fi;\
      if [ ! -z "$BUILD_DIR" ]; then cd "${BUILD_DIR}"; fi;\
      sh -c "${BUILD_TARGET}";\
    fi

# Copy all binaries to /root/bin, for a single place to copy into final image.
# If a colon (:) delimiter is present, binary will be renamed to the text after the delimiter.
RUN mkdir /root/bin
ARG RACE
ARG BINARIES
ENV BINARIES_ENV ${BINARIES}
RUN bash -c 'set -eux;\
  BINARIES_ARR=();\
  IFS=, read -ra BINARIES_ARR <<< "$BINARIES_ENV";\
  for BINARY in "${BINARIES_ARR[@]}"; do\
    BINSPLIT=();\
    IFS=: read -ra BINSPLIT <<< "$BINARY";\
    BINPATH=${BINSPLIT[1]+"${BINSPLIT[1]}"};\
    BIN="$(eval "echo "${BINSPLIT[0]+"${BINSPLIT[0]}"}"")";\
    if [ ! -z "$RACE" ] && GOVERSIONOUT=$(go version -m $BIN); then\
      if echo $GOVERSIONOUT | grep build | grep "-race=true"; then\
        echo "Race detection is enabled in binary";\
      else\
        echo "Race detection not enabled in binary!";\
        exit 1;\
      fi;\
    fi;\
    if [ ! -z "$BINPATH" ]; then\
      if [[ $BINPATH == *"/"* ]]; then\
        mkdir -p "$(dirname "${BINPATH}")";\
        cp "$BIN" "${BINPATH}"; \
      else \
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
  LIBRARIES_ARR=($LIBRARIES_ENV); for LIBRARY in "${LIBRARIES_ARR[@]}"; do cp $LIBRARY /root/lib/; done'

# Use minimal busybox from infra-toolkit image for final scratch image
FROM ghcr.io/p2p-org/cosmos-heighliner:infra-toolkit-v0.1.6 AS infra-toolkit

# Use ln and rm from full featured busybox for assembling final image
FROM busybox:1.34.1-musl AS busybox-full

# Build final image from scratch
FROM alpine:3

LABEL org.opencontainers.image.source="https://github.com/p2p-org/cosmos-heighliner"

WORKDIR /bin

# Install jq
COPY --from=infra-toolkit /usr/local/bin/jq /bin/

# Install chain binaries
COPY --from=build-env /root/bin /bin

# Install libraries
COPY --from=build-env /root/lib /lib

# # Install p2p user
# RUN addgroup --gid 1111 -S p2p && adduser --uid 1111 -S p2p -G p2p
# RUN chown 1111:1111 -R /home/p2p
# RUN chown 1111:1111 -R /etc/apk
# RUN chown 1111:1111 -R /tmp

WORKDIR /home/p2p
# USER p2p
