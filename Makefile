.PHONY: help install install-go install-npm \
        dev dev-mobile dev-mock dev-mock-mobile \
        dev1 dev2 dev-mock1 dev-mock2 dev-cli \
        build build-frontend build-backend build-release \
        test test-go test-frontend type-check \
        lint lint-go lint-frontend \
        fmt fmt-go fmt-frontend \
        docker docker-run clean

# ─── Default target ──────────────────────────────────────────────────────────
help:
	@echo ""
	@echo "  OpenBooks — Development Commands"
	@echo "  Vue 3 frontend + Go backend"
	@echo ""
	@echo "  ── Setup ──────────────────────────────────────────────"
	@echo "  make install           Install all dependencies (Go + npm)"
	@echo ""
	@echo "  ── Dev: Real IRC (irc.irchighway.net) ─────────────────"
	@echo "  make dev               Backend + frontend in one terminal"
	@echo "  make dev-mobile        Same, but LAN-accessible (for phone testing)"
	@echo "  make dev1              Backend only   (Terminal 1)"
	@echo "  make dev2              Frontend only  (Terminal 2)"
	@echo ""
	@echo "  ── Dev: Mock IRC (offline, no IRC account needed) ──────"
	@echo "  make dev-mock          Mock + backend + frontend in one terminal"
	@echo "  make dev-mock-mobile   Same, but LAN-accessible (for phone testing)"
	@echo "  make dev-mock1         Mock IRC server  (Terminal 1)"
	@echo "  make dev-mock2         Backend          (Terminal 2)"
	@echo "  make dev2              Frontend         (Terminal 3)"
	@echo ""
	@echo "  ── Build ───────────────────────────────────────────────"
	@echo "  make build             Build everything (frontend + backend)"
	@echo "  make build-frontend    Vue frontend only  → server/app/dist/"
	@echo "  make build-backend     Go backend only    → cmd/openbooks/openbooks"
	@echo "  make build-release     Stripped release binary"
	@echo ""
	@echo "  ── Quality ─────────────────────────────────────────────"
	@echo "  make type-check        Run vue-tsc type checking"
	@echo "  make test              Run all tests (Go + frontend)"
	@echo "  make test-go           Go tests only"
	@echo "  make test-frontend     Frontend tests only (if configured)"
	@echo "  make lint              Run all linters"
	@echo "  make fmt               Format all code"
	@echo ""
	@echo "  ── Other ───────────────────────────────────────────────"
	@echo "  make dev-cli           OpenBooks CLI mode (mock IRC)"
	@echo "  make docker            Build Docker image"
	@echo "  make docker-run        Run Docker image on :8080"
	@echo "  make clean             Remove build artifacts"
	@echo ""
	@echo "  Override username: make dev NAME=myuser  (default: openbooks_dev)"
	@echo ""

# ─── Setup ───────────────────────────────────────────────────────────────────
install: install-go install-npm

install-go:
	go mod download

install-npm:
	cd server/app && npm install

# =============================================================================
# DEVELOPMENT
# =============================================================================
# Real IRC server:
#   make dev            - All-in-one (recommended)
#   make dev-mobile     - All-in-one, exposed on LAN for phone/tablet testing
#   make dev1 + dev2    - Separate terminals (easier log reading)
#
# Mock IRC server (offline, no IRC credentials needed):
#   make dev-mock            - All-in-one
#   make dev-mock-mobile     - All-in-one, LAN-exposed for phone/tablet testing
#   make dev-mock1 + dev-mock2 + dev2  - Separate terminals
#
# Note: Go backend embeds server/app/dist at compile time. The all-in-one
#       targets build the frontend first, then run both services concurrently.
#       The frontend Vite dev server proxies WS to :5228 automatically.
# =============================================================================

# Default username (override with: make dev NAME=myname)
NAME ?= openbooks_dev

# ── All-in-one: Real IRC ──────────────────────────────────────────────────────
dev: build-frontend install-npm
	@echo ""
	@echo "  IRC:      irc.irchighway.net:6697"
	@echo "  Backend:  http://localhost:5228"
	@echo "  Frontend: http://localhost:5173  ← open in browser"
	@echo "  Username: $(NAME)"
	@echo ""
	@trap 'kill 0' EXIT; \
	(cd cmd/openbooks && go build && ./openbooks server --name $(NAME) --dir $(CURDIR)/books --persist --organize-downloads 2>&1 | sed 's/^/[backend] /') & \
	sleep 2 && (cd server/app && npm run dev 2>&1 | sed 's/^/[frontend] /') & \
	wait

# ── All-in-one: Real IRC, LAN-exposed (mobile/tablet) ────────────────────────
dev-mobile: build-frontend install-npm
	@echo ""
	@LOCAL_IP=$$(ipconfig getifaddr en0 2>/dev/null || hostname -I 2>/dev/null | awk '{print $$1}' || echo "YOUR_IP"); \
	echo "  IRC:      irc.irchighway.net:6697"; \
	echo "  Backend:  http://$$LOCAL_IP:5228"; \
	echo "  Frontend: http://$$LOCAL_IP:5173  ← open on your phone/tablet"; \
	echo "  Username: $(NAME)"; \
	echo ""; \
	trap 'kill 0' EXIT; \
	(cd cmd/openbooks && go build && ./openbooks server --name $(NAME) --dir $(CURDIR)/books --persist --organize-downloads 2>&1 | sed 's/^/[backend] /') & \
	sleep 2 && (cd server/app && npm run dev -- --host 2>&1 | sed 's/^/[frontend] /') & \
	wait

