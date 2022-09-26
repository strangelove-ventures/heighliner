FROM --platform=$BUILDPLATFORM golang:1.19-alpine

RUN apk add --update --no-cache curl make git libc-dev bash gcc linux-headers eudev-dev

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

RUN addgroup --gid 1025 -S heighliner && adduser --uid 1025 -S heighliner -G heighliner
