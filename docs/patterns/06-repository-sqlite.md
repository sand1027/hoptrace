# Repository & Adapter (v2)

## Repository

`ProbeRepository` is the persistence contract:

```go
type ProbeRepository interface {
    Save(ctx context.Context, report probe.ProbeReport) (ProbeRecord, error)
    Get(ctx context.Context, id string) (ProbeRecord, error)
    List(ctx context.Context, limit int) ([]ProbeRecord, error)
}
```

Call sites (API handlers, CLI history) depend on the interface — never on SQL.

## Adapter

| Adapter | Package | Role |
|---------|---------|------|
| `MemoryRepository` | `internal/repository` | Tests / ephemeral |
| `SQLiteRepository` | `internal/repository` | Default disk store (`~/.hoptrace/history.db`) |
| (v9) Postgres | same interface | Scale-out |

`SQLiteRepository` adapts SQL rows ↔ `ProbeRecord` / `ProbeReport` JSON.

## Where history lives

- Env override: `HOPTRACE_DB=/path/to/file.db`
- Default: `~/.hoptrace/history.db`
- Shared by **CLI** and **API**, so web UI and `hoptrace history` see the same runs.

```bash
hoptrace https://example.com
hoptrace history
hoptrace history show <id>
```
