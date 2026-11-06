# Support

## Using ShipCheck

Start with the [README](README.md): quick start, configuration, and
the full CLI reference in [docs/cli-reference.md](docs/cli-reference.md).
Field-level config help is in [docs/CONFIGURATION.md](docs/CONFIGURATION.md)
and per-check behaviour is in [docs/CHECKS.md](docs/CHECKS.md).

## Common problems

- **Exit code 2 on `verify`:** the config is invalid. The error names
  the offending field. Run `shipcheck doctor` to validate without
  running any checks.
- **A check fails with "connection refused":** ShipCheck reached the
  configured address and nothing was listening. Check the service is
  running and reachable from the machine running ShipCheck, and that
  the address in `shipcheck.yml` is right.
- **A check fails with a timeout:** the target is slow or unreachable.
  Raise the per-check `timeout` or the global `verification.timeout`,
  or investigate why the target is slow.
- **System checks return errors on macOS or Windows:** the system and
  worker process checks read Linux `/proc`. On other platforms they
  return a clear error instead of guessing. Disable the `system`
  category if you only need HTTP, database, Redis, or Docker checks.
- **Docker checks fail with a socket error:** ShipCheck talks to the
  Docker Engine API over the Unix socket (default
  `/var/run/docker.sock`). The user running ShipCheck needs permission
  to read that socket.
- **Secrets in error messages:** ShipCheck redacts DSNs and URLs before
  reporting. If you ever see a credential in output, treat it as a
  security bug and report it privately, as described in
  [SECURITY.md](SECURITY.md).

## Asking for help

Open a GitHub issue on this repository:

- Use the **bug report** template if ShipCheck verified something
  incorrectly, crashed, or gave a misleading result. Include the
  command, the output, the ShipCheck version, the OS, and the relevant
  part of `shipcheck.yml` with secrets redacted.
- Use the **feature request** template for a new check or capability.
  Say what release failure it would catch that ShipCheck misses today.

General questions about setup or config are also welcome as issues.
Before opening one, check the README, `docs/`, and existing issues to
see if the question is already answered.

## Security issues

Do not open a public issue for a security vulnerability. See
[SECURITY.md](SECURITY.md).

## Response time

This is a project maintained by one person alongside other work. Issues
and pull requests are reviewed as time allows. Clear reports with
reproduction steps get answered first.
