# Architecture Decisions

## ADR-001: Implementation language — Go

**Context.** The MVP is a CLI that must be easy to install on a VPS, fast
to start in a deploy pipeline, and reliable against production systems.

**Compared.**

| Criterion | Go | TypeScript / Node.js | Python |
| --- | --- | --- | --- |
| Single-binary distribution | Yes — static binary, no runtime | No — needs Node installed | No — needs interpreter + venv |
| Startup time | ~ms | ~100ms+ | ~100ms+ |
| CLI experience | Excellent (cobra-style, stdlib flags) | Good | Good |
| Cross-platform | Excellent | Good | Good |
| Dependency footprint | Tiny stdlib; 3 runtime deps here | npm tree, larger | pip / packaging friction |
| Plugin architecture | Interfaces + registry (this repo) | Good | Good |
| Developer adoption for DevOps CLIs | High (kubectl, Terraform, Docker) | Medium | High for scripts |

**Decision.** Go, one primary language for the core engine.

**Consequences.** Contributors need Go 1.23+. The binary is the whole
distribution — no runtime to install on the target host. The trade-off
is a smaller pool of contributors than Python/JS, accepted because the
product is infrastructure tooling where Go is the established choice.

## ADR-002: One binary, no server, no dashboard in v0.1.0

**Context.** The target user deploys without an SRE team and does not
want another service to run.

**Decision.** The MVP is a CLI only. No hosted dashboard, no SaaS
account, no background agent. CI integration is just the exit code.

**Consequences.** Historical reports and continuous monitoring are
Phase 3+ features, built on top of the same engine and report format —
not a rewrite.

## ADR-003: Warnings do not fail a release by default

**Context.** A disk at 91% is a problem, but it is usually not caused by
the release just deployed.

**Decision.** System resource checks are `warning` severity by default.
Critical failures (HTTP, DB, Redis, Docker, workers) fail the release.
Users can opt into strictness per check (`severity: critical`) or
globally (`--fail-on-warn`).

## ADR-004: Workers need three signals, not one

**Context.** A running worker process can be stuck, deadlocked, or
starved while still appearing in `ps`.

**Decision.** Worker health is process presence *plus* heartbeat
freshness *plus* queue backlog. The heartbeat is the strongest signal:
it proves the worker did work recently. Documented honestly as the
recommended setup for Laravel Horizon and generic queues.
