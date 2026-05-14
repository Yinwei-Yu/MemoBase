#!/usr/bin/env bash
set -euo pipefail

# ─── Colors ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}[INFO]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()   { echo -e "${RED}[ERROR]${NC} $*"; }

cd "$(dirname "$0")"

# ─── 1. Check prerequisites ──────────────────────────────────────────────────
info "Checking prerequisites..."

check_cmd() {
    if ! command -v "$1" &>/dev/null; then
        err "$1 not found. $2"
        return 1
    fi
    ok "$1 found"
}

check_cmd docker "Install Docker Desktop: https://docker.com/products/docker-desktop"

# ─── 1b. Check & start Ollama ────────────────────────────────────────────────
info "Checking Ollama..."

if ! command -v ollama &>/dev/null; then
    err "ollama not found. Install: https://ollama.com/download"
    exit 1
fi
ok "ollama found"

if curl -sf http://localhost:11434/api/tags &>/dev/null; then
    ok "Ollama server already running"
else
    warn "Ollama server not running, attempting to start..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        open -a Ollama 2>/dev/null || ollama serve &
    else
        ollama serve &
    fi
    # Wait for server
    for i in $(seq 1 30); do
        if curl -sf http://localhost:11434/api/tags &>/dev/null; then
            ok "Ollama server started"
            break
        fi
        sleep 1
    done
    if ! curl -sf http://localhost:11434/api/tags &>/dev/null; then
        err "Ollama server failed to start"
        exit 1
    fi
fi

# Check docker compose (v2 plugin or standalone)
if docker compose version &>/dev/null 2>&1; then
    ok "docker compose (v2 plugin)"
elif command -v docker-compose &>/dev/null; then
    ok "docker-compose (standalone)"
    # alias for later use
    docker_compose() { docker-compose "$@"; }
else
    err "docker compose not found. Install Docker Desktop or docker-compose-plugin"
    exit 1
fi

# ─── 2. Install protoc & Go plugins (if missing) ────────────────────────────
install_proto=false

if ! command -v protoc &>/dev/null; then
    warn "protoc not found"
    if [[ "$OSTYPE" == "darwin"* ]]; then
        info "Installing protoc via Homebrew..."
        brew install protobuf
    else
        err "Install protoc manually: https://grpc.io/docs/protoc-installation/"
        exit 1
    fi
fi
ok "protoc found"

if ! command -v protoc-gen-go &>/dev/null; then
    info "Installing protoc-gen-go..."
    go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
    install_proto=true
fi
ok "protoc-gen-go found"

if ! command -v protoc-gen-go-grpc &>/dev/null; then
    info "Installing protoc-gen-go-grpc..."
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
    install_proto=true
fi
ok "protoc-gen-go-grpc found"

# ─── 3. Generate proto files ────────────────────────────────────────────────
PROTO_FILE="agent-service/proto/agent.proto"
if [[ -f "$PROTO_FILE" ]]; then
    info "Generating protobuf Go code..."
    protoc \
        --plugin=protoc-gen-go="$(go env GOPATH)/bin/protoc-gen-go" \
        --plugin=protoc-gen-go-grpc="$(go env GOPATH)/bin/protoc-gen-go-grpc" \
        -I=. \
        --go_out=backend \
        --go_opt=module=memobase/backend \
        --go-grpc_out=backend \
        --go-grpc_opt=module=memobase/backend \
        "$PROTO_FILE"
    ok "Proto files generated"
else
    warn "$PROTO_FILE not found, skipping proto generation"
fi

# ─── 4. Stop existing containers & wipe DB volume ───────────────────────────
info "Stopping existing containers and wiping DB volume..."
docker compose down -v 2>/dev/null || true
ok "Clean slate"

# ─── 5. Build & start all services ──────────────────────────────────────────
info "Building and starting all services..."
docker compose up --build -d

# List available Ollama models
ollama_models=$(curl -sf http://localhost:11434/api/tags 2>/dev/null | python3 -c "import sys,json; models=json.load(sys.stdin).get('models',[]); print(', '.join(m['name'] for m in models) if models else '(none)')" 2>/dev/null || echo "(unknown)")

info "Waiting for services to become healthy..."
timeout=120
elapsed=0
while [[ $elapsed -lt $timeout ]]; do
    healthy=$(docker compose ps --format json 2>/dev/null | grep -c '"healthy"' || true)
    total=$(docker compose ps --format json 2>/dev/null | wc -l | tr -d ' ')
    if [[ "$healthy" -ge 3 ]]; then
        break
    fi
    sleep 3
    elapsed=$((elapsed + 3))
done

# ─── 6. Print status ────────────────────────────────────────────────────────
echo ""
echo "════════════════════════════════════════════════════════════════"
echo ""
ok "MemoBase is running!"
echo ""
echo -e "  ${CYAN}Frontend${NC}        http://localhost:5173"
echo -e "  ${CYAN}Backend API${NC}     http://localhost:8080/api/v1"
echo -e "  ${CYAN}API Health${NC}      http://localhost:8080/api/v1/readyz"
echo -e "  ${CYAN}Grafana${NC}         http://localhost:3000  (admin / admin)"
echo -e "  ${CYAN}Prometheus${NC}      http://localhost:9090"
echo -e "  ${CYAN}Qdrant${NC}          http://localhost:6333"
echo -e "  ${CYAN}Ollama${NC}          http://localhost:11434 (local, not in Docker)"
echo -e "  ${CYAN}Agent gRPC${NC}      localhost:50051"
echo ""
echo -e "  ${YELLOW}Demo login:${NC}  demo / demo123"
echo ""
echo -e "  ${CYAN}Ollama models:${NC}  $ollama_models"
echo -e "  ${YELLOW}Tip:${NC} Pull models as needed: ${CYAN}ollama pull <model>${NC}"
echo ""
echo "════════════════════════════════════════════════════════════════"
echo ""
echo -e "Run ${CYAN}docker compose logs -f${NC} to follow logs"
echo -e "Run ${CYAN}docker compose down -v${NC} to stop and wipe data"
echo ""
