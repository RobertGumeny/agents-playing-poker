# Offline Eval Collection

## Surfaces

The current implementation and operator-facing references live in:
- `cmd/poker-eval/main.go`
- `cmd/poker-eval/main_test.go`
- `internal/eval/load.go`
- `internal/eval/load_test.go`
- `internal/eval/collect.go`
- `internal/eval/collect_test.go`
- `docs/session-artifacts.md`
- `docs/eval-system.md`

## Loader behavior

`internal/eval` is an offline artifact reader, not a match runner.

Current behavior:
- `LoadSession(sessionDir)` requires readable `manifest.json` and `hands.jsonl`
- the loader also requires `manifest.Matches[0]` because v0 sessions are single-match sessions
- per-seat agent artifact discovery follows manifest seat order under `agents/<seat.Name>/`
- `pi-session.jsonl`, `memory-export.json`, and `stderr.log` are optional per seat
- if an optional artifact is present but unreadable or malformed, loading fails instead of silently skipping it

Important boundary: eval collection never opens `memory.akg` and does not depend on the AKG SDK. Any memory analysis comes only from `memory-export.json` when that additive export already exists.

## Pi-session and retry parsing semantics

The loader intentionally matches the artifact shapes already present in this repo.

Current parsing rules:
- decision prompts count as either fixture `fake_pi_session` events or real Pi user `message` events
- tool calls are counted only from assistant `message` events whose content items use `type == "toolCall"`
- tool counts are keyed by tool name exactly as logged
- retry metrics come from `stderr.log` lines matching `decision attempt N/M failed: ...`
- malformed-action retries are the subset of attempt failures whose log line contains `malformed action JSON`
- exhausted retries are counted from `decision engine exhausted retries; using safe fallback action`
- `max_attempts_observed` is the largest `M` seen in retry lines

This keeps collection compatible with both checked-in real Pi logs and the fake fixtures used by tests.

## `poker-eval collect` behavior

`poker-eval collect <session-dir>...` loads each named session directory, computes a summary, and writes `eval.json` next to the primary session artifacts.

Current behavior:
- requires at least one positional session directory
- processes the provided directories in argument order
- writes pretty-printed JSON with a trailing newline
- prints one line per successful session: `collected session_id=<id> output=<path>`
- rewrites `eval.json` deterministically from source artifacts only

The collector is additive. It does not mutate `manifest.json`, `hands.jsonl`, agent logs, or memory exports.

## Metric and summary semantics

`CollectSession` mirrors the stable contract in `docs/session-artifacts.md`.

Current derivations:
- session duration is `ended_at - started_at` in whole seconds; missing timestamps produce `0`
- `preflop_only_hands` counts hands whose recorded actions never leave `street == "preflop"`
- `showdown_hands` counts `showdown_reached == true`
- `fallback_action_count` counts `auto_fold`, `auto_check`, and any action carrying `forced_reason`
- `biggest_swing_hand` uses the largest positive winner `chips_delta` found in a hand result, not reconstructed gross pot size
- `chips_delta` comes from the manifest match result map for each seat
- `tool_calls_per_hand` is a simple `tool_calls[name] / hand_count` rate
- `memory_export` is summarized generically as node and edge totals plus per-type and per-relation buckets
- `source_artifacts` stores relative paths from the session root to the artifacts actually used

Important boundary: `eval.json` is a derived convenience summary. If it disagrees with `manifest.json`, `hands.jsonl`, `pi-session.jsonl`, or `memory-export.json`, the source artifacts still win.

## Test coverage

Deterministic coverage includes:
- loading real checked-in Pi session logs with current message and tool-call shapes
- loading fake fixture logs without requiring live Pi sessions
- optional `memory-export.json` parsing and generic graph summarization
- `stderr.log` retry parsing, including malformed-action retries and exhausted fallback counts
- session-level metric derivation for preflop-only hands, showdown rate, fallback actions, and biggest swing
- deterministic `eval.json` writing with trailing newline output
- CLI coverage for `poker-eval collect`

## Constraints to preserve

When extending this area, preserve these assumptions unless the focused docs change first:
- collection stays offline and artifact-only
- `eval.json` remains additive and safely regenerable
- loader behavior remains compatible with both real Pi logs and checked-in fake fixtures
- memory summaries stay generic and sourced from `memory-export.json`, not from direct `memory.akg` inspection
- per-seat ordering continues to follow manifest seat order so summaries line up with session authority

## Related references

- [`../session-artifacts.md`](../session-artifacts.md)
- [`../eval-system.md`](../eval-system.md)
- [`experiment-planning-and-session-artifacts.md`](experiment-planning-and-session-artifacts.md)
- [`experiment-execution-and-coverage.md`](experiment-execution-and-coverage.md)
