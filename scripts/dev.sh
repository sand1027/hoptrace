#!/usr/bin/env bash
# Local dashboard: API (SQLite) + Next.js web.
# No Postgres, Docker, or API keys required.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

API_PID=""
cleanup() {
  if [[ -n "${API_PID}" ]] && kill -0 "${API_PID}" 2>/dev/null; then
    kill "${API_PID}" 2>/dev/null || true
    wait "${API_PID}" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

echo "→ API  http://127.0.0.1:8080  (SQLite ~/.hoptrace/history.db)"
echo "→ Web  http://127.0.0.1:3000"
echo

go run ./apps/api -addr :8080 &
API_PID=$!

for _ in $(seq 1 40); do
  if curl -sf http://127.0.0.1:8080/v1/health >/dev/null 2>&1; then
    break
  fi
  if ! kill -0 "${API_PID}" 2>/dev/null; then
    echo "API exited before becoming healthy" >&2
    exit 1
  fi
  sleep 0.25
done

pnpm --filter @hoptrace/web dev
