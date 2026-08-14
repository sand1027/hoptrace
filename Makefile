.PHONY: api cli web test architecture tidy smoke

API_ADDR ?= :8080

api:
	go run ./apps/api -addr $(API_ADDR)

# Quote URL args so & in query strings is not interpreted by the shell.
# Example:
#   make cli URL='https://example.com/path?a=1&b=2'
#   make cli URL='https://example.com' FLAGS='--follow --metrics-only'
cli:
	go run ./apps/cli $(FLAGS) "$(URL)"

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
	go run ./apps/cli --follow --metrics-only https://httpbin.io/redirect/2
	go run ./apps/cli --slo total=5000,ttfb=4000 --metrics-only https://httpbin.io/get
