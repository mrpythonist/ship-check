# Configuration

ShipCheck reads `shipcheck.yml` (or `shipcheck.yaml` / `.shipcheck.yml`,
or the path passed with `--config`). Run `shipcheck init` to generate a
starter file.

## Full example

See [examples/shipcheck.yml](../examples/shipcheck.yml) (general),
[examples/laravel.yml](../examples/laravel.yml) (Laravel + Horizon),
and [examples/node-api.yml](../examples/node-api.yml) (Node.js API).

## Fields

| Field | Default | Notes |
| --- | --- | --- |
| `version` | - (required, must be `1`) | Config format version. |
| `project.name` | - (required) | Shown in reports. |
| `project.environment` | - (required) | e.g. `production`. |
| `verification.timeout` | `30s` | Global deadline for the whole run. |
| `verification.parallel` | `true` | Run checks concurrently. |
| `verification.fail_fast` | `false` | Stop after the first failure (sequential) / cancel the rest (parallel). |
| `reporting.format` | `terminal` | `terminal` or `json`. |
| `reporting.verbose` | `false` | Show a message for passing checks too. |

Per-category fields are documented in [CHECKS.md](CHECKS.md).

## Secrets

Never commit passwords or tokens. Use environment expansion:

```yaml
checks:
  database:
    dsn: ${DATABASE_URL}
  redis:
    password: ${REDIS_PASSWORD}
```

ShipCheck expands `${VAR}` in the raw file before parsing. If the
variable is unset it expands to an empty string and validation will
fail with a named error, rather than silently using a wrong value.

## Validation

Validation is strict and every error names its field, for example:

```
invalid configuration:
  - checks.http.endpoints[0].url: must be a valid http(s) URL
  - checks.database.driver: must be 'postgres' or 'mysql'
```

Run `shipcheck doctor` to validate config and list the checks that
would run, without executing them.
