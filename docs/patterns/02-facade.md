# Facade pattern

**What it is:** A single, simple entry point that hides a messier subsystem of collaborators.

**In Hoptrace:**

```go
analyzer := setup.DefaultAnalyzer()
report, err := analyzer.Analyze(ctx, req)
```

`probe.Analyzer` orchestrates:

1. Request execution (with timings)
2. Redirect following (template-method loop)
3. SLO evaluation
4. Phase listener notifications

CLI and API both call the same facade — they never wire DNS/TLS/export themselves for the happy path.

**Why it matters:** Two surfaces (CLI + API) stay thin. Domain complexity lives in one place.

**See also:** [Factory](03-factory.md) builds the facade; [Builder](02-builder.md) builds its input.
