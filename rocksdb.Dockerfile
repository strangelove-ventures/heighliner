FROM golang:alpine AS rocksdb-build

ARG ROCKSDB_VERSION

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

RUN cd /tmp && \
    git clone https://github.com/facebook/rocksdb.git && \
    cd rocksdb && \
    git checkout ${ROCKSDB_VERSION} && \
    make shared_lib

FROM golang:alpine AS build-env
ARG VERSION
ARG NAME
ARG GITHUB_ORGANIZATION
ARG GITHUB_REPO
ARG BINARY
ARG MAKE_TARGET
ARG BUILD_ENV

# Install rocksdb
COPY --from=rocksdb-build /usr/local/lib/libgflags* /usr/local/lib/
COPY --from=rocksdb-build /tmp/rocksdb/librocksdb.so* /usr/lib/
COPY --from=rocksdb-build /tmp/rocksdb/include/rocksdb /usr/include/rocksdb

RUN apk add --update --no-cache curl make git libc-dev bash gcc linux-headers eudev-dev cmake clang build-base llvm-static llvm-dev clang-static clang-dev

WORKDIR /go/src/github.com/${GITHUB_ORGANIZATION}

RUN git clone https://github.com/${GITHUB_ORGANIZATION}/${GITHUB_REPO}.git

WORKDIR /go/src/github.com/${GITHUB_ORGANIZATION}/${GITHUB_REPO}

ADD https://github.com/CosmWasm/wasmvm/releases/download/v1.0.0-beta5/libwasmvm_muslc.a /lib/libwasmvm_muslc.a
RUN sha256sum /lib/libwasmvm_muslc.a | grep d16a2cab22c75dbe8af32265b9346c6266070bdcf9ed5aa9b7b39a7e32e25fe0

RUN git checkout ${VERSION}

RUN export ${BUILD_ENV} && make ${MAKE_TARGET}

RUN cp ${BINARY} /root/cosmos

FROM alpine:edge

ARG BINARY
ENV BINARY ${BINARY}

RUN apk add --no-cache ca-certificates libstdc++ libgcc curl jq git

WORKDIR /root

# Install rocksdb
COPY --from=rocksdb-build /usr/local/lib/libgflags* /usr/local/lib/
COPY --from=rocksdb-build /tmp/rocksdb/librocksdb.so* /usr/lib/
COPY --from=rocksdb-build /tmp/rocksdb/include/rocksdb /usr/include/rocksdb

# Install go (needed by osmosis)
COPY --from=golang:alpine /usr/local/go/ /usr/local/go/
ENV PATH="/usr/local/go/bin:${PATH}"

COPY --from=build-env /root/cosmos .

RUN mv /root/cosmos /usr/bin/$(basename $BINARY)

EXPOSE 26657

CMD env $(basename $BINARY) start
