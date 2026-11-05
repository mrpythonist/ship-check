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

## Development

```bash
go test ./...   # all tests
go vet ./...    # static checks
gofmt -l .      # formatting (must be clean)
make build      # builds ./shipcheck
make test       # vet + test
```

Go 1.23+ is required.

## Adding a check

1. Implement `checks.Check` from `pkg/checks` in a new package under
   `internal/checks/<category>/`.
2. Add a `Factory(cfg any) ([]checks.Check, error)` and register it in
   `internal/cli.DefaultRegistry` (or your own registry in tests).
3. Add config fields in `internal/config`, with validation and defaults.
4. Add tests that fail when the target is actually broken.
5. Document it in `docs/CHECKS.md` and the README table.

## Pull requests

- Describe what you verified and how (commands + output).
- Note any check you could *not* verify live, and why.
- Keep the CHANGELOG entry under `[Unreleased]` if you add user-visible
  behaviour.

## Code of Conduct

Be kind and direct. See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
