.PHONY: help install build build-frontend build-backend test dev dev-mobile dev1 dev2 dev-mock dev-mock1 dev-mock2 dev-cli clean lint fmt docker

# Default target
help:
	@echo "OpenBooks Development Commands"
	@echo ""
	@echo "Setup:"
	@echo "  make install          Install all dependencies (Go + npm)"
	@echo ""
	@echo "Development (real IRC server - recommended):"
	@echo "  make dev              Run backend + frontend (connects to irc.irchighway.net)"
	@echo "  make dev-mobile       Same as dev, but frontend accessible from LAN (for phone testing)"
	@echo "  make dev1             Backend only  (Terminal 1)"
	@echo "  make dev2             Frontend only (Terminal 2)"
	@echo ""
	@echo "Development (mock IRC - for offline testing):"
	@echo "  make dev-mock         Run mock + backend + frontend"
	@echo "  make dev-mock1        Mock server   (Terminal 1)"
	@echo "  make dev-mock2        Backend       (Terminal 2)"
	@echo "  make dev2             Frontend      (Terminal 3)"
	@echo ""
	@echo "Build:"
	@echo "  make build            Build everything (frontend + backend)"
	@echo "  make build-frontend   Build React frontend only"
	@echo "  make build-backend    Build Go backend only"
	@echo ""
	@echo "Quality:"
	@echo "  make test             Run all tests"
	@echo "  make test-go          Run Go tests"
	@echo "  make test-frontend    Run frontend tests"
	@echo "  make lint             Run linters"
	@echo "  make fmt              Format code"
	@echo ""
	@echo "Other:"
	@echo "  make dev-cli          Run OpenBooks in CLI mode"
	@echo "  make docker           Build Docker image"
	@echo "  make clean            Clean build artifacts"

# Setup
install: install-go install-npm

install-go:
	go mod download

install-npm:
	cd server/app && npm install

# =============================================================================
# DEVELOPMENT
# =============================================================================
# Real IRC server (recommended):
#   make dev          - Backend + frontend, connects to irc.irchighway.net
#   make dev1 + dev2  - Same but in separate terminals (for debugging)
#
# Mock IRC server (offline testing):
#   make dev-mock           - Mock + backend + frontend, all in one terminal
#   make dev-mock1 + dev-mock2 + dev2  - Separate terminals
#
# Note: Frontend must be built before backend (backend embeds the frontend).
#       The targets handle this automatically via build-frontend dependency.
# =============================================================================

# Default dev username (override with: make dev NAME=myname)
NAME ?= openbooks_dev

# Option A: All-in-one with REAL IRC server (recommended)
dev: build-frontend install-npm
	@echo "Starting backend + frontend (connecting to real IRC server)..."
	@echo "Press Ctrl+C to stop all services"
	@echo ""
	@echo "Services:"
	@echo "  IRC:       irc.irchighway.net:6697 (real server)"
	@echo "  Backend:   localhost:5228"
	@echo "  Frontend:  localhost:5173"
	@echo "  Username:  $(NAME)"
	@echo ""
	@trap 'kill 0' EXIT; \
	(cd cmd/openbooks && echo "[backend] Building..." && go build && echo "[backend] Starting..." && ./openbooks server --name $(NAME) 2>&1 | sed 's/^/[backend] /') & \
	sleep 2 && (cd server/app && echo "[frontend] Starting..." && npm run dev 2>&1 | sed 's/^/[frontend] /') & \
	wait

