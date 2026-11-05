# Architecture

## Components

```
CLI (internal/cli)
  └─ Engine (internal/engine)
       ├─ Registry (internal/registry) ── factories per category
       │    ├─ http, database, redis, docker, workers, system
       │    └─ third-party checks (pkg/checks.Check)
       └─ Reporting (internal/reporting) ── terminal + JSON
Config (internal/config) ── YAML, env expansion, validation
Security (internal/security) ── redaction, probe keys
```

- **CLI** parses commands and flags, loads config, and maps the engine
  result to a process exit code. It owns no check logic.
- **Config** is the only place YAML is understood. It validates strictly
  and names the offending field. Defaults live here, not in checks.
- **Registry** maps a category name to a factory. The engine discovers
  checks only through the registry, so a new check category plugs in
  without touching the engine.
- **Engine** builds checks for enabled categories, runs them with a
  per-check timeout inside a global timeout, in parallel (default) or
  sequentially, recovers panics into `error` results, and computes the
  verdict: any critical failure or error fails the release; warnings
  alone do not, unless `--fail-on-warn`.
- **Checks** each implement `pkg/checks.Check`. They are small, single
  purpose, and production-safe (read-only, non-destructive, no shell).
- **Reporting** renders the same `engine.Report` as terminal text or
  JSON. Adding HTML/Markdown later means adding a renderer, not
  changing the engine.

## Execution lifecycle

1. `shipcheck verify` loads `shipcheck.yml`.
2. The engine builds checks for enabled categories.
3. Checks run with timeouts (parallel by default).
4. Results are collected, sorted deterministically by category/name.
5. Verdict + exit code are calculated.
6. The report is rendered; the CLI exits with the report's exit code.

## Decisions

See [DECISIONS.md](DECISIONS.md) for the language choice (Go) and other
architectural decisions, each with context, decision, and consequences.
