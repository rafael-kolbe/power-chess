FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/power-chess-server ./cmd/server

FROM alpine:3.21

RUN adduser -D -H appuser
USER appuser
WORKDIR /app

COPY --from=builder /bin/power-chess-server /app/power-chess-server
COPY --from=builder /app/web /app/web

EXPOSE 8080
ENTRYPOINT ["/app/power-chess-server"]

