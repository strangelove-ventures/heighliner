FROM golang:bullseye AS build-env

ARG PRE_BUILD
ARG VERSION
RUN export VERSION=${VERSION} && sh -c "${PRE_BUILD}"

RUN mkdir /root/bin
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

RUN mkdir /root/lib
ARG LIBRARIES
ENV LIBRARIES_ENV ${LIBRARIES}
RUN bash -c 'LIBRARIES_ARR=($LIBRARIES_ENV); for LIBRARY in "${LIBRARIES_ARR[@]}"; do cp $LIBRARY /root/lib/; done'

FROM debian:bullseye

LABEL org.opencontainers.image.source="https://github.com/p2p-org/cosmos-heighliner"

# Install binaries
COPY --from=build-env /root/bin /usr/bin

# Install libraries
COPY --from=build-env /root/lib /usr/lib

RUN groupadd -g 1111 -r p2p && useradd -u 1111 --no-log-init -r -g p2p p2p
WORKDIR /home/p2p
RUN chown -R p2p:p2p /home/p2p
# USER p2p
