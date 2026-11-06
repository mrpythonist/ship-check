# Security

ShipCheck runs against production, so it must never become the cause of
an outage. The rules it follows:

- **No credential leakage.** DSNs and URLs are redacted
  (`internal/security.Redact`) before any error reaches a log or a
  report. Tests assert passwords do not appear in failure messages.
- **Read-only database verification.** The default query is
  `SELECT 1`; config validation rejects queries containing `DROP `, and
  the check opens at most one connection. Use a read-only database user.
- **Safe Redis probes.** One uniquely namespaced temporary key,
  `shipcheck:probe:<random>`, with a 10-second TTL, deleted after use.
  Application keys are never touched.
- **No shell execution from YAML.** Nothing in `shipcheck.yml` is
  executed. Process checks scan `/proc` directly; Docker checks use the
  Engine API over the Unix socket.
- **Bounded work.** Every check has a timeout, the whole run has a
  global timeout, HTTP bodies are capped at 1 MiB, and Docker list
  responses are capped at 4 MiB.
- **Read-only Docker checks.** The Docker check never starts, stops,
  or restarts containers.
- **No destructive operations anywhere.** There is no check that writes
  to the application, the database (beyond the temporary Redis probe
  key), or the filesystem, except `shipcheck init` writing the config
  file you asked for.

## Permissions required

- Network access to the configured endpoints, databases, Redis and
  Docker socket - nothing else.
- Read access to `/proc` (Linux) and to the heartbeat file, if configured.
- No root access, except where the Docker socket itself requires it.
