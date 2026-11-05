#!/usr/bin/env bash
# Smoke test: build, verify PASS against a local HTTP server, then FAIL
# after the server stops, and check --json parses.
set -euo pipefail
cd "$(dirname "$0")/.."

go build -o /tmp/shipcheck-smoke ./cmd/shipcheck
work=$(mktemp -d)
trap 'rm -rf "$work"; kill $server_pid 2>/dev/null || true' EXIT

python3 -m http.server 18081 --bind 127.0.0.1 --directory "$work" >/dev/null 2>&1 &
server_pid=$!
sleep 1

cat > "$work/shipcheck.yml" <<'YAML'
version: 1
project: {name: smoke-app, environment: production}
verification: {timeout: 30s, parallel: true}
checks:
  http:
    enabled: true
    endpoints:
      - {name: Local Health, url: http://127.0.0.1:18081/, expected_status: 200, timeout: 5s}
  system: {enabled: true, disk_max_usage_percent: 99, memory_max_usage_percent: 99}
YAML

/tmp/shipcheck-smoke verify --config "$work/shipcheck.yml" >/dev/null
echo "PASS case: exit 0 OK"
/tmp/shipcheck-smoke verify --config "$work/shipcheck.yml" --json | python3 -c "import json,sys; d=json.load(sys.stdin); assert d['verdict']=='PASSED', d"
echo "JSON case: valid JSON, verdict PASSED OK"

kill $server_pid; sleep 1
if /tmp/shipcheck-smoke verify --config "$work/shipcheck.yml" >/dev/null 2>&1; then
  echo "FAIL case: expected exit 1, got 0" >&2; exit 1
fi
echo "FAIL case: exit 1 OK"
echo "Smoke test passed."
