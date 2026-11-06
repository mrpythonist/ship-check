# Development

## Requirements

- Go 1.23 or newer. Check with `go version`.
- `python3`, only for the smoke test server in `test/smoke.sh`.
- No Docker, no database, and no other service is needed for the unit
  tests. They use local fakes.

## Setup

```bash
git clone https://github.com/mrpythonist/ship-check.git
cd ship-check
go test ./...   # should pass with no services running
```

## Makefile targets

| Target | What it runs |
| --- | --- |
| `make build` | Builds `./shipcheck` with `-trimpath` and `-ldflags` that set the version string |
| `make test` | `go vet ./...`, then `go test ./...` |
| `make vet` | `go vet ./...` |
| `make fmt` | `gofmt -l cmd internal pkg` (prints files that need formatting; clean output means clean code) |
| `make install` | Builds, then copies the binary to `/usr/local/bin/shipcheck` |
| `make clean` | Removes `./shipcheck` |

The version string comes from the `VERSION` make variable (default
`v0.1.0`). Override it for a release build:

```bash
make build VERSION=v0.1.0
```

A plain `go build ./cmd/shipcheck` also works, but the version then
reports the compiled-in default instead of the release tag.

## Tests

```bash
go test ./...              # unit tests for every internal package
go test ./internal/engine/ # one package
go test -run TestConfig ./internal/config/  # one test by name
./test/smoke.sh            # end-to-end: build, PASS, FAIL, JSON parse
```

Tests live next to the code they test (`*_test.go`). They verify real
behaviour against real local targets:

- HTTP checks run against `httptest` servers.
- Redis checks run against a fake RESP server in
  `internal/checks/redis`.
- Docker checks run against a fake Unix-socket API in
  `internal/checks/docker`.
- Database checks run against unreachable targets and assert that
  credentials never appear in the failure message.

A test that cannot fail is not a test. If you add a check, add a test
that fails when the target is actually broken.

Manual integration scenarios (healthy app, database down, Redis down,
worker stopped, unhealthy container, slow endpoint, invalid config,
partial failure) are listed in [test/README.md](../test/README.md).
Run them against a real stack before a release. In particular, the
database, Redis, and Docker checks have not yet been run against live
servers in this environment; see [CHECKS.md](CHECKS.md).

## Project layout

```
cmd/shipcheck          entry point only (main.go calls cli.Run)
internal/cli           commands, flags, usage text, starter config
internal/config        YAML loading, ${VAR} expansion, validation, defaults
internal/engine        execution, timeouts, verdict, exit codes
internal/registry      category name to factory mapping
internal/checks/       one package per category
internal/reporting     terminal and JSON renderers
internal/security      secret redaction, probe keys, safe names
pkg/checks             public Check interface for third-party checks
docs/                  user and contributor documentation
examples/              config and CI examples
test/                  smoke test and manual scenarios
```

## Adding a check

1. Implement `checks.Check` from `pkg/checks` in a new package under
   `internal/checks/<category>/`.
2. Add a `Factory(cfg any) ([]checks.Check, error)` and register it in
   `internal/cli.DefaultRegistry` (or your own registry in tests).
3. Add config fields in `internal/config`, with validation and
   defaults. Validation errors must name the offending field.
4. Add tests that fail when the target is actually broken.
5. Document the check in `docs/CHECKS.md` and the README table, and
   add a CHANGELOG entry under `[Unreleased]`.

Rules for every check:

- Read-only and non-destructive. The only write allowed is the Redis
  probe key, namespaced and deleted after use.
- Bounded by the context deadline. Never ignore `ctx.Done()`.
- Never log or return a credential. Use `internal/security.Redact`.
- Never execute anything from config. Process checks scan `/proc`
  directly.
- Return a result, do not panic. The engine recovers panics, but a
  recovered check is an `error` result that fails the release.

## Config defaults

Defaults live in `internal/config`, not in the checks. `Load` unmarshals
the YAML over `Default()`, so omitted fields keep sensible values. The
defaults that matter most:

- Global verification timeout: 30s. Per-check timeout: 5s.
- HTTP `expected_status`: 200. TLS warning threshold: 14 days.
- Database query: `SELECT 1`. Docker socket: `/var/run/docker.sock`.
- System disk and memory thresholds: 90 percent. Load check: disabled
  unless `max_load_1m` is set.
- System checks are `warning` severity by default. Every other
  category is `critical` unless `severity: warning` is set.

## Debugging a failed run

- `shipcheck verify --verbose` shows the message for passing checks
  too, so you can see what each check actually observed.
- `shipcheck verify --json` gives the full structured result,
  including per-check `details` and the `error` field.
- `shipcheck doctor` validates the config and lists the checks that
  would run, without executing them.
- If a single category misbehaves, disable the others in a scratch
  copy of the config and run `verify --config` against that copy.
