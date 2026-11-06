# ShipCheck

> Don't trust a successful deployment. Verify the release.

ShipCheck is a command line tool that verifies a production release right after it is deployed. It answers one question: is this release actually healthy enough to trust?

A deploy command can succeed while the application is broken: the HTTP endpoint returns 200 but the database is down, Redis is unreachable, queue workers have stopped, containers are crash-looping, or a disk is full. ShipCheck checks those things with configurable verification checks and exits with a code your CI/CD pipeline can act on.

ShipCheck is not an uptime monitor and it is not a replacement for Prometheus, Grafana, or Kubernetes probes. Monitoring tells you something broke later. Probes tell Kubernetes whether to restart a pod. ShipCheck tells you, right after a deploy, whether the release you just shipped is genuinely operational, including the things probes never check, like whether workers are actually processing jobs.

## What it checks (v0.1.0)

| Category | What it verifies |
| --- | --- |
| HTTP | Endpoint status, response time threshold, body contains / not-contains, TLS certificate expiry |
| Database | PostgreSQL / MySQL connectivity plus a read-only query (`SELECT 1` by default) |
| Redis | PING plus a namespaced temporary read/write round-trip (never touches application keys) |
| Docker | Daemon reachable, required containers running, health status, restart count |
| Workers | Worker process present, heartbeat file fresh, Redis queue backlog under threshold |
| System | Disk usage, memory availability, CPU load, required processes |

A running process alone does not prove jobs are being processed. That is why workers get three checks, not one. See [docs/CHECKS.md](docs/CHECKS.md).

## Prerequisites

