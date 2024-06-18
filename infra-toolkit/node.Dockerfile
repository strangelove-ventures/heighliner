FROM alpine AS builder
RUN apk add git python3 gcc g++ linux-headers make
RUN git clone https://github.com/nodejs/node --single-branch --branch v18.16.0 && \
  cd node && \
  ./configure --fully-static --enable-static && \
  make -j$(nproc)

RUN cd node && make install

FROM scratch
LABEL org.opencontainers.image.source="https://github.com/p2p-org/cosmos-heighliner"
COPY --from=builder /usr/local /usr/local
