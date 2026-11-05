# CI/CD Integration

ShipCheck needs no hosted service. A pipeline step runs the binary and
uses its exit code: `0` means the release is verified, `1` means it
failed. That is the whole integration.

## GitHub Actions

See [examples/github-actions.yml](../examples/github-actions.yml). The
essential step, after deploy:

```yaml
- name: Verify Production Release
  run: shipcheck verify --json
```

Install the binary in an earlier step (download the release asset, or
`go install github.com/mrpythonist/ship-check/cmd/shipcheck@v0.1.0`).
The workflow in [.github/workflows/ci.yml](../.github/workflows/ci.yml)
runs the project's own tests on every push and PR.

## GitLab CI

See [examples/gitlab-ci.yml](../examples/gitlab-ci.yml):

```yaml
verify_release:
  stage: verify
  script:
    - shipcheck verify --json
```

## Generic shell pipeline

```bash
./deploy.sh || exit 1
shipcheck verify --json || {
  echo "Release verification FAILED — rolling back"
  ./rollback.sh
  exit 1
}
```

## Release workflow

Pushing a tag `v*` runs [.github/workflows/release.yml](../.github/workflows/release.yml),
which builds binaries for Linux, macOS and Windows (amd64/arm64) with
GoReleaser-style `go build` steps and attaches them to the GitHub
release. To cut v0.1.0:

```bash
git tag v0.1.0
git push origin v0.1.0
```
