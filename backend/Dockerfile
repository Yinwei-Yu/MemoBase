FROM golang:1.25-alpine AS builder
WORKDIR /app
RUN apk --no-cache add ca-certificates tzdata
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/memobase-server ./cmd/server

FROM alpine:3.20
WORKDIR /app
RUN apk --no-cache add ca-certificates tzdata wget \
    && addgroup -S app \
    && adduser -S -G app app
RUN mkdir -p /app/storage && chown -R app:app /app
USER app
COPY --from=builder /out/memobase-server /app/memobase-server
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -q -O- http://127.0.0.1:8080/api/v1/healthz >/dev/null || exit 1
ENTRYPOINT ["/app/memobase-server"]
