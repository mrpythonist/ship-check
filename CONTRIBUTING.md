# Contributing to ShipCheck

Thanks for helping make releases verifiable. This guide is short on purpose.

## Ground rules

- **No fake checks.** A check must verify real behaviour against a real
  target. Tests must prove it: HTTP against `httptest`, Redis against the
  fake RESP server in `internal/checks/redis`, Docker against the fake
  Unix-socket API in `internal/checks/docker`. A test that cannot fail is
  not a test.
- **Production-safe by default.** Checks are read-only and non-destructive.
  No shell execution from config, no credential logging (use
  `internal/security.Redact`), bounded timeouts, no destructive queries.
- **Small, focused changes.** One check or one feature per PR. Keep files
  focused; no massive files with unrelated responsibilities.
- **Docs with code.** If behaviour changes, update the README and the
  relevant doc in `docs/` in the same PR.

## Development setup

Requirements: Go 1.23 or newer. No other toolchain is needed.

```bash
git clone https://github.com/mrpythonist/ship-check.git
cd ship-check
go test ./...   # all tests
go vet ./...    # static checks
gofmt -l cmd internal pkg   # formatting, must print nothing
make build      # builds ./shipcheck
make test       # vet + test
./test/smoke.sh # end-to-end smoke test (needs python3 for the test server)
```

`make build` sets the version string with `-ldflags`. A plain `go build`
works too, but the version then reports the compiled-in default. See
[docs/development.md](docs/development.md).

## Branches and commits

- Work on a feature branch off `main`, named `feature/<short-name>` or
  `fix/<short-name>`. Example: `feature/mysql-sslmode` or
  `fix/redis-timeout`.
- One logical change per commit. Write the commit message in the
  imperative mood, with a short summary line, for example:
  `Add TLS minimum-days warning to HTTP checks`.
- Do not commit the built `shipcheck` binary, `.env` files, or any file
  with a real credential in it. The `.gitignore` covers the usual
  cases, but check `git status` before you commit.
- Do not rewrite history on `main`. Rebase your own branch if it falls
  behind; never force-push to `main`.

## Adding a check

1. Implement `checks.Check` from `pkg/checks` in a new package under
   `internal/checks/<category>/`.
2. Add a `Factory(cfg any) ([]checks.Check, error)` and register it in
   `internal/cli.DefaultRegistry` (or your own registry in tests).
3. Add config fields in `internal/config`, with validation and defaults.
4. Add tests that fail when the target is actually broken.
5. Document it in `docs/CHECKS.md` and the README table.

## Tests

- Run `make test` before opening a PR. All tests must pass and
  `gofmt -l cmd internal pkg` must print nothing.
- New behaviour needs a test that proves it. If you could not verify a
  check against a live target (real Postgres, Redis, or Docker), say so
  in the PR and in the docs. Do not claim live verification you did not do.
- The smoke test (`./test/smoke.sh`) covers the PASS and FAIL paths
  end to end. Run it when you touch the CLI, engine, or reporting.
- Manual integration scenarios (database down, worker stopped, slow
  endpoint, and more) are listed in `test/README.md`.

## Pull requests

- Open the PR against `main` from your feature branch.
- Fill in the pull request template: what changed, how you verified it
  (commands and output), and what you could not verify live.
- Keep the CHANGELOG entry under `[Unreleased]` if you add user-visible
  behaviour.
- One reviewer approval is enough for most changes. Large or risky
  changes (engine verdict logic, security handling, config validation)
  get extra review.
- If review asks for changes, push new commits to the same branch.
  Do not open a second PR for the same work.

## Reporting security issues

Do not open a public issue for a security vulnerability. See
[SECURITY.md](SECURITY.md).

## Code of Conduct

Be kind and direct. See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
