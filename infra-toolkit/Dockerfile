FROM --platform=$BUILDPLATFORM alpine:3 AS build-env

RUN apk add --update --no-cache \
  bash \
  curl \
  eudev-dev \
  gcc \
  git \
  libc-dev \
  linux-headers \
  make \
  wget \
  bison \
  flex \
  automake \
  autoconf \
  libtool

ARG TARGETARCH
ARG BUILDARCH

RUN LIBDIR=/lib; \
    if [ "${TARGETARCH}" = "arm64" ]; then \
      ARCH=aarch64; \
      if [ "${BUILDARCH}" != "arm64" ]; then \
        wget -c https://musl.cc/aarch64-linux-musl-cross.tgz -O - | tar -xzvv --strip-components 1 -C /usr; \
        LIBDIR=/usr/aarch64-linux-musl/lib; \
        mkdir -p $LIBDIR; \
      fi; \
    elif [ "${TARGETARCH}" = "amd64" ]; then \
      ARCH=x86_64; \
      if [ "${BUILDARCH}" != "amd64" ]; then \
        wget -c https://musl.cc/x86_64-linux-musl-cross.tgz -O - | tar -xzvv --strip-components 1 -C /usr; \
        LIBDIR=/usr/x86_64-linux-musl/lib; \
        mkdir -p $LIBDIR; \
      fi; \
    fi;

# Build minimal busybox
WORKDIR /
# busybox v1.34.1 stable
RUN git clone -b 1_34_1 --single-branch https://git.busybox.net/busybox
WORKDIR /busybox
ADD busybox.min.config .config
RUN if [ "${TARGETARCH}" = "arm64" ] && [ "${BUILDARCH}" != "arm64" ]; then \
      export CC=aarch64-linux-musl-gcc; \
    elif [ "${TARGETARCH}" = "amd64" ] && [ "${BUILDARCH}" != "amd64" ]; then \
      export CC=x86_64-linux-musl-gcc; \
    fi; \
    make

# Static jq
WORKDIR /
RUN git clone --recursive -b jq-1.6 --single-branch https://github.com/stedolan/jq.git
WORKDIR /jq
RUN autoreconf -fi;\
  ./configure --with-oniguruma=builtin;\ 
  make LDFLAGS=-all-static

FROM boxboat/config-merge:0.2.1 as config-merge

FROM alpine:3

RUN apk add --no-cache \
  curl \
  lz4 \
  nano \
  npm \
  wget \
  zstd-dev

# Install busybox
COPY --from=build-env /busybox/busybox /busybox/busybox

# Install jq
COPY --from=build-env /jq/jq /usr/local/bin/jq

# Add config-merge
COPY --from=config-merge /usr/local/config-merge /usr/local/config-merge
COPY --from=config-merge /usr/local/bin/config-merge /usr/local/bin/config-merge
COPY --from=config-merge /usr/local/bin/envsubst /usr/local/bin/envsubst

# Add dasel.
# The dasel repository does not post checksums of the published binaries,
# so use hardcoded binaries in order to avoid potential supply chain attacks.
# Note, dasel does publish docker images, but only for amd64,
# so we cannot copy the binary out like we do for config-merge.
RUN if [ "$(uname -m)" = "aarch64" ]; then \
      ARCH=arm64 DASELSUM="8e1f95b5f361f68ed8376d5a9593ae4249e28153a05b26f1f99f9466efeac5c9  /usr/local/bin/dasel"; \
    else \
      ARCH=amd64 DASELSUM="3efd202a525c43c027bddc770861dd637ec8389a4ca3ef2951da7165350219ed  /usr/local/bin/dasel"; \
    fi; \
    wget -O /usr/local/bin/dasel https://github.com/TomWright/dasel/releases/download/v1.26.0/dasel_linux_$ARCH && \
      sha256sum -c <(echo "$DASELSUM") && \
      chmod +x /usr/local/bin/dasel
