# Checks

Every check returns a structured result: status (`pass` / `fail` /
`warn` / `error`), duration, a human message, and a recommendation when
it fails. Severity decides the release verdict: `critical` failures fail
the release; `warning` failures only warn.

## HTTP — `checks.http`

One check per endpoint.

| Field | Notes |
| --- | --- |
| `url` | Required, `http(s)://`. |
| `expected_status` | Default `200`. |
| `timeout` | Per-endpoint, default `5s`. |
| `max_response_time_ms` | Fail if slower. |
| `body_contains` / `body_not_contains` | Substring match on the first 1 MiB of the body. |
| `check_tls` / `tls_min_days` | For HTTPS: fail if the certificate is expired, warn if it expires within `tls_min_days` (default 14). |

**Honest status:** tested against local `httptest` servers (pass, wrong
status, body mismatch, slow timeout, unreachable). TLS expiry is
implemented via a real TLS handshake; it has not been exercised against
a live expiring certificate in this environment.

## Database — `checks.database`

PostgreSQL (`driver: postgres`) or MySQL (`driver: mysql`). Connects,
pings, and runs a read-only query (`query`, default `SELECT 1`).
Configure with `dsn` (preferred, from `${DATABASE_URL}`) or
`host`/`port`/`user`/`password`/`database`. Destructive queries are
rejected by config validation. Credentials are redacted from every
error message.

**Honest status:** tested against unreachable targets (fails cleanly,
no password in the message). Not yet run against live PostgreSQL /
MySQL servers in this environment — see the README's known limitations.

## Redis — `checks.redis`

Raw RESP client, no extra dependency. Sends `PING`, then `SET` / `GET` /
`DEL` on `shipcheck:probe:<random>` with a 10-second TTL. Application
keys are never read or written. A failed `DEL` is a warning (the key
expires on its own), not a failure.

**Honest status:** tested against a fake RESP server in the test suite
and against unreachable targets. Not yet run against a live Redis
server in this environment.

## Docker — `checks.docker`

Talks to the Docker Engine API over the Unix socket (`socket`, default
`/var/run/docker.sock`). Verifies the daemon answers `/_ping`, lists
containers, and for each required container checks it is running, its
health status (when `require_health: true`), and its restart count
against `max_restarts`. Read-only: nothing is started or stopped.

**Honest status:** tested against a fake Unix-socket API in the test
suite. Not yet run against a live Docker daemon in this environment.

## Workers — `checks.workers`

Three independent checks, because a running process does not prove
jobs are being processed:

1. **Process** (`processes[].pattern`, `min_count`): scans `/proc`
   cmdlines for a substring match. No shell is involved.
2. **Heartbeat** (`heartbeat_file`, `heartbeat_max_age`, default 5m):
   the worker must touch this file regularly (for example, on every
   job or every minute). A stale heartbeat fails the release even if
   the process is running — this is the strongest signal.
3. **Queue backlog** (`queues[].key`, `max_length`, `address`): reads
   the Redis list length (`LLEN`) for a queue key and fails when it
   exceeds the threshold.

**Laravel Horizon:** Horizon stores queue state in Redis. Point a
queue check at the Horizon queue key (for example `queues:default`)
and a heartbeat check at a file your Horizon supervisor touches, or
at Horizon's own heartbeat if you expose one. A dedicated Horizon
API check is a Phase 2 item.

**Honest status:** process and heartbeat checks are tested locally
(fresh/stale/missing). Queue backlog is tested through the fake RESP
server's `LLEN`.

## System — `checks.system`

- **Disk** (`disk_path`, `disk_max_usage_percent`, default 90): `Statfs`.
- **Memory** (`memory_max_usage_percent`, default 90): `/proc/meminfo`.
- **Load** (`max_load_1m`, 0 = disabled): `/proc/loadavg`.
- **Processes** (`processes[]`): same `/proc` scan as workers.

These are `warning` severity by default; set `severity: critical` to
make them fail a release.

**Honest status:** tested on Linux in this environment. On other
platforms, memory/load checks return a clear `error` result instead of
guessing.
