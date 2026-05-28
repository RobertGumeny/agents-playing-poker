# Experiment Comparison and Operator Workflow

EPIC-14 completed the operator loop around planned experiments by adding experiment bootstrap, experiment discovery, and collected-session comparison reporting.

## Epic delivery summary

The archived EPIC-14 task log splits the work into three slices:
- `EPIC-14-001`: added `poker-eval compare` for treatment/control summaries over collected `eval.json` artifacts
- `EPIC-14-002`: added `poker-eval init` and `poker-eval ls` for experiment bootstrap and discovery
- `EPIC-14-003`: updated the operator docs for the full definition → run → collect → compare workflow

## Current surfaces

The current implementation and operator-facing references live in:
- `cmd/poker-eval/main.go`
- `cmd/poker-eval/main_test.go`
- `internal/eval/compare.go`
- `internal/experiment/definition.go`
- `README.md`
- `docs/eval-system.md`
- `docs/experiment-definition.md`
- `experiments/test-2b-retrieval-throttle.json`

## `poker-eval` command surface after EPIC-14

The experiment CLI now covers the full v0 operator loop:
- `poker-eval init`
- `poker-eval ls`
- `poker-eval status`
- `poker-eval run`
- `poker-eval collect`
- `poker-eval compare`

Responsibility split:
- `poker-run` remains the primitive that executes one real session and writes `sessions/<id>/...`
- `poker-eval` owns experiment planning, coverage inspection, multi-session execution orchestration, offline summary collection, and treatment/control comparison

That split is intentional. `poker-eval` does not clone match-running logic from `poker-run`; it delegates execution when a planned session must actually be launched.

## Bootstrap and discovery behavior

### `poker-eval init`

`init` writes a schema-valid JSON experiment definition directly in the existing experiment contract.

Current behavior:
- requires `-out`
- derives `id` from the output filename when `-id` is omitted
- defaults `hands_per_session` to `25`
- defaults `sessions_count` to `5`
- defaults `control.session_base` to `<id>-control`
- defaults `treatment.session_base` to `<id>-treatment`
- defaults `treatment.opponent` to `control.opponent`
- validates the generated definition before writing it
- prints `initialized experiment id=<id> output=<path> planned_sessions=<n>` on success

Important boundary: `init` scaffolds the durable JSON contract used elsewhere. It does not create a second template format.

### `poker-eval ls`

`ls` is the experiment inventory view.

Current behavior:
- scans `-experiments-dir` recursively for `.json` files
- defaults to `experiments/`
- prints one summary line per valid experiment
- prints `status=invalid` with the parse/validation error for bad definitions instead of failing the whole listing
- also prints deterministic per-group coverage summaries for `control` and `treatment`

This keeps discovery resilient when one checked-in experiment file is broken.

## Comparison behavior

`poker-eval compare` consumes a planned experiment plus collected per-session summaries.

Current behavior:
- requires `-experiment`
- optionally accepts `-sessions-dir` and defaults it to `sessions`
- expands the experiment through the same deterministic planning path used by `run` and `status`
- requires readable `eval.json` for every planned session
- fails on missing collected coverage, malformed collected data, session-id mismatches, seed mismatches, hand-count mismatches, incomplete collected sessions, or unsupported `expected_direction` metrics
- renders Markdown to stdout rather than writing a report file itself

Operators who want a saved report should redirect stdout.

## Comparison report semantics

`internal/eval/compare.go` currently renders these sections:
- experiment heading and optional hypothesis
- `Summary` table for base metrics
- `Tool Use` table when any tool-call-derived metrics were observed
- `Warnings` when observed group metadata is mixed
- `Per-Session Results` table

Base summary metrics are currently:
- `chips_per_hand`
- `session_duration_s`
- `preflop_only_rate`
- `showdown_rate`
- `fallback_action_count`
- `decision_prompt_count_per_session`

Tool metrics are derived from collected seat summaries:
- `<tool>_per_session`
- `<tool>_per_hand`

Direction checks:
- come from `expected_direction` in the experiment definition
- are evaluated as treatment minus control
- render `✅` only when the observed delta matches the expected increase/decrease direction
- render `❌ unchanged` or `❌ <observed> (expected <direction>)` otherwise

## Observed metadata and warnings

Comparison trusts planned identifiers first and uses collected metadata for consistency checks.

Current behavior:
- the planned group `agent` identifier must match exactly one collected seat by name or version
- when the planned group includes `opponent`, that identifier must also match exactly one collected seat
- when a group omits planned `opponent`, compare derives the non-agent seat from collected data
- mixed observed agent identities inside a group produce warnings
- mixed observed opponents produce warnings only when the experiment definition omitted explicit `opponent` metadata for that group

This preserves historical comparisons where the original experiment file did not fully pin opponent metadata while still surfacing drift.

## Operator workflow to preserve

After EPIC-14, the documented v0 workflow is:
1. scaffold or edit an experiment JSON definition
2. inspect discovered experiments with `poker-eval ls`
3. inspect one plan with `poker-eval status`
4. preview or execute the plan with `poker-eval run`
5. derive `eval.json` with `poker-eval collect`
6. render a treatment/control report with `poker-eval compare`

Artifact flow:
- experiment definition JSON is the planning authority
- `poker-run` writes primary session artifacts under `sessions/<id>/`
- `poker-eval collect` adds derived `eval.json`
- `poker-eval compare` reads those collected summaries and emits Markdown to stdout

## Test coverage delivered

EPIC-14 added deterministic coverage for:
- valid experiment template generation through `poker-eval init`
- recursive experiment discovery and coverage summaries through `poker-eval ls`
- comparison report rendering over collected fixtures
- `expected_direction` pass/fail output
- malformed experiment rejection
- incomplete collected coverage rejection
- warnings for inconsistent observed group metadata

The archived task logs recorded the verification recipe:
- `go build ./...`
- `go test ./...`
- `go vet ./...`

## Constraints to preserve

When extending this area, preserve these assumptions unless the focused docs change first:
- experiment JSON remains the only experiment-definition format
- `poker-run` stays the single-session execution primitive
- `compare` continues to operate on collected `eval.json`, not by reopening raw primary artifacts directly
- planned experiment coverage must stay deterministic and definition-driven
- markdown report generation stays stdout-first so callers can redirect or post-process it
- warnings should surface ambiguous observed metadata instead of silently normalizing it away

## Related references

- [`../eval-system.md`](../eval-system.md)
- [`../experiment-definition.md`](../experiment-definition.md)
- [`../session-artifacts.md`](../session-artifacts.md)
- [`experiment-execution-and-coverage.md`](experiment-execution-and-coverage.md)
- [`offline-eval-collection.md`](offline-eval-collection.md)