# ── All-in-one: Mock IRC ──────────────────────────────────────────────────────
dev-mock: build-frontend install-npm
	@echo ""
	@echo "  Mock IRC: localhost:6667  (offline, no IRC account needed)"
	@echo "  Backend:  http://localhost:5228"
	@echo "  Frontend: http://localhost:5173  ← open in browser"
	@echo "  Username: $(NAME)"
	@echo ""
	@trap 'kill 0' EXIT; \
	(cd cmd/mock_server && go run . 2>&1 | sed 's/^/[mock]     /') & \
	sleep 2 && (cd cmd/openbooks && go build && ./openbooks server --name $(NAME) --tls=false --server localhost:6667 2>&1 | sed 's/^/[backend]  /') & \
	sleep 3 && (cd server/app && npm run dev 2>&1 | sed 's/^/[frontend] /') & \
	wait

# ── All-in-one: Mock IRC, LAN-exposed (mobile/tablet) ────────────────────────
dev-mock-mobile: build-frontend install-npm
	@echo ""
	@LOCAL_IP=$$(ipconfig getifaddr en0 2>/dev/null || hostname -I 2>/dev/null | awk '{print $$1}' || echo "YOUR_IP"); \
	echo "  Mock IRC: localhost:6667  (offline, no IRC account needed)"; \
	echo "  Backend:  http://$$LOCAL_IP:5228"; \
	echo "  Frontend: http://$$LOCAL_IP:5173  ← open on your phone/tablet"; \
	echo "  Username: $(NAME)"; \
	echo ""; \
	trap 'kill 0' EXIT; \
	(cd cmd/mock_server && go run . 2>&1 | sed 's/^/[mock]     /') & \
	sleep 2 && (cd cmd/openbooks && go build && ./openbooks server --name $(NAME) --tls=false --server localhost:6667 2>&1 | sed 's/^/[backend]  /') & \
	sleep 3 && (cd server/app && npm run dev -- --host 2>&1 | sed 's/^/[frontend] /') & \
	wait

# ── Separate terminals: Real IRC ──────────────────────────────────────────────
dev1: build-frontend
	@echo "Backend → :5228  (username: $(NAME))"
	@echo "Run 'make dev2' in another terminal once backend is ready."
	cd cmd/openbooks && go build && ./openbooks server --name $(NAME) --dir $(CURDIR)/books --persist --organize-downloads

dev2: install-npm
	@echo "Frontend → :5173  (http://localhost:5173)"
	cd server/app && npm run dev

# ── Separate terminals: Mock IRC ──────────────────────────────────────────────
dev-mock1:
	@echo "Mock IRC → :6667"
	@echo "Run 'make dev-mock2' in another terminal once mock is ready."
	cd cmd/mock_server && go run .

dev-mock2: build-frontend
	@echo "Backend → :5228  (mock IRC, username: $(NAME))"
	@echo "Run 'make dev2' in another terminal once backend is ready."
	cd cmd/openbooks && go build && ./openbooks server --name $(NAME) --tls=false --server localhost:6667

# ── CLI mode ──────────────────────────────────────────────────────────────────
dev-cli:
	cd cmd/openbooks && go build && ./openbooks cli --tls=false --server localhost:6667

# ─── Build ───────────────────────────────────────────────────────────────────
build: build-frontend build-backend

build-frontend:
	@echo "Building Vue frontend → server/app/dist/"
	cd server/app && npm run build

build-backend:
	@echo "Building Go backend → cmd/openbooks/openbooks"
	cd cmd/openbooks && go build -o openbooks

build-release:
	@echo "Building stripped release binary → ./openbooks"
	CGO_ENABLED=0 go build -ldflags="-s -w" -o openbooks ./cmd/openbooks

# ─── Test & Quality ──────────────────────────────────────────────────────────
test: test-go test-frontend

test-go:
	go test -v ./...

test-frontend:
	cd server/app && npm test --if-present

type-check:
	@echo "Running vue-tsc type check..."
	cd server/app && npx vue-tsc --noEmit

lint: lint-go lint-frontend

lint-go:
	go vet ./...
	@which golangci-lint > /dev/null && golangci-lint run || echo "  golangci-lint not found, skipping (install via https://golangci-lint.run)"

lint-frontend:
	cd server/app && npm run lint --if-present

fmt: fmt-go fmt-frontend

fmt-go:
	go fmt ./...

fmt-frontend:
	cd server/app && npx prettier --write "src/**/*.{ts,vue}" 2>/dev/null || true

# ─── Docker ──────────────────────────────────────────────────────────────────
docker:
	docker build -t openbooks .

docker-run:
	docker run -p 8080:80 -v $(PWD)/books:/books openbooks

# ─── Clean ───────────────────────────────────────────────────────────────────
clean:
	rm -f cmd/openbooks/openbooks
	rm -f openbooks
	rm -rf server/app/dist
	rm -rf server/app/node_modules/.cache
