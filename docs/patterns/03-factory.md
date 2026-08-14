# Factory pattern

**What it is:** Centralize object creation so callers ask for “what they need” instead of knowing construction details.

**In Hoptrace:**

```go
// Wired production analyzer (executor + logging decorator)
analyzer := setup.DefaultAnalyzer()

// Exporter by mode name
exporter, err := export.Factory("metrics-only", os.Stdout, "")
```

`probe.NewAnalyzer(opts...)` is a lighter factory using functional options (`WithExecutor`, `WithDNS`, `WithListener`).

**Why it matters:** Construction policy (which decorator wraps which strategy) lives in one file. Apps stay declarative.
