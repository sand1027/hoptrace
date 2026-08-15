# Security

## Reporting

Please report security issues via GitHub Issues or privately to the repository owner.
Do not open public issues for actively exploitable remote vulnerabilities.

## Local defaults

- Probe history is stored in `~/.hoptrace/history.db` on the machine that runs hoptrace.
- The API defaults to **soft auth** (no API key required). Use `-require-auth` when exposing the API beyond localhost.
- The API includes an SSRF guard that blocks probes to private/loopback addresses unless `-allow-private` is set.
- Share links (`/v1/share/{token}`) are intentionally public read-only views of a stored probe report.

## Secrets

Never commit API keys. Prefer `HOPTRACE_API_KEY` in the environment for CLI/API clients.
