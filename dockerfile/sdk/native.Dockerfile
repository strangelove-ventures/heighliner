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

RUN [ ! -z "$PRE_BUILD" ] && sh -c "${PRE_BUILD}"; \
    [ ! -z "$BUILD_ENV" ] && export ${BUILD_ENV}; \
    [ ! -z "$BUILD_TAGS" ] && export "${BUILD_TAGS}"; \
    [ ! -z "$BUILD_DIR" ] && cd "${BUILD_DIR}"; \
    LDFLAGS='-linkmode external -extldflags "-static"' make ${BUILD_TARGET}

RUN mkdir /root/bin
ARG BINARIES
ENV BINARIES_ENV ${BINARIES}
RUN bash -c 'BINARIES_ARR=($BINARIES_ENV); for BINARY in "${BINARIES_ARR[@]}"; do cp $BINARY /root/bin/ ; done'

RUN addgroup --gid 1025 -S heighliner && adduser --uid 1025 -S heighliner -G heighliner

FROM scratch

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

# Install chain binaries
COPY --from=build-env /root/bin /usr/local/bin

# Install libraries
COPY --from=build-env /usr/local/lib /usr/local/lib

# Install heighliner user
COPY --from=build-env /etc/passwd /etc/passwd

WORKDIR /home/heighliner
USER heighliner
