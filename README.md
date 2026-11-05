# ShipCheck

> **Don't trust a successful deployment. Verify the release.**

ShipCheck is a production release verification engine. After every deployment it answers one question: **"Is this release actually healthy enough to trust?"**

A deploy command can succeed while the application is broken: the HTTP endpoint returns 200 but the database is down, Redis is unreachable, queue workers have stopped, containers are crash-looping, or a disk is full. ShipCheck detects those problems with configurable, extensible verification checks, and exits with a code your CI/CD pipeline can act on.

ShipCheck is **not** another uptime monitor, and it is not a replacement for Prometheus, Grafana, or Kubernetes probes. Monitoring tells you something broke *later*. Probes tell Kubernetes whether to restart a pod. ShipCheck tells *you, right after a deploy*, whether the release you just shipped is genuinely operational — including the things probes never check, like whether workers are actually processing jobs.

## What it checks (v0.1.0)

| Category | What it verifies |
| --- | --- |
| **HTTP** | Endpoint status, response-time threshold, body contains / not-contains, TLS certificate expiry |
| **Database** | PostgreSQL / MySQL connectivity + a read-only query (`SELECT 1` by default) |
| **Redis** | PING + a namespaced temporary read/write round-trip (never touches app keys) |
| **Docker** | Daemon reachable, required containers running, health status, restart count |
| **Workers** | Worker process present, heartbeat file fresh, Redis queue backlog under threshold |
| **System** | Disk usage, memory availability, CPU load, required processes |

A running process alone does not prove jobs are being processed — that is why workers get three checks, not one. See [docs/CHECKS.md](docs/CHECKS.md).

## Quick start

### Install

```bash
# From source (Go 1.23+)
git clone https://github.com/mrpythonist/ship-check.git
cd ship-check
make build
sudo cp shipcheck /usr/local/bin/

# Or run without installing
go run ./cmd/shipcheck version
```

