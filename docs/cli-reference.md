# CLI Reference

ShipCheck is a single binary, `shipcheck`. Every command and flag below
is taken from `internal/cli/cli.go`. There are no hidden flags.

## Global usage

```
shipcheck init [--config shipcheck.yml] [--force]
shipcheck verify [--config shipcheck.yml] [--json] [--verbose] [--fail-on-warn]
shipcheck doctor [--config shipcheck.yml]
shipcheck list-checks
shipcheck version
```

Running `shipcheck` with no arguments prints the usage text to stderr
and exits with code 2. `shipcheck help`, `shipcheck --help`, and
`shipcheck -h` print the usage text to stdout and exit with code 0.
An unknown command prints `unknown command "..."` plus the usage text
and exits with code 2.

## `shipcheck version`

Prints the version and exits 0.

```
shipcheck version
shipcheck --version
shipcheck -v
```

Output:

```
shipcheck v0.1.0
```

The version string is set at build time with `-ldflags` (see
`Makefile`). A plain `go build` without those flags reports the
compiled-in default, `v0.1.0`.

## `shipcheck init`

Writes a starter `shipcheck.yml` and exits 0.

```
shipcheck init [--config shipcheck.yml] [--force]
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `--config` | `shipcheck.yml` | Path of the config file to write |
| `--force` | off | Overwrite the file if it already exists |

Behaviour:

- If the file already exists and `--force` is not given, init prints
  `<path> already exists. Use --force to overwrite it.` to stderr and
  exits with code 2. Nothing is overwritten.
- If the file cannot be written (bad path, no permission), init prints
  the error and exits with code 3.
- On success it prints `Created <path>` and reminds you to edit the
  endpoints and credentials, then run `shipcheck verify`.

The starter config enables only the HTTP category, with one example
endpoint (`https://example.com/api/health`). Database, Redis, Docker,
workers, and system checks are present in the file but disabled. Edit
the file before verifying a real release. The full starter text is the
`exampleConfig` constant in `internal/cli/cli.go`.

## `shipcheck verify`

Runs every enabled check and prints the report. This is the command CI
pipelines run.

```
shipcheck verify [--config shipcheck.yml] [--json] [--verbose] [--fail-on-warn]
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `--config` | auto-detected | Path to the config file |
| `--json` | off | Print a machine-readable JSON report instead of terminal output |
| `--verbose` | off | Show a message for every check, including passing ones |
| `--fail-on-warn` | off | Exit 1 when any check warns, instead of exiting 0 |

Config file detection, when `--config` is not given: ShipCheck looks
for `shipcheck.yml`, then `shipcheck.yaml`, then `.shipcheck.yml` in the
current directory, and uses the first one that exists. If none exists
it tries `shipcheck.yml` and reports the read error.

Config file settings also apply. `reporting.format: json` turns on JSON
output and `reporting.verbose: true` turns on verbose output, so a flag
or the config setting is enough.

Exit codes:

| Code | Meaning |
| --- | --- |
| 0 | Release verified (warnings only, unless `--fail-on-warn`) |
| 1 | Release FAILED: a critical check failed, or a check errored |
| 2 | Configuration error: the file could not be read, parsed, or validated |
| 3 | Internal error: the checks could not run, or the report could not be rendered |

### Terminal report

The default output starts with the project and environment, then one
line per check:

```
[PASS] Local Health                       2ms
[FAIL] Database                           518µs
       request to ... failed: connection refused
       Recommendation: Check the application is running and reachable from this host.
```

A check name longer than 34 characters is truncated in terminal output.
The message and recommendation print for every non-passing check, and
for passing checks too when `--verbose` is set.

The summary lists total, passed, failed, warnings, and errors, then the
verdict (`PASSED`, `FAILED`, `WARNING`, or `ERROR`), the first actionable
failure with its recommendation, and the exit code. The verdict and exit
code follow these rules, from `internal/engine`:

- Any critical failure, or any check error, fails the release (exit 1).
- Warnings alone give verdict `WARNING` and exit 0, unless
  `--fail-on-warn` is set, in which case they exit 1.
- No failures, warnings, or errors gives verdict `PASSED` and exit 0.

### JSON report

`shipcheck verify --json` prints a single indented JSON object. The
shape is `engine.Report`:

| Field | Meaning |
| --- | --- |
| `project` | `project.name` from the config |
| `environment` | `project.environment` from the config |
| `timestamp` | When the run started |
| `duration_ms` | Whole run duration, in whole milliseconds |
| `verdict` | `PASSED`, `FAILED`, `WARNING`, or `ERROR` |
| `exit_code` | 0 or 1 for a completed run |
| `checks` | One object per check |
| `total`, `passed`, `failed`, `warnings`, `errors` | Counts |

Each check object (`checks.Result`) has `check_id`, `name`, `category`,
`status` (`pass`, `fail`, `warn`, `error`, `skipped`), `severity`
(`critical` or `warning`), `duration_ms` (whole milliseconds),
`message`, and, when present, `recommendation`, `error`, and `details`.

## `shipcheck doctor`

Validates the config and lists the checks that would run, without
executing any of them.

```
shipcheck doctor [--config shipcheck.yml]
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `--config` | auto-detected | Path to the config file, same detection as `verify` |

Output:

```
ShipCheck v0.1.0
Config: shipcheck.yml
PASS config is valid (project my-saas, environment production)
Enabled checks: http, database
Discovered 3 check(s):
  - [http] API Health (http:api-health, critical)
  - [database] PostgreSQL Connection (database:connection, critical)
  ...
Doctor finished. Run `shipcheck verify` for a full verification.
```

Doctor also prints read-only preflight facts:

- When Docker checks are enabled, whether the Docker socket exists,
  or a `WARN` line if it is not reachable.
- When worker checks are enabled with a heartbeat file, a `WARN` line
  if the heartbeat file is missing.

Exit codes: 0 when the config is valid, 2 when the config is invalid,
3 when the checks cannot be built from a valid config. Use doctor in CI
as a cheap config check before the deploy step.

## `shipcheck list-checks`

Lists the built-in check categories. Takes no flags and exits 0.

```
shipcheck list-checks
```

Output:

```
Available verification checks:
  database - Database Health
      PostgreSQL / MySQL connectivity and read-only query.
  docker - Docker Health
      Daemon, required containers, health status, restart count.
  http - HTTP Health
      Endpoint status, response time, body validation, TLS expiry.
  redis - Redis Health
      PING plus namespaced temporary read/write round-trip.
  system - System Health
      Disk, memory, CPU load, required processes.
  workers - Worker Health
      Worker processes, heartbeat freshness, queue backlog.
Enable a category in shipcheck.yml under checks: to use it.
```

Categories print in sorted order. The descriptions are the registry
metadata in `internal/cli.DefaultRegistry`.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Pass, or warnings only |
| 1 | A critical check failed, or a check errored |
| 2 | Configuration or usage error (bad config, bad flags, unknown command, file already exists on `init`) |
| 3 | Internal execution error (checks could not run, report could not be rendered, config could not be written on `init`) |

A check that errors (it crashed or could not run at all) fails the
release, because an unverified release must not be trusted. The engine
recovers panics in a check and turns them into an `error` result instead
of crashing the whole run.
