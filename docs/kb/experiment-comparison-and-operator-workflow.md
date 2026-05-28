# Experiment Comparison and Operator Workflow

## Surfaces

Current implementation lives in:

- `cmd/poker/main.go`
- `internal/evalrun/`
- `internal/eval/compare.go`
- `internal/eval/collect.go`
- `internal/experiment/definition.go`
- `docs/eval-system.md`
- `docs/experiment-definition.md`

## Operator workflow

The current workflow is experiment-first:

1. Create or edit `experiments/<id>.json`.
2. Inspect coverage with `poker experiment status <id>`.
3. Run missing work with `poker experiment run <id>` or do the whole loop with `poker experiment go <id>`.
4. Analyze existing sessions with `poker experiment analyze <id>` when execution is already complete.
5. Review `reports/<id>.md` and the underlying `sessions/<session-id>/` bundles.

`poker experiment go <id>` is the preferred one-command research path because it runs missing/incomplete sessions, collects missing summaries, and writes the comparison report.

## Artifact flow

The artifact chain is:

1. Experiment JSON plans the comparison.
2. `cmd/poker-run` executes each needed session and writes primary session artifacts.
3. `internal/eval` collects `eval.json` summaries from present sessions.
4. `internal/eval` compares control vs treatment and renders Markdown.
5. `cmd/poker` writes `reports/<id>.md`.

The experiment definition is the plan. `manifest.json` and `hands.jsonl` are the primary session authority. `eval.json` and `reports/<id>.md` are derived analysis artifacts.

## Comparison behavior

Comparison expands the experiment through the same deterministic planning path used by status and run, then requires `eval.json` for every planned session.

It validates:

- collected session ids
- seeds
- hand counts
- completion status
- planned agent identity
- planned or derived opponent identity
- supported `expected_direction` metrics

Report content currently includes:

- experiment heading and hypothesis
- aggregate summary table
- tool-use table when tool-call metrics exist
- warnings for mixed observed metadata
- per-session results
- expected-direction pass/fail checks

`expected_direction` is always treatment relative to control.

## Metadata warnings

The comparison layer trusts planned identifiers first and uses observed metadata to detect drift.

Current behavior:

- the planned group `agent` must match exactly one collected seat by name or version
- a planned `opponent` must also match exactly one collected seat
- if `opponent` is omitted, comparison derives the non-agent seat from artifacts
- mixed observed agent or opponent identities produce warnings rather than silent normalization

This keeps old or partially specified experiment files inspectable while surfacing ambiguity.

## Constraints to preserve

- The JSON experiment file remains the only experiment-definition format.
- Reports compare one planned experiment at a time.
- Comparison reads collected `eval.json` summaries rather than reopening all raw artifacts directly.
- Markdown reports are deterministic and file-based.
- Warnings should expose ambiguous metadata instead of hiding it.

## Related references

- [`../eval-system.md`](../eval-system.md)
- [`../experiment-definition.md`](../experiment-definition.md)
- [`../session-artifacts.md`](../session-artifacts.md)
- [`experiment-execution-and-coverage.md`](experiment-execution-and-coverage.md)
- [`offline-eval-collection.md`](offline-eval-collection.md)
