# Decorator pattern

**What it is:** Wrap an object that implements an interface with another object of the same interface to add behavior.

**In Hoptrace:**

```go
inner := executor.NewHTTPExecutor()
decorated := executor.NewLoggingExecutor(inner, platform.Logger())
analyzer := probe.NewAnalyzer(probe.WithExecutor(decorated))
```

`LoggingExecutor` implements `probe.RequestExecutor` and forwards to the inner executor while emitting structured logs.

**Why it matters:** Cross-cutting concerns (logging, metrics, retries later) do not pollute the core httptrace executor. Stack more decorators as needed.
