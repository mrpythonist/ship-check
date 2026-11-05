# Changelog

All notable changes to ShipCheck are documented here. The format follows
Keep a Changelog, and the project adheres to Semantic Versioning.

## [0.1.0] - 2026-11-05

### Added

- CLI: `init`, `verify`, `doctor`, `list-checks`, `version` commands.
- Verification engine with per-check and global timeouts, parallel and
  sequential execution, `fail_fast`, panic recovery, and exit codes
  (0 pass, 1 failed, 2 config error, 3 internal error).
- Check registry (`pkg/checks.Check` interface) for third-party checks.
- HTTP checks: status, response-time threshold, body contains /
  not-contains, TLS certificate expiry.
- Database checks: PostgreSQL and MySQL connectivity plus a read-only
  query, with credential redaction.
- Redis checks: PING plus a namespaced temporary read/write round-trip.
- Docker checks: daemon, required containers, health status, restart count.
- Worker checks: process presence, heartbeat freshness, Redis queue backlog.
- System checks: disk, memory, CPU load, required processes.
- YAML configuration with strict validation, `${VAR}` environment
  expansion, and sensible defaults; `shipcheck init` writes an example.
- Terminal and JSON reporting (`--json`); warnings do not fail a release
  unless `--fail-on-warn` or `severity: critical`.
- GitHub Actions and GitLab CI examples.
- Documentation: README, architecture, configuration, checks, CI/CD,
  security, and architecture decisions (including the Go choice).
- Test suite: config, engine, registry, reporting, security, and every
  check category, using real local servers (httptest, fake RESP server,
  fake Docker Unix-socket API) — no fake passing checks.

### Known limitations

- System checks (memory, load, process scan) are implemented for Linux
  (`/proc`); on other platforms they return a clear error instead of
  guessing.
- Database and Redis checks have been tested against unreachable targets
  and local fakes, not against live PostgreSQL / MySQL / Redis servers
  in this environment; see docs/CHECKS.md for the honest verification
  status of each check.
- Docker checks require access to the Docker Unix socket.
- No Kubernetes support in v0.1.0 by design.