- Go 1.23 or newer, to build from source. No other runtime is needed: the build produces a single static binary.
- A `shipcheck.yml` config file in the project you want to verify (create one with `shipcheck init`).
- Network access from the machine running ShipCheck to the endpoints, databases, Redis, and Docker socket you configure.
- Linux for the full system and worker process checks. Those checks read `/proc`; on other platforms they return a clear error instead of guessing. See [Known limitations](#known-limitations).

## Installation

No release has been published yet, so there are no prebuilt binaries to download. Install from source until the first release is cut (see [Releases](#releases)).

```bash
# Clone and build (Go 1.23+)
git clone https://github.com/mrpythonist/ship-check.git
cd ship-check
make build
sudo cp shipcheck /usr/local/bin/

# Or install straight from the module (Go 1.23+)
go install github.com/mrpythonist/ship-check/cmd/shipcheck@latest

# Or run without installing
go run ./cmd/shipcheck version
```

`make build` sets the version string with `-ldflags`. A plain `go build` works too, but the version then reports the compiled-in default. See [docs/development.md](docs/development.md).

## Quick start

```bash
shipcheck init          # creates shipcheck.yml in the current directory
# edit the endpoints and credentials in shipcheck.yml, then:
shipcheck doctor        # validate the config and list the checks that would run
shipcheck verify        # run the checks and report the verdict
```

Example output from the automated smoke test (`test/smoke.sh`, run against a local test server during v0.1.0 verification):

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

When the application is down, the failure is specific and actionable:

```
[FAIL] Local Health                       518µs
       request to http://127.0.0.1:18080/ failed: ... connection refused
       Recommendation: Check the application is running and reachable from this host.
...
Release Status: FAILED
Exit Code: 1
```

## Usage

### Commands

```
shipcheck init [--config shipcheck.yml] [--force]
shipcheck verify [--config shipcheck.yml] [--json] [--verbose] [--fail-on-warn]
shipcheck doctor [--config shipcheck.yml]
shipcheck list-checks
shipcheck version
shipcheck help
```

Run `shipcheck` with no arguments to print usage. `shipcheck --help`, `shipcheck -h`, and `shipcheck help` all print the same usage text. `shipcheck --version` and `shipcheck -v` print the version.

### `shipcheck init`

Writes a starter `shipcheck.yml` in the current directory.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--config` | `shipcheck.yml` | Path of the config file to write |
| `--force` | off | Overwrite the file if it already exists |

Without `--force`, init refuses to overwrite an existing config and exits with code 2.

### `shipcheck verify`

Runs every enabled check and prints the report. This is the command CI pipelines run.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--config` | auto-detected | Path to the config file. If omitted, ShipCheck looks for `shipcheck.yml`, then `shipcheck.yaml`, then `.shipcheck.yml` in the current directory |
| `--json` | off | Print a machine-readable JSON report instead of terminal output |
| `--verbose` | off | Show a message for every check, including passing ones |
| `--fail-on-warn` | off | Exit 1 when any check warns, instead of exiting 0 |

Config file settings also apply: `reporting.format: json` turns on JSON output and `reporting.verbose: true` turns on verbose output, so flags and config agree and either one is enough.

### `shipcheck doctor`

Validates the config and lists the checks that would run, without executing any of them. It also reports read-only preflight facts: whether the Docker socket exists (when Docker checks are enabled) and whether the worker heartbeat file exists (when one is configured).

| Flag | Default | Meaning |
| --- | --- | --- |
| `--config` | auto-detected | Path to the config file, same detection as `verify` |

Doctor exits 0 when the config is valid, 2 when it is not.

### `shipcheck list-checks`

Lists the six built-in check categories with a one-line description of each. Takes no flags.

### `shipcheck version`

Prints `shipcheck v0.1.0` (or the version set at build time). Takes no flags.

Full CLI reference, with every command and flag derived from the source: [docs/cli-reference.md](docs/cli-reference.md).

### Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Release verified (warnings only, unless `--fail-on-warn`) |
| 1 | Release FAILED: a critical check failed, or a check errored |
| 2 | Configuration or usage error |
| 3 | Internal execution error (no checks could run) |

Warnings alone do not fail a release. System resource checks (disk, memory, load) are `warning` severity by default. Set `severity: critical` on a check, or pass `--fail-on-warn`, to make them fail the release.

### Machine-readable output

```bash
shipcheck verify --json
```

Produces a JSON report with project, environment, timestamp, verdict, exit code, per-check results, and totals. `duration_ms` is whole milliseconds. CI pipelines should use this, or simply rely on the exit code. The JSON shape is defined by `engine.Report` and `checks.Result` in the source.

## Configuration

`shipcheck.yml` (full annotated examples: [examples/shipcheck.yml](examples/shipcheck.yml), [examples/laravel.yml](examples/laravel.yml), [examples/node-api.yml](examples/node-api.yml); field reference: [docs/CONFIGURATION.md](docs/CONFIGURATION.md)):

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

Validation is strict and names the offending field, for example `checks.http.endpoints[0].url: must be a valid http(s) URL`. Secrets come from `${VAR}` environment expansion; never commit passwords or tokens in `shipcheck.yml`. If a referenced variable is unset, it expands to an empty string and validation fails with a named error instead of silently using a wrong value.

## CI/CD integration

ShipCheck needs no hosted service. A pipeline step runs the binary and uses its exit code: 0 means the release is verified, 1 means it failed. That is the whole integration.

```yaml
# .github/workflows/deploy.yml
- name: Verify Production Release
  run: shipcheck verify --json
```

Working examples for GitHub Actions, GitLab CI, and generic shell pipelines are in [examples/](examples/) and [docs/CI-CD.md](docs/CI-CD.md). Note: the examples download a `v0.1.0` release asset. That asset does not exist yet, because no release has been published. Until the first release is cut, install the binary in CI with `go install github.com/mrpythonist/ship-check/cmd/shipcheck@latest` or build it from a checkout.

A typical deploy-then-verify shell pipeline:

```bash
./deploy.sh || exit 1
shipcheck verify --json || {
  echo "Release verification FAILED, rolling back"
  ./rollback.sh
  exit 1
}
```

## Architecture

```
cmd/shipcheck        entry point
internal/cli         commands (init / verify / doctor / list-checks / version)
internal/config      YAML loading, env expansion, strict validation
internal/engine      execution, timeouts, parallel/sequential, verdict and exit code
internal/registry    check plugin registry (factories per category)
internal/checks/     http, database, redis, docker, workers, system
internal/reporting   terminal and JSON reports
internal/security    secret redaction, namespaced probe keys
pkg/checks           the public Check interface for third-party checks
```

Why Go? See [docs/DECISIONS.md](docs/DECISIONS.md): a single static binary, fast startup, small footprint, good CLI support, and a standard library that covers HTTP, TLS, and networking without a heavy dependency tree. The core has only three runtime dependencies: YAML parsing and the two database drivers.

Full architecture notes: [docs/architecture.md](docs/architecture.md) and [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Project structure

```
.
├── cmd/shipcheck          CLI entry point (main.go)
├── internal/
│   ├── checks/            http, database, redis, docker, workers, system
│   ├── cli/               command parsing, flags, usage text
│   ├── config/            shipcheck.yml loading, validation, defaults
│   ├── engine/            check execution, timeouts, verdict, exit codes
│   ├── registry/          category to factory mapping
│   ├── reporting/         terminal and JSON renderers
│   └── security/          redaction, probe keys, safe names
├── pkg/checks/            public Check interface for third-party checks
├── docs/                  architecture, checks, config, CLI, CI/CD, security
├── examples/              shipcheck.yml, Laravel, Node API, CI examples
├── test/                  smoke test and manual integration scenarios
├── Makefile               build, test, vet, fmt, install, clean
└── shipcheck.yml          (generated by shipcheck init, not committed)
```

## Testing

```bash
make test        # go vet + go test ./...
go test ./...    # unit tests for every internal package
go vet ./...     # static checks
gofmt -l cmd internal pkg   # formatting, must print nothing
./test/smoke.sh  # build, verify PASS against a local HTTP server,
                 # then verify FAIL (exit 1) after the server stops,
                 # and check --json output parses
```

Tests verify real behaviour against real local targets: HTTP against `httptest`, Redis against a fake RESP server, Docker against a fake Unix-socket API. A test that cannot fail is not a test. Manual integration scenarios (healthy app, database down, Redis down, worker stopped, unhealthy container, slow endpoint, invalid config, partial failure) are listed in [test/README.md](test/README.md).

## Deployment

ShipCheck is a CLI, not a service. There is nothing to deploy or host. You run the binary on the machine that deploys, or in the CI job that runs after the deploy, pointed at the release you just shipped. It reads `shipcheck.yml`, runs the checks, prints the report, and exits with the verdict code.

### Releases

No release has been published yet. The release workflow (`.github/workflows/release.yml`) is prepared in this repository and builds binaries for Linux, macOS, and Windows (amd64/arm64) when a `v*` tag is pushed. To cut the first release, after the workflow file is in place on GitHub:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The workflow attaches the binaries to the GitHub release. Until then, install from source as described above.

## Security

ShipCheck is designed to run against production, so it must never cause an outage:

- No credential leakage: DSNs and URLs are redacted before logging or reporting.
- Read-only database verification (`SELECT 1` by default; destructive queries are rejected by config validation).
- Redis probe uses a uniquely namespaced temporary key (`shipcheck:probe:<random>`) with a 10 second TTL, and deletes it after use.
- No arbitrary shell execution from YAML: process checks scan `/proc` directly; nothing in config is executed.
- Strict per-check and global timeouts; HTTP bodies are capped at 1 MiB so an error page cannot exhaust memory or leak into reports.
- Docker checks are read-only over the Engine API; nothing is started, stopped, or restarted.

Full details, required permissions, and how to report a vulnerability: [SECURITY.md](SECURITY.md) and [docs/SECURITY.md](docs/SECURITY.md).

## Extending ShipCheck

Implement the `checks.Check` interface from `pkg/checks`, register a factory in the registry, and the engine, reporting, and exit codes pick it up with no core changes:

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

## How ShipCheck differs

| Tool | What it does | What ShipCheck adds |
| --- | --- | --- |
| Uptime Kuma / Gatus / Healthchecks.io | Continuous uptime and endpoint monitoring | Point-in-time release verdict after a deploy, with a CI exit code |
| Prometheus / Grafana | Metrics and alerting over time | No infrastructure to run; a single binary that answers "is this release healthy?" |
| Kubernetes liveness/readiness probes | Per-pod restart decisions | Framework-aware checks (workers, queues, heartbeats) outside Kubernetes, on VPS, Docker Compose, or EC2 |
| `curl` in a deploy script | One endpoint, one status | Six categories, structured reports, timeouts, severity, and an extensible registry |

## Known limitations

- System checks (memory, load, process scan) are implemented for Linux (`/proc`). On other platforms they return a clear error instead of guessing.
- Database and Redis checks have been tested against unreachable targets and local fakes, not against live PostgreSQL, MySQL, or Redis servers in this environment. The Docker check has been tested against a fake Unix-socket API, not against a live Docker daemon. See [docs/CHECKS.md](docs/CHECKS.md) for the verification status of each check.
- TLS expiry is implemented with a real TLS handshake but has not been exercised against a live expiring certificate in this environment.
- No Kubernetes support in v0.1.0, by design.
- No prebuilt release binaries yet. See [Releases](#releases).

## Roadmap

- Phase 2: Laravel / Node.js-specific checks, scheduled-job verification, external API checks, custom workflow verification, release profiles.
- Phase 3: Background agent, continuous verification, webhooks, multi-app management.
- Phase 4: Plugin ecosystem, historical reports, hosted dashboard (optional; the CLI always works standalone).
- Phase 5: Anomaly detection and release regression analysis. AI stays optional; the core engine never requires it.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Run `make test` before opening a PR. Report bugs with the bug report template and request features with the feature request template. See [SUPPORT.md](SUPPORT.md) if you need help using ShipCheck.

## License

MIT. See [LICENSE](LICENSE).
