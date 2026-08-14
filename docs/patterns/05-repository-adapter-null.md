# Repository, Adapter, Null Object

## Repository

`repository.ProbeRepository` abstracts persistence:

```go
Save(ctx, report) (ProbeRecord, error)
Get(ctx, id) (ProbeRecord, error)
List(ctx, limit) ([]ProbeRecord, error)
```

v1 ships `MemoryRepository`. v2 will add SQLite without changing API handlers.

## Adapter

- `executor.adaptTimings` adapts `httptrace` timestamps into the domain `Timing` model.
- `tlsinfo.FromConnectionState` adapts `tls.ConnectionState` into `TLSInfo`.
- Future: SQLite/Postgres adapters implement `ProbeRepository`.

## Null Object

```go
type NoopExporter struct{}
func (NoopExporter) Export(ProbeReport) error { return nil }

type NoopListener struct{}
func (NoopListener) OnPhase(string, int, float64) {}
```

Avoids nil checks when export or observation is disabled.
