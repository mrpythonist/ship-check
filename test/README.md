# Integration tests

Unit tests live next to the code they test and use real local servers
(`httptest`, a fake RESP server, a fake Docker Unix-socket API) - see
each package's `*_test.go`.

## Manual integration scenarios

Run these against a real stack before a release:

1. **Healthy app** - app + Postgres + Redis + Docker all up:
   `shipcheck verify` exits 0.
2. **Database down** - stop Postgres: the database check fails, exit 1,
   and the message names the database, not a generic error.
3. **Redis down** - stop Redis: the Redis check fails, exit 1.
4. **Worker stopped** - stop the queue worker and let the heartbeat go
   stale: the heartbeat check fails even though the app still answers HTTP.
5. **Unhealthy container** - stop a required container: the Docker
   check fails with the container name.
6. **Slow endpoint** - add latency over `max_response_time_ms`: the
   HTTP check fails with the measured time.
7. **Invalid config** - break a field in `shipcheck.yml`: exit 2 with
   the field named.
8. **Partial failure** - one critical check fails, the rest pass: the
   report lists every check, the verdict is FAILED, exit 1.

## Automated smoke

```bash
./test/smoke.sh   # builds the binary, serves a local HTTP endpoint,
                  # verifies PASS (exit 0), then verifies FAIL (exit 1)
                  # after the server stops, and checks --json parses.
```
