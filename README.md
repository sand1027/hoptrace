# Hoptrace

HTTP latency profiler that breaks each request into **DNS → Connect → TLS → Wait → Transfer** phases.

Shared Go probe engine powers:
- **CLI** (`apps/cli`) — curl-like terminal tool
- **API** (`apps/api`) — JSON HTTP API
- **Web** (`apps/web`) — Next.js waterfall UI

## Quick start

```bash
# Install JS deps
pnpm install

# Run API (port 8080)
make api

# Run CLI probe (quote URLs that contain &)
make cli URL='https://httpbin.io/get'
make cli URL='https://example.com/path?a=1&b=2' FLAGS='--metrics-only'

# Or call go directly
go run ./apps/cli 'https://httpbin.io/get'
go run ./apps/cli --follow 'https://httpbin.io/redirect/2'

# Run web UI (port 3000, proxies /api → :8080)
make web
```

## Architecture

See [architecture/](architecture/) for Mermaid sources and PNG diagrams.
Design pattern explainers live in [docs/patterns/](docs/patterns/).

## Roadmap

v1 core probe → v2 history → v3 saved probes → v4 live stream → v5 schedules →
v6 auth → v7 alerting → v8 plugins → v9 scale/polish.

## License

Apache-2.0
