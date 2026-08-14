# Builder pattern

**What it is:** Construct a complex object step-by-step with a fluent API, validating only at the end.

**In Hoptrace:**

```go
req, err := probe.NewRequestBuilder().
    URL("https://httpbin.io/post").
    Method("POST").
    Header("Accept", "application/json").
    Body([]byte(`{"ok":true}`)).
    FollowRedirect(true).
    SLO(map[string]float64{"total": 2000}).
    Build()
```

`RequestBuilder` owns defaults (GET, 30s timeout, max 10 redirects) and validation (scheme, mutually exclusive SSL flags, SLO keys).

**Why it matters:** Call sites stay readable. Invalid combinations fail early with clear errors instead of half-built structs.
