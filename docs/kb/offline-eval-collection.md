# Offline Eval Collection

## Surfaces

Current implementation lives in:

- `cmd/poker/main.go`
- `internal/eval/load.go`
- `internal/eval/collect.go`
- `internal/eval/compare.go`
- `docs/session-artifacts.md`
- `docs/eval-system.md`

## Role in the root workflow

`poker experiment analyze <id>` and `poker experiment go <id>` collect missing per-session summaries before rendering the experiment report.

For every planned session classified as `present`, collection writes:

```text
sessions/<session-id>/eval.json
```

Only missing `eval.json` files are collected by the root command. The summary is derived from existing artifacts and is safe to regenerate.

## Loader behavior

`internal/eval` is an offline artifact reader, not a match runner.

Current behavior:

- `LoadSession(sessionDir)` requires readable `manifest.json` and `hands.jsonl`.
- v0 sessions are expected to contain one match in `manifest.matches[0]`.
- per-seat agent artifact discovery follows manifest seat order under `agents/<seat.Name>/`.
- `pi-session.jsonl`, `memory-export.json`, and `stderr.log` are optional per seat.
- optional artifacts fail collection if present but malformed or unreadable.

Collection never opens `memory.akg` directly and does not depend on the AKG SDK. Memory analysis comes from `memory-export.json` when that additive export exists.

## Parsed observability data

Pi-session parsing currently counts:

- decision prompts from real Pi user messages and test fixture events
- tool calls from assistant message content blocks with `type == "toolCall"`
- tool-call totals keyed by exact tool name

Retry parsing comes from `stderr.log` lines for:

- decision-attempt failures
- malformed-action JSON retries
- exhausted retry fallback actions
- maximum attempts observed

This keeps offline summaries useful without replaying model behavior.

## Metric semantics

`eval.json` follows the schema in [`../session-artifacts.md`](../session-artifacts.md).

Current derivations include:

- session duration from manifest timestamps
- preflop-only hand count from `hands.jsonl`
- showdown hand count from hand records
- fallback action count from forced or automatic actions
- biggest swing hand from the largest positive winner delta in hand results
- chip deltas from manifest match results
- tool calls per hand from parsed Pi transcripts
- memory node/edge summaries from `memory-export.json`
- `source_artifacts` paths for artifacts actually used

`eval.json` is a derived convenience summary. If it disagrees with primary artifacts, the primary artifacts win.

## Constraints to preserve

- Collection stays offline and artifact-only.
- `eval.json` remains additive and regenerable.
- Memory summaries stay sourced from `memory-export.json`, not direct AKG parsing.
- Per-seat ordering follows manifest seat order.
- Loader behavior remains compatible with both real Pi logs and deterministic fixtures.

## Related references

- [`../eval-system.md`](../eval-system.md)
- [`../session-artifacts.md`](../session-artifacts.md)
- [`experiment-execution-and-coverage.md`](experiment-execution-and-coverage.md)
- [`experiment-comparison-and-operator-workflow.md`](experiment-comparison-and-operator-workflow.md)
