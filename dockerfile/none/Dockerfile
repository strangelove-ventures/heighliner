FROM golang:bullseye AS build-env

ARG PRE_BUILD
ARG VERSION
RUN export VERSION=${VERSION} && sh -c "${PRE_BUILD}"

RUN mkdir /root/bin
ARG BINARIES
ENV BINARIES_ENV ${BINARIES}
RUN bash -c 'BINARIES_ARR=($BINARIES_ENV); for BINARY in "${BINARIES_ARR[@]}"; do cp $BINARY /root/bin/; done'

RUN mkdir /root/lib
ARG LIBRARIES
ENV LIBRARIES_ENV ${LIBRARIES}
RUN bash -c 'LIBRARIES_ARR=($LIBRARIES_ENV); for LIBRARY in "${LIBRARIES_ARR[@]}"; do cp $LIBRARY /root/lib/; done'

FROM debian:bullseye

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

# Install binaries
COPY --from=build-env /root/bin /usr/bin

# Install libraries
COPY --from=build-env /root/lib /usr/lib

RUN groupadd -g 1025 -r heighliner && useradd -u 1025 --no-log-init -r -g heighliner heighliner
WORKDIR /home/heighliner
RUN chown -R heighliner:heighliner /home/heighliner
USER heighliner
