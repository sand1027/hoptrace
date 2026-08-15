# Design patterns in Hoptrace

These short notes map GoF-style patterns to real packages in this repo.

| Doc | Pattern | Code |
|-----|---------|------|
| [01-strategy](01-strategy.md) | Strategy | DNS, Executor, Exporter interfaces |
| [02-facade](02-facade.md) | Facade | `probe.Analyzer` |
| [02-builder](02-builder.md) | Builder | `probe.RequestBuilder` |
| [03-factory](03-factory.md) | Factory | `setup.DefaultAnalyzer`, `export.Factory` |
| [04-decorator](04-decorator.md) | Decorator | `executor.LoggingExecutor` |
| [05-repository-adapter-null](05-repository-adapter-null.md) | Repository / Adapter / Null Object | `repository`, timing adapters |
| [06-repository-sqlite](06-repository-sqlite.md) | Repository + SQLite Adapter (v2) | `SQLiteRepository`, shared history DB |
| [07-command-factory](07-command-factory.md) | Command + Factory (v3) | Cobra commands, `DefaultAnalyzer`, exporters |
| [08-observer-live](08-observer-live.md) | Observer + live stream (v4) | `EventListener`, WebSocket, compare |
| [09-v5-v9](09-v5-v9.md) | Schedules → Scale (v5–v9) | Worker, auth, notify, plugins, Postgres, rate limits |

Also used lightly:

- **Template Method** — redirect loop in `Analyzer.Analyze`
- **Observer** — `PhaseListener` (live UI in v4)
- **Command** — Cobra subcommands / API handlers
- **Singleton** — `platform.Logger()` only
