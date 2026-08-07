FROM golang:1.25-alpine3.21 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/server ./cmd/server/

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata curl && \
    addgroup -g 10001 -S app && \
    adduser -u 10001 -S -G app app

ADD https://github.com/golang-migrate/migrate/releases/download/v4.18.1/migrate.linux-amd64.tar.gz /tmp/migrate.tar.gz
RUN tar -xzf /tmp/migrate.tar.gz -C /usr/local/bin migrate && \
    chmod +x /usr/local/bin/migrate && \
    rm /tmp/migrate.tar.gz

WORKDIR /app

COPY --from=builder --chown=app:app /app/bin/server .
COPY --from=builder --chown=app:app /app/migrations ./migrations
COPY --from=builder --chown=app:app /app/docs ./docs
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

USER app

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -sf http://localhost:${PORT:-8080}/health || exit 1

ENTRYPOINT ["./entrypoint.sh"]
