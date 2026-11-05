# Security Policy

## Supported versions

| Version | Supported |
| --- | --- |
| 0.1.x | Yes |

## Reporting a vulnerability

Do **not** open a public issue for a security vulnerability. Email the
maintainer (see the repository owner) with a description, steps to
reproduce, and the impact. You will receive a response within 7 days, and
a fix will be released before public disclosure wherever possible.

## What ShipCheck does with secrets

- ShipCheck never writes credentials to logs or reports. DSNs and URLs
  pass through `security.Redact` before any error is surfaced.
- Secrets belong in environment variables (`${DATABASE_URL}` in
  `shipcheck.yml`), never in committed config files.
- The Redis probe key is uniquely namespaced (`shipcheck:probe:<random>`),
  has a 10-second TTL, and is deleted after the check.
- Database verification is read-only (`SELECT 1` by default); config
  validation rejects destructive queries.
- No YAML value is ever executed as a shell command. Process checks scan
  `/proc` directly.

## Permissions ShipCheck needs

- Network access to the endpoints, databases, Redis and Docker socket
  you configure — nothing else.
- Read access to `/proc` for process and system checks (Linux).
- Read access to the heartbeat file path, if you configure one.
- No root access is required, except where the Docker socket itself
  requires it (that is a Docker permission, not a ShipCheck one).
