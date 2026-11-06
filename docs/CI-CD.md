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

Install the binary in an earlier step. Until the first release is
published, build it from source or install the module
(`go install github.com/mrpythonist/ship-check/cmd/shipcheck@latest`);
after the first release, download the release asset for your platform.
The workflow in [.github/workflows/ci.yml](../.github/workflows/ci.yml)
runs the project's own tests on every push and PR. That workflow file
is prepared in this repository; it has to be added to GitHub through
the web UI, because the API cannot push workflow files.

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
  echo "Release verification FAILED - rolling back"
  ./rollback.sh
  exit 1
}
```

## Release workflow

Pushing a tag `v*` runs [.github/workflows/release.yml](../.github/workflows/release.yml),
which builds binaries for Linux, macOS and Windows (amd64/arm64) with
plain `go build` steps and attaches them to the GitHub release. No
release has been published yet, and the workflow file has to be added
to GitHub through the web UI first (the API cannot push workflow
files). Once it is in place, cut v0.1.0 with:

```bash
git tag v0.1.0
git push origin v0.1.0
```
