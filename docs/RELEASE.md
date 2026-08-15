# Release notes (v9)

## Install
```bash
make install
hoptrace version   # hoptrace v9.0.0
```

## Run (local — SQLite)

```bash
make install
make dev    # API :8080 + dashboard :3000
# or: make api && make web
```

No `DATABASE_URL`, Docker, or API keys required. History lives in `~/.hoptrace/history.db`.

## New since v4
- **Schedules:** `hoptrace schedule add name url --every 60` (API must be up)
- **Baselines:** `hoptrace baseline 'https://example.com'`
- **API keys:** optional — only with `-require-auth` / hosted API
- **Alerts:** pass `webhook_url` / `slack_webhook` on probe or schedule
- **Share:** dashboard **Copy share link** → `/share/{token}` (API `POST /v1/probes/{id}/share`)
- **Postgres:** stub adapter only — **not wired** into `apps/api` (local = SQLite)
- **OpenAPI:** [openapi.yaml](../openapi.yaml)

## SBOM / release

```bash
make release          # dist/hoptrace_{darwin,linux}_{amd64,arm64}
# or push a tag:
git tag v9.0.0 && git push origin v9.0.0   # GitHub Actions publishes assets
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | OK |
| 4 | SLO failed |
| 64 | Usage |
| 70 | Software error |
| 75 | Probe / network failure |
