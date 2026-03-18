FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN go build -o memobase-server ./cmd/server

FROM alpine:3.20
WORKDIR /app
RUN adduser -D app
USER app
COPY --from=builder /app/memobase-server /app/memobase-server
COPY --from=builder /app/storage /app/storage
EXPOSE 8080
CMD ["/app/memobase-server"]
