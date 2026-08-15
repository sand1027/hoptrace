# Exporter plugin example (v8)

Hoptrace exporters are registered via `internal/plugin.RegisterExporter`.

```go
package myplugin

import (
  "github.com/sandeepv/hoptrace/internal/plugin"
  "github.com/sandeepv/hoptrace/internal/probe"
)

func init() {
  plugin.RegisterExporter("my-format", func() probe.Exporter {
    return MyExporter{}
  })
}
```

Then `export.Factory("my-format", ...)` resolves it.

Built-in demo: `jsonl` mode and registered name `jsonl-plugin`.
