.PHONY: api cli web test architecture tidy smoke install dev release

API_ADDR ?= :8080
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

api:
	go run ./apps/api -addr $(API_ADDR)

# Local dashboard: SQLite API + web UI (clone → run).
# Usage: pnpm install && make install && make dev
dev:
	bash scripts/dev.sh

# Quote URL args so & in query strings is not interpreted by the shell.
# Example:
#   make cli URL='https://example.com/path?a=1&b=2'
#   make cli URL='https://example.com' FLAGS='--follow --metrics-only'
cli:
	go run ./apps/cli $(FLAGS) "$(URL)"

install:
	go build -o bin/hoptrace ./apps/cli
	go build -o bin/hoptrace-api ./apps/api
	mkdir -p "$(HOME)/.local/bin"
	ln -sf "$(CURDIR)/bin/hoptrace" "$(HOME)/.local/bin/hoptrace"
	@echo "Installed: $(HOME)/.local/bin/hoptrace"
	@echo "Try: hoptrace version"

# Cross-compile CLI binaries into dist/ (also published on git tag v*).
release:
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o dist/hoptrace_darwin_arm64 ./apps/cli
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o dist/hoptrace_darwin_amd64 ./apps/cli
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/hoptrace_linux_amd64 ./apps/cli
	GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/hoptrace_linux_arm64 ./apps/cli
	cp openapi.yaml dist/
	@echo "Built $(VERSION) → dist/"

web:
	pnpm --filter @hoptrace/web dev

test:
	go test ./...

tidy:
	go mod tidy

architecture:
	node scripts/render-architecture.mjs

smoke:
	go run ./apps/cli --metrics-only https://httpbin.io/get
	go run ./apps/cli history
	go run ./apps/cli --follow --metrics-only https://httpbin.io/redirect/2
	go run ./apps/cli --slo total=5000,ttfb=4000 --metrics-only https://httpbin.io/get
