FROM alpine:3.21

ARG TARGETARCH

COPY dist/linux-${TARGETARCH}/copilotpi-linux-${TARGETARCH} /usr/local/bin/copilotpi
COPY config.defaults.toml /app/config.defaults.toml
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

RUN apk add --no-cache ca-certificates tzdata && \
    chmod +x /usr/local/bin/copilotpi /usr/local/bin/docker-entrypoint.sh && \
    adduser -D -u 1000 copilotpi && \
    mkdir -p /app/data && \
    chown -R copilotpi:copilotpi /app

USER copilotpi

WORKDIR /app
VOLUME ["/app/data"]
EXPOSE 8080

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["-config", "/app/config.toml"]
