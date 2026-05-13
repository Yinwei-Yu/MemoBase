# MemoBase Project Setup

## Prerequisites

```bash
# protoc (macOS)
brew install protobuf

# Go protobuf plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

## Proto Generation

Run from project root after changing `agent-service/proto/agent.proto`:

```bash
protoc \
  --plugin=protoc-gen-go=$(go env GOPATH)/bin/protoc-gen-go \
  --plugin=protoc-gen-go-grpc=$(go env GOPATH)/bin/protoc-gen-go-grpc \
  -I=. \
  --go_out=backend \
  --go_opt=module=memobase/backend \
  --go-grpc_out=backend \
  --go-grpc_opt=module=memobase/backend \
  agent-service/proto/agent.proto
```

Generated files: `backend/proto/agent.pb.go`, `backend/proto/agent_grpc.pb.go`

## Dev Commands

```bash
# Backend
cd backend && go mod tidy && go run ./cmd/server

# Frontend
cd frontend && npm install && npm run dev

# Tests
cd backend && go test ./internal/...
```
