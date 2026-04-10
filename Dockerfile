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

RUN adduser -D -H appuser
USER appuser
WORKDIR /app

COPY --from=builder /bin/power-chess-server /app/power-chess-server
COPY --from=builder /app/web /app/web

EXPOSE 8080
ENTRYPOINT ["/app/power-chess-server"]
