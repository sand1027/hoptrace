# Strategy pattern

**What it is:** Define a family of interchangeable algorithms behind a shared interface. Clients depend on the interface, not a concrete implementation.

**In Hoptrace:**

```go
type DNSResolver interface {
    Resolve(ctx context.Context, host, port string) (ip, family string, dnsMS float64, err error)
}

type RequestExecutor interface {
    Execute(ctx context.Context, req ProbeRequest, targetURL string) (StepResult, error)
}

type Exporter interface {
    Export(report ProbeReport) error
}
```

- `internal/dns.SystemResolver` — production DNS strategy
- `internal/executor.HTTPExecutor` — httptrace-backed HTTP strategy
- `internal/export.{Waterfall,Compact,Metrics,JSON}Exporter` — output strategies

**Why it matters:** You can swap DNS, HTTP stacks, or output formats without touching `Analyzer`. That is the Open/Closed Principle in practice.

**Try it:** Implement a `HardcodedDNS` that always returns `1.2.3.4` and inject it with `probe.WithDNS(...)`.
