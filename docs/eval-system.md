# Experiment and Evaluation System

This document describes the current experiment-first evaluation workflow.

## Operator model

The normal workflow is:

1. Create a slug subdirectory under `research/experiments/` and place the JSON experiment definition inside it:
   `research/experiments/<slug>/<slug>.json`
2. Run:

   ```bash
   poker experiment go <experiment-id>
   ```

3. Review:
   - `research/experiments/<slug>/reports/<experiment-id>.md`
   - `research/experiments/<slug>/sessions/<session-id>/manifest.json`
   - `research/experiments/<slug>/sessions/<session-id>/hands.jsonl`
   - `research/experiments/<slug>/sessions/<session-id>/eval.json`
   - per-agent logs and memory artifacts

The experiment definition is the planning authority. The filesystem answers which planned sessions are present, missing, or incomplete.

## Root CLI surface

The canonical CLI is `poker experiment` from `cmd/poker`.

| Command | Purpose |
| --- | --- |
| `poker experiment status <id>` | Load `research/experiments/<id>/<id>.json` and print planned coverage. |
| `poker experiment run <id>` | Run only missing or incomplete planned sessions. |
| `poker experiment analyze <id>` | Collect missing `eval.json` summaries and write the report into the experiment's `reports/` dir. |
| `poker experiment go <id>` | Run missing/incomplete sessions, collect summaries, and write the report. |

All commands accept `-experiments-dir` and `-experiment` for explicit path overrides. `run` and `go` also accept runtime model controls such as `-model` and `-thinking-level`.

Sessions and reports are derived from the experiment file location: sessions live at `<experiment-dir>/sessions/` and reports at `<experiment-dir>/reports/`.

When `-model` is omitted, the runner uses the required `model` field from the experiment JSON.

## Experiment definition

An experiment definition is a checked-in JSON plan for a two-group comparison:

- `control`
- `treatment`

Example:

```json
{
  "id": "my-memory-test",
  "hypothesis": "Durable AKG memory should improve chip efficiency against stateless play.",
  "model": "anthropic:claude-sonnet-4-6",
  "hands_per_session": 25,
  "control": {
    "session_base": "stateless-control",
    "sessions_count": 5,
    "agent": "llm-stateless",
    "opponent": "heuristic"
  },
  "treatment": {
    "session_base": "akg-durable-treatment",
    "sessions_count": 5,
    "agent": "llm-akg-durable",
    "opponent": "heuristic"
  },
  "expected_direction": {
    "chips_per_hand": "increase",
    "tokens_per_hand": "decrease"
  }
}
```

The normative schema is [`experiment-definition.md`](experiment-definition.md). It defines required fields, group session modes, seed derivation, validation rules, and `expected_direction` semantics.

## Planning and coverage

Experiment planning is deterministic:

- `control` sessions are planned before `treatment` sessions.
- A group either derives sessions from `session_base` + `sessions_count` or lists explicit `sessions`.
- Omitted seeds default positionally to `1..N`.
- Planned session directories are under the experiment's `sessions/` subdirectory (e.g. `research/experiments/<slug>/sessions/`).

Coverage inspection classifies each planned session as:

- `present` — primary artifacts exist and match the plan
- `missing` — no usable session directory exists
- `incomplete` — a directory exists, but primary artifacts are absent, mismatched, or incomplete

Coverage is based on `manifest.json` and `hands.jsonl`. Analysis artifacts such as `report.md`, `eval.json`, and `memory-export.json` do not decide whether a planned session is complete.

## Execution

`poker experiment run` and `poker experiment go` execute missing and incomplete sessions. Present sessions are skipped.

The root command delegates actual single-session execution to the existing match runner binary built from `cmd/poker-run`. This preserves one implementation of match setup, agent process wiring, server-authoritative rules, artifact writing, and timeout behavior.

A planned session can only be launched when the group includes `opponent` metadata. Offline analysis can sometimes infer opponents from existing artifacts, but live execution must know both seats before the run starts.

Runtime-only settings such as `-model` and `-thinking-level` do not mutate the checked-in experiment definition.

## Collection and report generation

`poker experiment analyze` performs two additive analysis steps:

1. For every present planned session missing `eval.json`, collect a deterministic summary from session artifacts.
2. Compare control and treatment summaries and write `<experiment-dir>/reports/<experiment-id>.md`.

`eval.json` is derived from:

- `manifest.json`
- `hands.jsonl`
- optional `pi-session.jsonl`
- optional `stderr.log`
- optional `memory-export.json`

It is safe to regenerate. It must not override primary session truth.

The comparison report includes aggregate treatment/control metrics, per-session rows, tool-use metrics when observed, warnings for inconsistent observed metadata, and `expected_direction` pass/fail checks where supported.

## Artifact authority

Primary session authority:

- `manifest.json`
- `hands.jsonl`

Primary agent memory authority:

- `agents/<name>/memory.akg`

Additive analysis and observability artifacts:

- `report.md`
- `eval.json`
- `agents/<name>/pi-session.jsonl`
- `agents/<name>/stderr.log`
- `agents/<name>/memory-export.json`

The stable additive schemas are documented in [`session-artifacts.md`](session-artifacts.md).

## Implementation map

Current implementation areas:

- `cmd/poker/main.go` — root `poker experiment` CLI
- `internal/experiment/definition.go` — experiment JSON parsing, validation, and deterministic planning
- `internal/evalrun/` — coverage inspection and planned-session execution
- `internal/eval/` — offline session loading, summary collection, comparison, and Markdown rendering
- `internal/sessionlog/` — memory export support

## Scope and non-goals

The v0 evaluation system is intentionally file-based and local.

It does not try to:

- become a dashboard or long-running service
- aggregate unrelated experiment definitions into one report
- provide solver-grade poker statistics
- hide primary artifacts behind a database
- make single-session ad hoc runs the normal research workflow

The durable record is the checked-in experiment JSON plus the generated session/report artifacts.
