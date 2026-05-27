# Scripted Baseline Agents And Step-4 Demo

EPIC-4 implemented the non-LLM baseline agents and the end-to-end local demo path for v0.

## Epic delivery summary

The archived EPIC-4 task log shows the step-4 work landed in three slices:
- `EPIC-4-001`: added the minimal `random` Go agent that only chooses from server-advertised legal actions
- `EPIC-4-002`: added the deterministic `heuristic` Go agent and the rules fix needed for short-blind all-in hands to complete cleanly
- `EPIC-4-003`: added CLI-level `poker-server` demo coverage, timeout verification, and operator-facing run guidance

## Scope delivered

The current scripted baseline surface lives primarily in:
- `cmd/poker-demo`
- `cmd/random-agent`
- `cmd/heuristic-agent`
- `internal/randomagent`
- `internal/heuristicagent`
- `cmd/poker-server/main_test.go`
- `cmd/poker-demo/main_test.go`

It now provides:
- two protocol-compliant long-lived Go agents that can complete full matches over stdio JSONL
- one intentionally non-strategic baseline (`random`) and one simple deterministic scripted baseline (`heuristic`)
- server-authoritative legality, where both agents trust `legal_actions` from the wire contract instead of recomputing betting legality
- a documented top-level `go run ./cmd/poker-demo` flow that builds the shipped Go binaries, runs a non-LLM `random` versus `heuristic` match, and prints the resulting session bundle plus the canonical artifacts to inspect next
- `poker-demo` as the primary supported operator entrypoint for build-order step 4
- retention of `poker-server` as the low-level primitive and debugging escape hatch for explicit seat command wiring
- CLI-level proof that timeout enforcement still produces a forced timeout action (`auto_check` when possible, otherwise `auto_fold`) and does not hang the server process

## Normative sources

This step-4 implementation is anchored to the existing repository docs:
- [`../research.md`](../research.md) for the baseline strategy lineup and current experiment framing
- [`../wire-protocol.md`](../wire-protocol.md) for message flow and action payloads
- [`../domain/texas-holdem.md`](../domain/texas-holdem.md)
- [`../domain/heads-up-nlhe.md`](../domain/heads-up-nlhe.md)
- [`../README.md`](../README.md) for the operator-facing demo command

## `internal/randomagent`

`internal/randomagent` is the intentionally minimal baseline.

Current behavior:
- responds to `session_init` with `session_ready` and reports version `random/0.1.0`
- ignores notification-only `hand_start` and `hand_end` messages
- exits cleanly on `session_end`
- samples one entry from `legal_actions`
- preserves exact server-provided `call` amounts
- samples inclusive totals between server-provided `min` and `max` for `bet` and `raise`

Important constraint: the random agent is deliberately stateless and does not evaluate hand strength. Its job is only to demonstrate a legal-action baseline.

## `internal/heuristicagent`

`internal/heuristicagent` is the simple scripted baseline.

Current behavior:
- responds to `session_init` with `session_ready` and reports version `heuristic/0.1.0`
- stores hole cards from `hand_start`
- builds a lightweight decision profile from hole cards plus the current board
- uses deterministic thresholds rather than RNG, memory, search, or solver logic

Current heuristic shape:
- **preflop**: a coarse strength score based on rank, pairs, suitedness, connectivity, and broadway/A-high bonuses
- **postflop made hands**: maps the best current category to a rough equity estimate
- **draw bonuses**: adds flush-draw, straight-draw, and some overcard value
- **action selection**:
  - if no chips are required to continue, prefer the minimum legal `bet` or `raise` when aggression is high enough, otherwise `check`
  - when facing a wager, compare rough equity to pot odds; strong spots prefer the minimum legal aggressive action, weak spots fold when clearly behind, otherwise call when available

Important constraint: this agent is meant to behave differently from `random`, not to approximate strong poker strategy.

## Interaction with rules and orchestration

EPIC-4 also exposed one important rules detail that later work should preserve: if a player cannot cover a blind, the engine allows that player to post a short all-in blind, closes betting once only matched all-in states remain, and refunds unmatched commitments before final settlement. The domain explanation lives in [`../domain/heads-up-nlhe.md`](../domain/heads-up-nlhe.md), while the broader orchestration behavior remains documented in [`server-orchestration.md`](server-orchestration.md).

## Demo and verification surface

The durable operator path for the non-LLM demo is:
1. run `go run ./cmd/poker-demo` from the repo root
2. optionally override a small supported set of match knobs such as `-session-id`, `-sessions-dir`, `-match-id`, `-seed`, or `-hand-count`
3. inspect `sessions/<id>/manifest.json`, `hands.jsonl`, and per-agent logs

For the wrapper-specific UX contract and layering decisions added in EPIC-5, see [`one-command-scripted-demo-flow.md`](one-command-scripted-demo-flow.md).

Execution-relevant constraints to preserve in future operator docs:
- `poker-demo` shells out to `go build`, so a working Go toolchain is required at runtime
- the wrapper compiles `poker-server`, `random-agent`, and `heuristic-agent` into a temporary directory before launching the match
- the wrapper intentionally reuses `poker-server`, so artifact layout, timeout handling, and match semantics stay identical to the lower-level CLI

For lower-level debugging or future wrappers, `poker-server` remains available with explicit `-agent0-cmd` and `-agent1-cmd` wiring.

`cmd/poker-server/main_test.go` currently proves:
- the shipped server binary can run a real `random` versus `heuristic` match and write a valid session bundle
- a sleeping helper agent that exceeds `-decision-deadline` is recorded as a forced timeout action with `forced_reason: "decision_timeout"` (`auto_check` when legal, otherwise `auto_fold`)
- the server still exits cleanly after timeout enforcement

`cmd/poker-demo/main_test.go` proves:
- the wrapper command runs the default scripted match through `poker-server`
- supported CLI overrides still produce a valid session bundle with the requested hand count
- focused wrapper-level unit coverage still guards the argument wiring and session-bundle inspection paths without duplicating full-match integration coverage already exercised by `poker-server` and `internal/match`

Related lower-level coverage remains in:
- `internal/randomagent/agent_test.go`
- `internal/heuristicagent/agent_test.go`
- `internal/match/runner_test.go`

## Current boundaries

Still out of scope here:
- LLM-backed agents
- AKG-backed memory strategies
- prompt-stuffing baselines
- Pi integration and compaction hooks
- multiplayer or tournament scheduling

Those belong to later subsystem work and should be grounded in the focused docs for that layer.

## Why this matters for later work

Future agent and evaluation tasks can treat EPIC-4 as the stable baseline layer for:
- a legal but intentionally weak no-memory policy (`random`)
- a deterministic scripted no-memory policy (`heuristic`)
- the expected local operator workflow for running a complete non-LLM match
- the timeout and artifact behavior that later LLM agents must fit into without changing the server contract
