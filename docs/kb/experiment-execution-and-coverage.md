# Experiment Execution and Coverage

## Surfaces

The current implementation and operator-facing references live in:
- `cmd/poker-eval/main.go`
- `cmd/poker-eval/main_test.go`
- `internal/experiment/definition.go`
- `internal/experiment/definition_test.go`
- `docs/experiment-definition.md`
- `experiments/test-2b-retrieval-throttle.json`
- `README.md`

## CLI surface

The experiment-driven CLI surface now includes:
- `poker-eval init`
- `poker-eval ls`
- `poker-eval run`
- `poker-eval status`

The binary also includes `poker-eval collect` and `poker-eval compare`, but those offline summary paths are covered separately in [`offline-eval-collection.md`](offline-eval-collection.md).

Shared behavior for `run` and `status`:
- both require `-experiment`
- both load the JSON definition through `experiment.Load`
- both expand the experiment through `Definition.Plan(sessionsDir)`
- both inspect every planned session directory before printing output
- both print experiment totals, a config line, per-group summaries, and one line per planned session

Current flags:
- `init`: `-out`, `-id`, `-hypothesis`, `-hands-per-session`, `-sessions-count`, `-control-agent`, `-control-opponent`, `-control-session-base`, `-treatment-agent`, `-treatment-opponent`, `-treatment-session-base`
- `ls`: `-experiments-dir`, `-sessions-dir`
- `run`: `-experiment`, `-sessions-dir`, `-dry-run`, `-model`, `-thinking-level`
- `status`: `-experiment`, `-sessions-dir`

`init` derives `id` from the output filename when omitted, defaults control/treatment session bases to `<id>-control` and `<id>-treatment`, and writes a schema-valid JSON template. `ls` recursively scans an experiments directory for `.json` files and prints either coverage summaries or an invalid-definition error per file.

`run` defaults `thinking_level` to `low` so Pi-backed sessions inherit the repo's normal low-thinking default unless the operator overrides it.

## Planning and output semantics

EPIC-12 keeps planning authority in `internal/experiment`.

Current behavior:
- control sessions are planned before treatment sessions
- within each group, session order comes from deterministic expansion already defined by EPIC-11
- `SessionDir` is always `filepath.Join(sessionsRootDir, sessionID)`
- dry-run and live `run` mode print the same deterministic plan and coverage view before any execution starts
- group summary output is forced to `control` then `treatment` order

This means the experiment definition remains the source of truth for what should exist on disk, while the filesystem only answers whether each planned session is already usable.

## Coverage inspection semantics

`loadPlanCoverage` classifies each planned session as one of:
- `present`
- `missing`
- `incomplete`

Current rules:
- missing session directory → `missing`
- any complete manifest + hands match for the planned session → `present`
- any existing directory that fails validation → `incomplete`

`inspectExistingSession` currently marks a session `incomplete` for these reasons:
- `manifest_missing`
- `manifest_unreadable`
- `session_id_mismatch`
- `seed_mismatch`
- `hand_count_mismatch`
- `manifest_missing_match`
- `match_incomplete`
- `agent_missing`
- `opponent_missing`
- `hands_missing`
- `hands_unreadable`
- `hands_count_mismatch`

Important boundary: coverage is derived from `manifest.json` and `hands.jsonl`. The command does not consult `report.md`, `memory-export.json`, or future `eval.json` artifacts when deciding whether a planned session is runnable or already present.

## Execution behavior

`poker-eval run` does not reimplement match execution.

Current execution semantics:
- `present` sessions are skipped
- `missing` sessions are launched
- `incomplete` sessions are also launched again
- execution stops on the first launch failure
- a session that needs execution must have non-empty `opponent` metadata in the experiment definition

Delegation path:
- `poker-eval` builds `./cmd/poker-run` into `.tmp/bin/poker-run`
- it invokes that binary with `-agent0`, `-agent1`, `-hands`, `-seed`, `-session-id`, `-sessions-dir`, and `-thinking-level`
- `-model` is forwarded only when explicitly provided

This preserves `poker-run` as the low-level primitive for single-session execution and avoids duplicating match setup logic inside `poker-eval`.

## Test coverage

Deterministic coverage includes:
- dry-run output showing totals, config, group summaries, and per-session rows
- execution behavior that skips `present` sessions and reruns `missing` or `incomplete` sessions
- runtime failure when execution is required but the planned group omitted `opponent`
- `status` output over planned coverage
- fixture-backed inspection of `present` sessions
- fixture-backed incomplete-session reasons including missing manifests, seed mismatches, incomplete matches, and wrong hand counts
- planner behavior for session-base mode, explicit-session mode, default seeds, and cross-group session-plan conflicts

## Constraints to preserve

When extending this area, preserve these assumptions unless the focused docs change first:
- experiment definitions remain the source of truth for planned coverage
- execution continues to delegate to `poker-run` instead of cloning its match-running logic
- coverage decisions stay tied to primary session artifacts, especially `manifest.json` and `hands.jsonl`
- only `present` sessions are skipped automatically; `incomplete` means rerunnable work today
- output ordering stays deterministic so dry-run and status are scriptable
- runtime-only knobs such as model selection must not mutate the checked-in experiment definition

## Related references

- [`../experiment-definition.md`](../experiment-definition.md)
- [`../session-artifacts.md`](../session-artifacts.md)
- [`../eval-system.md`](../eval-system.md)
- [`experiment-planning-and-session-artifacts.md`](experiment-planning-and-session-artifacts.md)
- [`repeatable-benchmark-reporting.md`](repeatable-benchmark-reporting.md)
