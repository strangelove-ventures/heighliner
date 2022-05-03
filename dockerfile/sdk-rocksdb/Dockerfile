FROM golang:alpine AS rocksdb-build

RUN echo "@testing http://nl.alpinelinux.org/alpine/edge/testing" >>/etc/apk/repositories
RUN apk add --update --no-cache build-base linux-headers git cmake bash perl #wget mercurial g++ autoconf libgflags-dev cmake \
  bash zlib zlib-dev bzip2 bzip2-dev snappy snappy-dev lz4 lz4-dev zstd@testing zstd-dev@testing libtbb-dev@testing libtbb@testing

RUN cd /tmp && \
    git clone https://github.com/gflags/gflags.git && \
    cd gflags && \
    mkdir build && \
    cd build && \
    cmake -DBUILD_SHARED_LIBS=1 -DGFLAGS_INSTALL_SHARED_LIBS=1 .. && \
    make install && \
    cd /tmp && \
    rm -R /tmp/gflags/

ARG ROCKSDB_VERSION

RUN cd /tmp && \
    git clone https://github.com/facebook/rocksdb.git && \
    cd rocksdb && \
    git checkout ${ROCKSDB_VERSION} && \
    make shared_lib

FROM golang:alpine AS build-env

# Install rocksdb
COPY --from=rocksdb-build /usr/local/lib/libgflags* /usr/local/lib/
COPY --from=rocksdb-build /tmp/rocksdb/librocksdb.so* /usr/lib/
COPY --from=rocksdb-build /tmp/rocksdb/include/rocksdb /usr/include/rocksdb

RUN apk add --update --no-cache curl make git libc-dev bash gcc linux-headers eudev-dev cmake clang build-base llvm-static llvm-dev clang-static clang-dev

RUN git clone https://github.com/notional-labs/tendermint && \
  cd tendermint && \
  git checkout remotes/origin/callum/app-version && \
  go install -tags rocksdb ./... 

ARG GITHUB_ORGANIZATION

WORKDIR /go/src/github.com/${GITHUB_ORGANIZATION}

ARG GITHUB_REPO

RUN git clone https://github.com/${GITHUB_ORGANIZATION}/${GITHUB_REPO}.git

WORKDIR /go/src/github.com/${GITHUB_ORGANIZATION}/${GITHUB_REPO}

RUN wget -O /lib/libwasmvm_muslc.a https://github.com/CosmWasm/wasmvm/releases/download/v1.0.0-beta10/libwasmvm_muslc.$(uname -m).a

ARG VERSION

RUN git checkout ${VERSION}

ARG BUILD_TARGET
ARG BUILD_ENV
ARG BUILD_TAGS
ARG PRE_BUILD

RUN [ ! -z "$PRE_BUILD" ] && sh -c "${PRE_BUILD}"; \
    [ ! -z "$BUILD_ENV" ] && export ${BUILD_ENV}; \
    [ ! -z "$BUILD_TAGS" ] && export "${BUILD_TAGS}"; \
    make ${BUILD_TARGET}

RUN mkdir /root/bin

ARG BINARIES

ENV BINARIES_ENV ${BINARIES}
RUN bash -c 'BINARIES_ARR=($BINARIES_ENV); for BINARY in "${BINARIES_ARR[@]}"; do cp $BINARY /root/bin/ ; done'

FROM alpine:edge

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

ARG BINARY
ENV BINARY ${BINARY}

RUN apk add --no-cache ca-certificates libstdc++ libgcc curl jq git nano lz4 wget

WORKDIR /root

# Install rocksdb
COPY --from=rocksdb-build /usr/local/lib/libgflags* /usr/local/lib/
COPY --from=rocksdb-build /tmp/rocksdb/librocksdb.so* /usr/lib/
COPY --from=rocksdb-build /tmp/rocksdb/include/rocksdb /usr/include/rocksdb

# Install tendermint
COPY --from=build-env /go/bin/tendermint /usr/bin/

# Install chain binary
COPY --from=build-env /root/cosmos .
RUN mv /root/cosmos /usr/bin/$(basename $BINARY)

RUN addgroup -S heighliner && adduser -S heighliner -G heighliner
WORKDIR /home/heighliner
USER heighliner
