# Observer & live streaming (v4)

## Observer

`probe.EventListener` receives lifecycle events:

- `step_start` / `step_done`
- `phase` (dns, connect, tls, wait, xfer, ttfb, total)
- `done` / `error`

`MultiListener` fans events to many observers. `ChanListener` bridges the analyzer to a WebSocket writer.

```go
listener := probe.NewChanListener(64)
analyzer := setup.DefaultAnalyzer(probe.WithEvents(listener))
report, err := analyzer.Analyze(ctx, req)
```

## WebSocket

`GET /v1/probes/stream` upgrades to WS. Client sends a probe JSON body; server streams events, then a final `complete` message with the report id.

Web UI checkbox **Live waterfall** connects to `ws://127.0.0.1:8080/v1/probes/stream` (override with `NEXT_PUBLIC_HOPTRACE_WS`).

## Compare

- CLI: `hoptrace history compare <id-a> <id-b>`
- Web: Compare panel picks two history runs and shows phase deltas
