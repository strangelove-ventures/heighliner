FROM ghcr.io/strangelove-ventures/infra-toolkit:v0.1.0 AS infra-toolkit
RUN addgroup --gid 1025 -S heighliner && adduser --uid 1025 -S heighliner -G heighliner

FROM ghcr.io/agoric/agoric-3-proposals:main

LABEL org.opencontainers.image.source="https://github.com/strangelove-ventures/heighliner"

ARG FINAL_IMAGE
RUN if [ ! -z "$FINAL_IMAGE" ]; then sh -c "$FINAL_IMAGE"; fi

# Install trusted CA certificates
COPY --from=infra-toolkit /etc/ssl/cert.pem /etc/ssl/cert.pem

# Install heighliner user
COPY --from=infra-toolkit /etc/passwd /etc/passwd
COPY --from=infra-toolkit --chown=1025:1025 /home/heighliner /home/heighliner

ENTRYPOINT []
CMD []

WORKDIR /home/heighliner
USER heighliner
