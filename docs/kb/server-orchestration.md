# Server Orchestration Foundation

EPIC-3 implemented build-order step 3 from [`../spec.md`](../spec.md): the Go `poker-server` process lifecycle and stdio match orchestration for v0 agent sessions.

## Scope delivered

The current orchestration surface lives primarily in:
- `cmd/poker-server`
- `internal/match`
- `internal/sessionlog`

It now provides:
- spawning exactly two agent processes for a match without shell wrapping
- JSONL stdin/stdout message exchange using the typed `internal/wire` contract
- per-agent `stdout.log` and `stderr.log` capture under the session bundle
- server-owned hand progression driven by `internal/rules`
- decision-deadline enforcement with server-recorded `auto_fold` on timeout
- `manifest.json` and `hands.jsonl` session artifact writing
- deterministic `hands.jsonl` output for a fixed seed and deterministic agents

## Normative sources

This implementation is anchored to repository docs rather than ad hoc runtime behavior:
- [`../spec.md`](../spec.md) for lifecycle, timeout policy, and session outputs
- [`../wire-protocol.md`](../wire-protocol.md) for JSONL envelope and message flow
- [`../domain/texas-holdem.md`](../domain/texas-holdem.md)
- [`../domain/heads-up-nlhe.md`](../domain/heads-up-nlhe.md)

## `internal/match`

`internal/match` owns the live server loop for one v0 heads-up match.

Key responsibilities:
- start each long-lived agent process once per match
- send `session_init`, await `session_ready`, and preserve the reported version for the manifest
- send `hand_start`, `your_turn`, `hand_end`, and final `session_end`
- ignore optional agent `log` messages while continuing to wait for correlated replies
- reject malformed protocol traffic at the wire boundary instead of trusting it
- translate wire `action` payloads into authoritative rules-engine actions
- abort the match if an agent exits unexpectedly or sends invalid protocol data

Timeout policy:
- each `your_turn` wait uses the configured decision deadline
- on timeout, the server applies a fold itself via the rules engine
- the persisted hand artifact records `action: "auto_fold"` with `forced_reason: "decision_timeout"`
- timeout handling stays server-side; agents do not self-report timeout outcomes

## `internal/sessionlog`

`internal/sessionlog` owns v0 artifact layout and JSON encoding.

Current outputs:
- `sessions/<id>/manifest.json`
- `sessions/<id>/hands.jsonl`
- `sessions/<id>/agents/<name>/stdout.log`
- `sessions/<id>/agents/<name>/stderr.log`

Important current shape decisions:
- `hands.jsonl` is streamed one hand per line in play order
- `actions` is the server-authoritative hand log, including forced timeout actions
- manifest match results accumulate per-hand deltas rather than relying on final stack snapshots, which preserves cash-game auto-rebuy economics

## `cmd/poker-server`

The CLI currently exposes a minimal local-run surface:
- session metadata flags (`session-id`, `match-id`, seed, blinds, hand count)
- decision deadline configuration
- explicit executable paths plus repeatable args for seat 0 and seat 1

It is intentionally small for v0 and delegates orchestration behavior to `internal/match`.

## Test coverage

`internal/match/runner_test.go` currently covers:
- full happy-path session execution against helper child processes
- stderr capture into session logs
- decision timeout conversion into persisted `auto_fold`
- byte-for-byte deterministic `hands.jsonl` output for repeated runs with the same seed and deterministic agents

## Current boundaries

Still out of scope here:
- the real `random` and `heuristic` Go agents from build-order step 4
- Pi session log capture (`pi-session.jsonl`)
- multiplayer orchestration
- tournament scheduling / budget gates

## Why this matters for later work

Later agent tasks can treat the current server as the canonical runtime contract for:
- process lifecycle
- reply correlation behavior in practice
- timeout enforcement semantics
- session artifact locations and action-log shape
- deterministic replay expectations for scripted agents
