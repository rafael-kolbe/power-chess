FROM golang:1.26-alpine AS builder

WORKDIR /app

# Vendored modules: the image builds without contacting proxy.golang.org (works offline / flaky DNS).
COPY go.mod go.sum ./
COPY vendor ./vendor

COPY cmd ./cmd
COPY internal ./internal
COPY web ./web

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=vendor -trimpath -o /bin/power-chess-server ./cmd/server

FROM alpine:3.21

RUN adduser -D -H appuser && apk add --no-cache su-exec
WORKDIR /app

COPY scripts/session-log-wrap.sh /app/session-log-wrap.sh
RUN chmod 755 /app/session-log-wrap.sh

COPY --from=builder /bin/power-chess-server /app/power-chess-server
COPY --from=builder /app/web /app/web

# Entrypoint runs as root so tee can create files on bind-mounted log dirs; the server drops to appuser (see SESSION_LOG_RUN_AS).
ENV SESSION_LOG_RUN_AS=appuser
EXPOSE 8080
ENTRYPOINT ["/app/session-log-wrap.sh"]
CMD ["/app/power-chess-server"]
