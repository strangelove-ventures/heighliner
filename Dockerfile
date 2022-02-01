FROM golang:alpine AS build-env
ARG VERSION
ARG NAME
ARG GITHUB_ORGANIZATION
ARG GITHUB_REPO
ARG BINARY
ARG MAKE_TARGET

ENV PACKAGES curl make git libc-dev bash gcc linux-headers eudev-dev

RUN apk add --no-cache $PACKAGES

WORKDIR /go/src/github.com/${GITHUB_ORGANIZATION}

RUN git clone https://github.com/${GITHUB_ORGANIZATION}/${GITHUB_REPO}.git

WORKDIR /go/src/github.com/${GITHUB_ORGANIZATION}/${GITHUB_REPO}

RUN git checkout ${VERSION} && make ${MAKE_TARGET}

FROM alpine:edge

RUN apk add --no-cache ca-certificates
WORKDIR /root

COPY --from=build-env /go/src/github.com/${GITHUB_ORGANIZATION}/${GITHUB_REPO}/${BINARY} /usr/bin/

WORKDIR /${NAME}

RUN echo "$(basename $BINARY)" > /${NAME}/startup.sh

RUN chmod +x /${NAME}/startup.sh

USER root

EXPOSE 26657

ENTRYPOINT [ "./startup.sh" ]