Prebuilt binaries for Linux, macOS and Windows are published on the [Releases page](https://github.com/mrpythonist/ship-check/releases) (see [docs/CI-CD.md](docs/CI-CD.md) for the release workflow).

### Verify your first release

```bash
shipcheck init          # creates shipcheck.yml
# edit the endpoints, then:
shipcheck verify
```

Real terminal output (against a local test server):

```
ShipCheck v0.1.0

Production Release Verification
Project: smoke-app
Environment: production

[PASS] Local Health                       2ms
[PASS] Disk Usage                         218µs
[PASS] Memory Availability                162µs

-------------------------------------------
Verification Summary

Total Checks: 3
Passed:       3
Failed:       0
Warnings:     0

Release Status: PASSED

-------------------------------------------
Exit Code: 0
```

And when the app is down, the failure is specific and actionable:

```
[FAIL] Local Health                       518µs
       request to http://127.0.0.1:18080/ failed: ... connection refused
       Recommendation: Check the application is running and reachable from this host.
...
Release Status: FAILED
Exit Code: 1
```

## Commands

```
shipcheck init [--config shipcheck.yml] [--force]
shipcheck verify [--config shipcheck.yml] [--json] [--verbose] [--fail-on-warn]
shipcheck doctor [--config shipcheck.yml]
shipcheck list-checks
shipcheck version
```

### Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Release verified (warnings only, unless `--fail-on-warn`) |
| 1 | Release **FAILED** — a critical check failed, or a check errored |
| 2 | Configuration / usage error |
| 3 | Internal execution error (no checks could run) |

Warnings alone do not fail a release: system resource checks (disk, memory, load) are `warning` severity by default. Set `severity: critical` on a check, or pass `--fail-on-warn`, to make them fail the release.

### Machine-readable output

```bash
shipcheck verify --json
```

Produces a JSON report with project, environment, timestamp, verdict, exit code, and per-check results (`duration_ms` is whole milliseconds). CI pipelines should use this, or simply rely on the exit code.

## Configuration

`shipcheck.yml` (see [examples/shipcheck.yml](examples/shipcheck.yml) and [docs/CONFIGURATION.md](docs/CONFIGURATION.md)):

```yaml
version: 1

project:
  name: my-saas
  environment: production

verification:
  timeout: 30s
  parallel: true
  fail_fast: false

checks:
  http:
    enabled: true
    endpoints:
      - name: API Health
        url: https://example.com/api/health
        expected_status: 200
        timeout: 5s
        max_response_time_ms: 2000
        body_contains: "ok"
        check_tls: true
        tls_min_days: 14

  database:
    enabled: true
    driver: postgres
    dsn: ${DATABASE_URL}        # secrets from the environment, never committed

  redis:
    enabled: true
    address: 127.0.0.1:6379

  docker:
    enabled: true
    containers:
      - name: my-saas-app
        max_restarts: 3

  workers:
    enabled: true
    processes:
      - name: Laravel Queue Worker
        pattern: "queue:work"
        min_count: 1
    heartbeat_file: /var/www/my-saas/storage/worker-heartbeat.txt
    heartbeat_max_age: 5m

  system:
    enabled: true
    disk_max_usage_percent: 90
    memory_max_usage_percent: 90

reporting:
  format: terminal
  verbose: false
```

Validation is strict and names the offending field (e.g. `checks.http.endpoints[0].url: must be a valid http(s) URL`). Secrets come from `${VAR}` environment expansion; never commit passwords or tokens in `shipcheck.yml`.

## CI/CD integration

```yaml
# .github/workflows/deploy.yml
- name: Verify Production Release
  run: shipcheck verify --json
```

The job fails when the release fails, because the exit code is 1. No hosted ShipCheck service is involved. Working examples for GitHub Actions, GitLab CI and generic shell pipelines are in [examples/](examples/) and [docs/CI-CD.md](docs/CI-CD.md).

## Architecture

```
cmd/shipcheck        entry point
internal/cli         commands (init / verify / doctor / list-checks / version)
internal/config      YAML loading, env expansion, strict validation
internal/engine      execution, timeouts, parallel/sequential, verdict + exit code
internal/registry    check plugin registry (factories per category)
internal/checks/     http, database, redis, docker, workers, system
internal/reporting   terminal + JSON reports
internal/security    secret redaction, namespaced probe keys
pkg/checks           the public Check interface for third-party checks
```

Why Go? See [docs/DECISIONS.md](docs/DECISIONS.md): single static binary, fast startup, small footprint, excellent CLI and cross-platform support, and a stdlib that covers HTTP, TLS and networking without a heavy dependency tree. The core has only three runtime dependencies: YAML parsing and the two database drivers.

Full architecture notes: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Extending ShipCheck

Implement the `checks.Check` interface from `pkg/checks`, register a factory in the registry, and the engine, reporting and exit codes pick it up with no core changes:

```go
type Check interface {
    ID() string
    Name() string
    Category() string
    Description() string
    Severity() Severity
    Timeout() time.Duration
    Run(ctx context.Context) Result
}
```

Checks must be production-safe: read-only, non-destructive, bounded by the context deadline, and must never log credentials. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

ShipCheck is designed to run against production, so it must never *cause* an outage:

- No credential leakage: DSNs and URLs are redacted before logging or reporting.
- Read-only database verification (`SELECT 1` by default; destructive queries are rejected by config validation).
- Redis probe uses a uniquely namespaced temporary key (`shipcheck:probe:<random>`) with a 10s TTL, and deletes it after use.
- No arbitrary shell execution from YAML: process checks scan `/proc` directly; nothing in config is executed.
- Strict per-check and global timeouts; HTTP bodies are capped at 1 MiB so an error page cannot exhaust memory or leak into reports.
- Docker checks are read-only over the Engine API; nothing is started, stopped or restarted.

Full details and required permissions: [SECURITY.md](SECURITY.md) and [docs/SECURITY.md](docs/SECURITY.md).

## How ShipCheck differs

| Tool | What it does | What ShipCheck adds |
| --- | --- | --- |
| Uptime Kuma / Gatus / Healthchecks.io | Continuous uptime / endpoint monitoring | Point-in-time *release* verdict after a deploy, with a CI exit code |
| Prometheus / Grafana | Metrics and alerting over time | No infrastructure to run; a single binary that answers "is *this* release healthy?" |
| Kubernetes liveness/readiness probes | Per-pod restart decisions | Framework-aware checks (workers, queues, heartbeats) outside Kubernetes, on VPS / Docker Compose / EC2 |
| `curl` in a deploy script | One endpoint, one status | Six categories, structured reports, timeouts, severity, and an extensible registry |

## Roadmap

- **Phase 2:** Laravel / Node.js-specific checks, scheduled-job verification, external API checks, custom workflow verification, release profiles.
- **Phase 3:** Background agent, continuous verification, webhooks, multi-app management.
- **Phase 4:** Plugin ecosystem, historical reports, hosted dashboard (optional; the CLI always works standalone).
- **Phase 5:** Anomaly detection and release regression analysis. AI stays optional; the core engine never requires it.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Run `make test` before opening a PR; tests must verify real behaviour (HTTP against `httptest`, Redis against a fake RESP server, Docker against a fake Unix-socket API) — no fake passing checks.

## License

MIT — see [LICENSE](LICENSE).
