# Experiment Execution and Coverage

## Surfaces

Current implementation lives in:

- `cmd/poker/main.go`
- `internal/evalrun/`
- `internal/experiment/definition.go`
- `cmd/poker-run/main.go`
- `docs/eval-system.md`
- `docs/experiment-definition.md`

## Root CLI

The canonical operator surface is the root experiment command:

- `poker experiment status <id>`
- `poker experiment run <id>`
- `poker experiment analyze <id>`
- `poker experiment go <id>`

`status` and `run` load a JSON experiment definition, expand it deterministically, and inspect planned session coverage under the selected sessions root.

Shared flags:

- `-experiments-dir` — defaults to `research/experiments`; expects slug subdirectories containing `<slug>.json`
- `-experiment` — explicit path to an experiment JSON (overrides positional id)

Sessions and reports are derived from the experiment file location rather than separate flags. Given an experiment at `<experiments-dir>/<slug>/<slug>.json`, sessions live at `<experiments-dir>/<slug>/sessions/` and reports at `<experiments-dir>/<slug>/reports/`.

Execution flags for `run` and `go`:

- `-model` — runtime override for Pi-backed agents
- `-thinking-level` — runtime thinking-level override; default is `low`

When `-model` is omitted, the required `model` value from the experiment JSON is used.

## Planning behavior

Planning authority stays in `internal/experiment`.

Current behavior:

- control sessions are planned before treatment sessions
- group order is deterministic
- session-base mode derives `<session_base>-1..N`
- explicit-session mode preserves the listed order exactly
- omitted seeds default to positional `1..N`
- duplicate or conflicting planned session ids are rejected

The experiment definition remains the source of truth for what should exist on disk.

## Coverage inspection

`internal/evalrun` classifies each planned session as:

- `present`
- `missing`
- `incomplete`

Coverage is based on primary artifacts only:

- `manifest.json`
- `hands.jsonl`

A session can be incomplete because artifacts are missing, unreadable, mismatched against the plan, have the wrong seed/hand count, record the wrong agents, or show an incomplete match.

Derived artifacts do not affect coverage classification:

- `report.md`
- `eval.json`
- `memory-export.json`
- agent logs

## Execution behavior

`poker experiment run` and `poker experiment go` launch only sessions that are `missing` or `incomplete`. `present` sessions are skipped.

Execution delegates to the single-session runner from `cmd/poker-run`; the root command builds that binary once and then launches planned sessions. This keeps match execution, server-authoritative rules, process management, timeout behavior, and artifact writing in one place.

A planned group must include `opponent` metadata for live execution. Offline analysis may derive opponents from existing artifacts, but the runner must know both seats before starting a missing session.

## Constraints to preserve

- Experiment JSON remains the planning authority.
- Coverage remains based on `manifest.json` and `hands.jsonl`.
- Runtime overrides must not mutate the checked-in experiment file.
- Present sessions are skipped automatically.
- Missing and incomplete sessions are considered runnable work.
- Single-session execution remains delegated to `cmd/poker-run`.

## Related references

- [`../eval-system.md`](../eval-system.md)
- [`../experiment-definition.md`](../experiment-definition.md)
- [`../session-artifacts.md`](../session-artifacts.md)
- [`experiment-planning-and-session-artifacts.md`](experiment-planning-and-session-artifacts.md)