# Option A2: Same as dev but accessible from LAN (for mobile testing)
dev-mobile: build-frontend install-npm
	@echo "Starting backend + frontend (accessible from LAN)..."
	@echo "Press Ctrl+C to stop all services"
	@echo ""
	@LOCAL_IP=$$(ipconfig getifaddr en0 2>/dev/null || hostname -I 2>/dev/null | awk '{print $$1}'); \
	echo "Services:"; \
	echo "  IRC:       irc.irchighway.net:6697 (real server)"; \
	echo "  Backend:   $$LOCAL_IP:5228"; \
	echo "  Frontend:  $$LOCAL_IP:5173  <-- Open this on your phone"; \
	echo "  Username:  $(NAME)"; \
	echo ""; \
	trap 'kill 0' EXIT; \
	(cd cmd/openbooks && echo "[backend] Building..." && go build && echo "[backend] Starting..." && ./openbooks server --name $(NAME) 2>&1 | sed 's/^/[backend] /') & \
	sleep 2 && (cd server/app && echo "[frontend] Starting..." && npm run dev -- --host 2>&1 | sed 's/^/[frontend] /') & \
	wait

# Option B: All-in-one with MOCK IRC server (for offline testing)
dev-mock: build-frontend install-npm
	@echo "Starting mock + backend + frontend..."
	@echo "Press Ctrl+C to stop all services"
	@echo ""
	@echo "Services:"
	@echo "  Mock IRC:  localhost:6667"
	@echo "  Backend:   localhost:5228"
	@echo "  Frontend:  localhost:5173"
	@echo "  Username:  $(NAME)"
	@echo ""
	@trap 'kill 0' EXIT; \
	(cd cmd/mock_server && echo "[mock] Starting..." && go run . 2>&1 | sed 's/^/[mock] /') & \
	sleep 2 && (cd cmd/openbooks && echo "[backend] Building..." && go build && echo "[backend] Starting..." && ./openbooks server --name $(NAME) --tls=false --server localhost:6667 2>&1 | sed 's/^/[backend] /') & \
	sleep 3 && (cd server/app && echo "[frontend] Starting..." && npm run dev 2>&1 | sed 's/^/[frontend] /') & \
	wait

# Option C: Separate terminals (for debugging - run in order: dev1 -> dev2)
dev1: build-frontend
	@echo "Starting Backend on :5228 (real IRC server)..."
	@echo "Username: $(NAME) (override with: make dev1 NAME=myname)"
	@echo "Once ready, run 'make dev2' in another terminal"
	cd cmd/openbooks && go build && ./openbooks server --name $(NAME)

dev2: install-npm
	@echo "Starting Frontend on :5173..."
	@echo "Open http://localhost:5173 in your browser"
	cd server/app && npm run dev

# For mock server testing (run: dev-mock1 -> dev-mock2 -> dev2)
dev-mock1:
	@echo "Starting Mock IRC server on :6667..."
	@echo "Wait for 'waiting' message, then run 'make dev-mock2' in another terminal"
	cd cmd/mock_server && go run .

dev-mock2: build-frontend
	@echo "Starting Backend on :5228 (mock IRC)..."
	@echo "Username: $(NAME)"
	@echo "Once ready, run 'make dev2' in another terminal"
	cd cmd/openbooks && go build && ./openbooks server --name $(NAME) --tls=false --server localhost:6667

dev-cli:
	cd cmd/openbooks && go build && ./openbooks cli --tls=false --server localhost:6667

# Build
build: build-frontend build-backend

build-frontend:
	cd server/app && npm run build

build-backend:
	cd cmd/openbooks && go build -o openbooks

build-release:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o openbooks ./cmd/openbooks

# Test
test: test-go test-frontend

test-go:
	go test -v ./...

test-frontend:
	cd server/app && npm test --if-present

# Quality
lint: lint-go lint-frontend

lint-go:
	go vet ./...
	@which golangci-lint > /dev/null && golangci-lint run || echo "golangci-lint not installed, skipping"

lint-frontend:
	cd server/app && npm run lint --if-present

fmt: fmt-go fmt-frontend

fmt-go:
	go fmt ./...

fmt-frontend:
	cd server/app && npm run format --if-present || npx prettier --write "src/**/*.{ts,tsx}" 2>/dev/null || true

# Docker
docker:
	docker build -t openbooks .

docker-run:
	docker run -p 8080:80 -v $(PWD)/books:/books openbooks

# Clean
clean:
	rm -f cmd/openbooks/openbooks
	rm -rf server/app/dist
	rm -rf server/app/node_modules/.cache
