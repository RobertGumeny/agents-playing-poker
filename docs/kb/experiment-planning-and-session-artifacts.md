# Experiment Planning and Session Artifacts

## Surfaces

Current implementation and contracts live in:

- `docs/experiment-definition.md`
- `docs/session-artifacts.md`
- `internal/experiment/definition.go`
- `internal/sessionlog/memory_export.go`
- `internal/match/runner.go`
- `internal/eval/collect.go`

## Experiment-definition behavior

`internal/experiment` is a planning package. It does not run sessions and does not collect metrics.

Current behavior:

- parsing is JSON-only through the standard library
- unknown fields fail validation
- top-level `id`, `model`, `hands_per_session`, `control`, and `treatment` are required
- each group must provide `agent`
- each group uses exactly one session mode:
  - `session_base` + `sessions_count`
  - explicit `sessions`
- group labels are fixed as `control` and `treatment`
- omitted seeds default deterministically to `1..N`
- `expected_direction` accepts `increase` or `decrease`

Planning expands a definition into ordered planned runs containing group label, session id, seed, agent, opponent, and target session directory.

## Memory-export behavior

`sessionlog.WriteMemoryExport(agentDir)` writes an additive JSON snapshot when an agent directory contains `memory.akg`.

Current teardown behavior:

- match runner closes agents first
- match runner writes `manifest.json`
- match runner attempts memory export for each agent directory
- missing `memory.akg` is a no-op
- export failure is logged non-fatally
- export failure does not make a completed match incomplete

The export is generic:

- nodes include `type`, `id`, `title`, `body`, `meta`, and `tags`
- edges include `from`, `relation`, `to`, and `meta`
- missing metadata is normalized to `{}`
- missing tags are normalized to `[]`

`memory.akg` remains the authoritative graph. `memory-export.json` is for offline analysis.

## Artifact authority model

Primary session artifacts:

- `manifest.json`
- `hands.jsonl`

Primary agent memory artifact:

- `agents/<name>/memory.akg`

Derived/additive artifacts:

- `report.md`
- `eval.json`
- `agents/<name>/pi-session.jsonl`
- `agents/<name>/memory-export.json`
- `agents/<name>/stderr.log`

Evaluation tooling must preserve this authority split. Derived summaries are useful for comparison, but they do not override the primary artifacts.

## Constraints to preserve

- Experiment definitions stay JSON and stdlib-decodable.
- Planned-session expansion stays deterministic.
- Additive artifacts must not change match outcomes or completion semantics.
- Offline summaries yield to primary artifacts when they disagree.
- Memory export stays generic and does not encode a strategy-specific node taxonomy.

## Related references

- [`../experiment-definition.md`](../experiment-definition.md)
- [`../session-artifacts.md`](../session-artifacts.md)
- [`../eval-system.md`](../eval-system.md)
- [`experiment-execution-and-coverage.md`](experiment-execution-and-coverage.md)
- [`server-orchestration.md`](server-orchestration.md)
- [`llm-akg-durable-active-retrieval.md`](llm-akg-durable-active-retrieval.md)
