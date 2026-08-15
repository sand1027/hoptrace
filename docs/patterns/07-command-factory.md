# Command & Factory (v3)

## Command

CLI actions are discrete commands (Cobra):

- `hoptrace <url>` — probe now
- `hoptrace save <name> <url>` — persist a template
- `hoptrace run <name>` — execute a template
- `hoptrace saved list|show|delete`
- `hoptrace history` / `history show` / `history compare`

Each command encapsulates one user intention — parse inputs, call the domain, present output.

## Factory

- `setup.DefaultAnalyzer(opts...)` builds the production pipeline (executor + logging decorator + optional observers)
- `export.Factory(mode, ...)` builds the right output Strategy (`waterfall`, `compact`, `metrics-only`, `json`)
- `probe.NewRequestBuilder()` assembles validated requests for both CLI and API
