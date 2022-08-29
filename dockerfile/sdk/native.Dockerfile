FROM golang:alpine AS build-env

RUN apk add --update --no-cache curl make git libc-dev bash gcc linux-headers eudev-dev

ARG TARGETARCH
ARG BUILDARCH

RUN wget -O /lib/libwasmvm_muslc.a https://github.com/CosmWasm/wasmvm/releases/download/v1.0.0/libwasmvm_muslc.$(uname -m).a

ARG GITHUB_ORGANIZATION
ARG REPO_HOST

WORKDIR /go/src/${REPO_HOST}/${GITHUB_ORGANIZATION}

ARG GITHUB_REPO
ARG VERSION

WORKDIR /go/src/${REPO_HOST}/${GITHUB_ORGANIZATION}

RUN git clone https://${REPO_HOST}/${GITHUB_ORGANIZATION}/${GITHUB_REPO}.git
WORKDIR /go/src/${REPO_HOST}/${GITHUB_ORGANIZATION}/${GITHUB_REPO}
RUN git checkout ${VERSION}

ARG BUILD_TARGET
ARG BUILD_ENV
ARG BUILD_TAGS
ARG PRE_BUILD
ARG BUILD_DIR

RUN set -eux; \
    export CGO_ENABLED=1 LDFLAGS='-linkmode external -extldflags "-static"'; \
    if [ ! -z "$PRE_BUILD" ]; then sh -c "${PRE_BUILD}"; fi; \
    if [ ! -z "$BUILD_TARGET" ]; then \
      if [ ! -z "$BUILD_ENV" ]; then export ${BUILD_ENV}; fi; \
      if [ ! -z "$BUILD_TAGS" ]; then export "${BUILD_TAGS}"; fi; \
      if [ ! -z "$BUILD_DIR" ]; then cd "${BUILD_DIR}"; fi; \
      make ${BUILD_TARGET}; \
    fi

RUN mkdir /root/bin
ARG BINARIES
ENV BINARIES_ENV ${BINARIES}
RUN bash -c 'BINARIES_ARR=($BINARIES_ENV); for BINARY in "${BINARIES_ARR[@]}"; do ldd $BINARY; cp $BINARY /root/bin/ ; done'

RUN mkdir /root/lib
ARG LIBRARIES
ENV LIBRARIES_ENV ${LIBRARIES}
RUN bash -c 'LIBRARIES_ARR=($LIBRARIES_ENV); for LIBRARY in "${LIBRARIES_ARR[@]}"; do cp $LIBRARY /root/lib/; done'

RUN addgroup --gid 1025 -S heighliner && adduser --uid 1025 -S heighliner -G heighliner

# Will use this for copying a few minimal utils
FROM busybox:latest AS busybox

# Build final image from scratch
FROM scratch

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

# Install sh and ln (will be two copies of the busybox binary) for making hard links
COPY --from=busybox /bin/sh /bin/ln /bin/

# Add hard links for read-only utils, then remove ln and rm
# Will then only have one copy of the busybox binary, with all other utils pointing to the same underlying data
RUN ln /bin/sh /bin/pwd; \
    ln /bin/sh /bin/ls; \
    ln /bin/sh /bin/cat; \
    ln /bin/sh /bin/less; \
    ln /bin/sh /bin/rm; \
    rm /bin/rm /bin/ln

# Install chain binaries
COPY --from=build-env /root/bin /bin

# Install libraries
COPY --from=build-env /root/lib /lib

# Install heighliner user
COPY --from=build-env /etc/passwd /etc/passwd
COPY --from=build-env --chown=1025:1025 /home/heighliner /home/heighliner

WORKDIR /home/heighliner
USER heighliner
