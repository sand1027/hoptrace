# Architecture diagrams

Mermaid sources (`.mmd`) and exported PNGs for the Hoptrace monorepo.

| Diagram | Source | PNG | Description |
|---------|--------|-----|-------------|
| System context | [01-system-context.mmd](01-system-context.mmd) | [01-system-context.png](01-system-context.png) | Clients, API, shared probe core, persistence |
| Containers | [02-container.mmd](02-container.mmd) | [02-container.png](02-container.png) | Monorepo apps and internal packages |
| Probe pipeline | [03-probe-pipeline.mmd](03-probe-pipeline.mmd) | [03-probe-pipeline.png](03-probe-pipeline.png) | Builder → Facade → Strategies → Decorators |
| Design patterns | [04-design-patterns.mmd](04-design-patterns.mmd) | [04-design-patterns.png](04-design-patterns.png) | Pattern map used across the codebase |
| Probe sequence | [05-sequence-probe.mmd](05-sequence-probe.mmd) | [05-sequence-probe.png](05-sequence-probe.png) | End-to-end request timing flow |

## Regenerate PNGs

From the repo root:

```bash
make architecture
# or
npx --yes @mermaid-js/mermaid-cli -i architecture/01-system-context.mmd -o architecture/01-system-context.png
```

Requires Node.js / npx. Chromium is pulled in by `@mermaid-js/mermaid-cli` on first run.
