# Security

## Reporting

Please report security issues via GitHub Issues or privately to the repository owner.
Do not open public issues for actively exploitable remote vulnerabilities.

## Tenancy model (honest)

Hoptrace is **single-tenant per process/database**. One SQLite file (`~/.hoptrace/history.db` by default) holds all history, saved probes, schedules, keys, and share links for that machine.

`workspace_id` exists on API keys, schedules, and failure records as soft tagging for future multi-tenant work. **It is not isolation**: probe history and schedule lists are not hard-scoped across workspaces today. Do not treat hoptrace as a multi-tenant SaaS control plane.

## Local defaults

- Probe history is stored in `~/.hoptrace/history.db` on the machine that runs hoptrace.
- The API defaults to **soft auth** (no API key required). Use `-require-auth` when exposing the API beyond localhost.
- Share links (`/v1/share/{token}`) are intentionally public read-only views of a stored probe report.

## SSRF

The API SSRF guard (fail-closed):

- Blocks `localhost`, loopback, RFC1918 private, link-local, CGNAT (`100.64/10`), and cloud metadata hostnames.
- Resolves DNS and rejects any A/AAAA that lands on a blocked address; DNS lookup failures are denied.
- Re-validates **every redirect hop** during a probe (not only the initial URL).
- Also validates schedule/probe webhook and Slack webhook URLs before outbound notify.
- Use `-allow-private` only for trusted local debugging.

Response bodies are capped (default **10 MiB**, CLI `--max-body`) to limit memory abuse.

## Secrets

Never commit API keys. Prefer `HOPTRACE_API_KEY` in the environment for CLI/API clients.
