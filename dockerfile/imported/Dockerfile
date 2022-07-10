ARG BASE_IMAGE
ARG VERSION
FROM $BASE_IMAGE:$VERSION

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

# supports either debian based or alpine based
RUN (apt update && apt install -y ca-certificates jq curl git gcc nano lz4 wget unzip) || \
  apk add --no-cache ca-certificates jq curl git gcc nano lz4 wget unzip

RUN addgroup --gid 1025 -S heighliner || groupadd -g 1025 -r heighliner
RUN adduser --uid 1025 -S heighliner -G heighliner || useradd -u 1025 --no-log-init -r -g heighliner heighliner
WORKDIR /home/heighliner
USER heighliner

ENTRYPOINT []
CMD []
