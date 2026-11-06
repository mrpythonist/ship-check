## What this changes

## Why

## How I verified it

- `make test` (vet + tests):
- `./test/smoke.sh`:
- Live check (if any), and what target it ran against:

## Notes

- Docs updated (README / docs/):
- CHANGELOG entry added under `[Unreleased]` (if user-visible behaviour changed):
- Known limitations / what I could not verify live:

## Checklist

- [ ] Tests pass (`make test`) and `gofmt -l cmd internal pkg` prints nothing
- [ ] No fake checks: new or changed checks are tested against a real target
- [ ] No credentials committed (check `git status` and `git diff` before pushing)
- [ ] Docs updated in the same PR if behaviour changed
