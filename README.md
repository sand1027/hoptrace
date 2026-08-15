# Hoptrace

HTTP latency profiler — break every request into **DNS → Connect → TLS → Wait → Transfer**.

| Surface | Use |
|---------|-----|
| **CLI** | Fast local probes (`hoptrace <url>`) |
| **Dashboard** | Compose/send requests, waterfall, history, library |

**Local store: SQLite** (`~/.hoptrace/history.db`). No Postgres, Docker, or API keys required to try it.

## Clone and run (local dashboard)

```bash
git clone <repo> && cd hoptrace
pnpm install
make install          # CLI → ~/.local/bin/hoptrace
make dev              # API :8080 + dashboard :3000
```

Open [http://127.0.0.1:3000](http://127.0.0.1:3000) — send a request, watch the waterfall, browse history.

Or run pieces separately:

```bash
make api              # :8080 (SQLite, no auth)
make web              # :3000
```

## Install CLI only

```bash
go install github.com/sandeepv/hoptrace/apps/cli@latest   # after push
# or from a clone:
make install
```

Tagged releases (`v*`) publish cross-compiled binaries via GitHub Actions. Locally: `make release` → `dist/`.

## CLI

```bash
hoptrace 'https://httpbin.io/get'
hoptrace save demo https://httpbin.io/get
hoptrace run demo
hoptrace history
hoptrace dashboard          # opens http://127.0.0.1:3000
hoptrace --slo total=500 'https://httpbin.io/get'
```

Start the dashboard stack with `make dev`, then `hoptrace dashboard` opens it in your browser.
TTY waterfall uses color phase bars (auto on interactive terminals).  
Force on: `HOPTRACE_COLOR=1` · force off: `NO_COLOR=1`. Add `-v` for probe debug logs.

### Exit codes (scripting)

| Code | Meaning |
|------|---------|
| `0` | OK |
| `4` | SLO failed |
| `64` | Usage / bad flags |
| `70` | Software error |
| `75` | Probe / network failure |

CLI and dashboard share the same probe engine and SQLite history.

## Storage

| Mode | Store |
|------|--------|
| **Local (default)** | SQLite — zero setup |
| Hosted / scale later | Postgres adapter exists (`DATABASE_URL`) but is **optional** and not needed for clone-and-run |

## Develop

```bash
make test             # go test ./...
pnpm --filter @hoptrace/web typecheck
```

CI runs both on every PR (`.github/workflows/ci.yml`).

## Docs

- [architecture/](architecture/) — diagrams
- [docs/patterns/](docs/patterns/) — design patterns
- [openapi.yaml](openapi.yaml) — API contract
- [docs/RELEASE.md](docs/RELEASE.md) — release notes
- [ROADMAP.md](ROADMAP.md)

## License

Apache-2.0
