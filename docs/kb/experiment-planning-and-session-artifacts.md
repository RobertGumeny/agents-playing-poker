# Experiment Planning and Session Artifacts

## Surfaces

The current implementation and contracts live in:
- `docs/experiment-definition.md`
- `internal/experiment/definition.go`
- `internal/experiment/definition_test.go`
- `docs/session-artifacts.md`
- `internal/sessionlog/memory_export.go`
- `internal/sessionlog/memory_export_test.go`
- `internal/match/runner.go`
- `internal/match/runner_test.go`

## Experiment-definition behavior

`internal/experiment` is a narrow planning package, not a session runner.

Current behavior:
- parsing is JSON-only via `encoding/json`
- unknown fields fail fast through `DisallowUnknownFields`
- every definition must provide `id`, `hands_per_session`, `control`, and `treatment`
- each group must use exactly one session mode: `session_base` + `sessions_count` or explicit `sessions`
- group labels are fixed as `control` and `treatment`
- `expected_direction` currently accepts only `increase` and `decrease`
- omitted `seeds` deterministically default to positional `1..N`

Important boundary: the package only expands planned sessions into `(group label, session id, seed)` tuples. It does not verify that sessions exist on disk and does not collect metrics.

## Memory-export behavior

`sessionlog.WriteMemoryExport(agentDir)` is the only EPIC-11 runtime code path.

Current teardown semantics:
- `internal/match.Runner` closes agents first
- the runner writes `manifest.json`
- the runner then attempts `WriteMemoryExport` for each agent directory
- export failure is logged non-fatally through `ProgressWriter` or `stderr`
- missing `memory.akg` is a no-op
- unreadable `memory.akg` does not flip a completed session to failed

The export is intentionally generic:
- all nodes are copied with `type`, `id`, `title`, `body`, `meta`, and `tags`
- all outbound edges are copied with `from`, `relation`, `to`, and `meta`
- `meta` is normalized to `{}` instead of `null`
- `tags` is normalized to `[]` instead of `null`
- the JSON snapshot does not hard-code poker-specific node inventories

`memory.akg` remains the authoritative durable store. `memory-export.json` is a derived analysis snapshot.

## Artifact authority model

The authority split that eval tooling must preserve:
- `manifest.json` and `hands.jsonl` remain the primary server-authored session records
- `memory.akg` remains the primary agent-authored memory store
- `memory-export.json` is additive and replaceable from `memory.akg`
- `eval.json` is a derived offline summary artifact and must never override the primary artifacts

The stable `eval.json` schema is in `docs/session-artifacts.md`; `poker-eval collect` is the collector that writes it.

## Test coverage

Deterministic coverage includes:
- valid session-base and explicit-session experiment definitions
- invalid experiment definitions, including unknown fields, duplicate session ids, bad seed lengths, and unsupported direction values
- graph export shape from a real AKG store
- no-op behavior when `memory.akg` is absent
- unreadable-memory export failures
- runner teardown integration for successful export and non-fatal export warnings

## Constraints to preserve

When extending this area, preserve these assumptions unless the focused docs change first:
- experiment definitions stay JSON and stdlib-decodable
- planned-session expansion stays deterministic
- additive artifacts must not change match outcomes or completion semantics
- offline summaries must yield to `manifest.json`, `hands.jsonl`, `pi-session.jsonl`, and `memory-export.json` when they disagree
- memory-export stays generic so later tooling is not coupled to the current durable-agent node taxonomy

## Related references

- [`../experiment-definition.md`](../experiment-definition.md)
- [`../session-artifacts.md`](../session-artifacts.md)
- [`../eval-system.md`](../eval-system.md)
- [`experiment-execution-and-coverage.md`](experiment-execution-and-coverage.md)
- [`server-orchestration.md`](server-orchestration.md)
- [`llm-akg-durable-active-retrieval.md`](llm-akg-durable-active-retrieval.md)
