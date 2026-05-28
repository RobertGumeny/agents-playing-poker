# LLM Stateless Pi Baseline

EPIC-7 delivered the first runnable Pi-backed LLM agent in the v0 build order: `llm-stateless`.

## Epic delivery summary

The archived EPIC-7 task log shows four delivery slices:
- `EPIC-7-001`: replaced the placeholder decision path with a real Pi SDK-backed per-decision client
- `EPIC-7-002`: finished the `llm-stateless` package rename, build output, and external-process entrypoint wiring
- `EPIC-7-003`: added fake-client integration coverage for the shared stateless protocol path
- `EPIC-7-004`: added subprocess coverage for the published command shape and canonical Pi session artifact handling

## Scope delivered

The current stateless Pi-agent surface lives primarily in:
- `pi-agents/shared/src/runner.ts`
- `pi-agents/shared/src/prompt.ts`
- `pi-agents/shared/src/state.ts`
- `pi-agents/shared/src/pi-session.ts`
- `pi-agents/llm-stateless/src/main.ts`
- `pi-agents/shared/test/runner.test.ts`
- `pi-agents/shared/test/pi-session.test.ts`
- `pi-agents/llm-stateless/test/main.test.ts`
- `pi-agents/README.md`

It now provides:
- a buildable npm workspace for Pi poker agents without introducing repo-wide JS tooling
- a shared stdio JSONL runner that owns protocol decoding, prompt construction, reply correlation, action validation, retry budgeting, and state reset on `hand_end` / `session_end`
- a runnable `poker-agent-llm-stateless` executable suitable for `poker-server -agent*-cmd` plus repeated `-agent*-arg` usage
- a real Pi SDK decision path that creates a fresh in-memory Pi session for every `your_turn`
- optional Pi model and thinking configuration through environment variables rather than hard-coded model selection
- deterministic safe fallback behavior when Pi replies are malformed, illegal, or exhausted their retry budget
- canonical Pi observability logs written to `pi-session.jsonl` in the agent session directory when one is available

## Normative sources

This baseline is anchored to:
- [`../research.md`](../research.md) for strategy definitions and the stateless-vs-memory experiment design
- [`../wire-protocol.md`](../wire-protocol.md) for the server↔agent JSONL contract
- [`../domain/README.md`](../domain/README.md) for Hold'em terms referenced by prompts and payloads
- [`../README.md`](../README.md) for current operator-facing run paths

## Workspace and package boundaries

`pi-agents/shared/` is the reusable runtime layer for future Pi strategies. It contains:
- protocol codecs and envelope validation
- shared session and hand state derived only from protocol messages
- prompt construction from current state plus strategy-provided augmentations
- action parsing and legality fallback
- the outer stdin/stdout runner loop
- the Pi-session persistence seam

`pi-agents/llm-stateless/` is intentionally thin. It defines the no-memory strategy boundary, reads environment configuration, chooses either the real Pi decision client or a scripted fake client, and then delegates to the shared runner.

Future Pi strategies should extend the strategy seam rather than forking the runner.

## What “stateless” means here

For this repository, stateless is a strategy guarantee, not just an implementation convenience.

Current behavior to preserve:
- each `your_turn` creates a fresh Pi SDK session via `createAgentSession(...)`
- the session manager is in-memory only
- Pi compaction, context-file discovery, skills, themes, prompt templates, tools, and retry are disabled for this baseline
- the strategy contributes no prior-hand prompt sections in `beforeDecision`
- `afterHandEnd` deliberately forgets the hand instead of storing strategic memory
- only current protocol state reaches the model prompt

The shared runner still keeps enough local process state to answer the current hand correctly, but that state is not exposed as prior-hand strategic context to the stateless model.

## Prompt and decision contract

The stateless prompt currently includes:
- session and match identifiers
- seat metadata
- current hand number and street
- hero hole cards
- current board
- pot, `to_call`, and stack map
- current-hand action history only
- server-advertised legal actions

The Pi system prompt separately constrains the model to:
- choose exactly one legal action
- return JSON only with shape `{\"action\": string, \"amount\"?: number}`
- keep bet and raise sizes within server-provided integer bounds

The server remains authoritative. If the model returns malformed or illegal output, the shared runtime validates it against `legal_actions` and falls back safely.

## Observability artifacts are not memory

EPIC-7 made an important distinction that later work should preserve:
- Pi session JSONL exports are durable observability and debugging artifacts
- they are not strategic memory for `llm-stateless`
- the agent never reads those logs back into future prompts

Current persistence behavior:
- when `session_init.memory_dir` is present, `llm-stateless` uses it as the default session-log directory
- `PI_POKER_PI_SESSION_DIR` can override that location explicitly
- each decision's exported Pi JSONL is appended into one canonical `pi-session.jsonl`
- the temporary per-decision export file is removed after append

This keeps Pi artifacts aligned with the existing `sessions/<id>/agents/<name>/` bundle layout instead of introducing a parallel artifact convention.

## Runtime knobs

`llm-stateless` currently reads these optional environment variables:
- `PI_POKER_MODEL`
- `PI_POKER_THINKING_LEVEL`
- `PI_POKER_MAX_DECISION_ATTEMPTS`
- `PI_POKER_PI_SESSION_DIR`
- `PI_POKER_FAKE_DECISIONS_JSON` for deterministic subprocess tests only

`PI_POKER_FAKE_DECISIONS_JSON` is a test seam, not part of the research strategy surface.

## Verification surface

The durable automated coverage added by EPIC-7 proves:
- the shared runner replies with correct `session_ready` and `action` envelopes and preserves `in_reply_to` correlation
- later stateless prompts do not contain previous-hand showdown or result data
- state is cleared correctly at `hand_end` and `session_end`
- retry exhaustion logs to stderr and still returns a safe legal action
- the published `poker-agent-llm-stateless` bin is stable for subprocess launching
- the built package can run as a child process with repeated extra args and still speak the Go server contract
- subprocess execution can write the canonical `pi-session.jsonl` artifact without live credentials or billable model calls

The primary commands exercised during the epic were:
- `cd pi-agents && npm run build`
- `cd pi-agents && npm run typecheck`
- `cd pi-agents && npm test`
- `go build ./...`
- `go test ./...`
- `go vet ./...`

## Current boundaries

Still out of scope here:
- match-long prompt stuffing (`llm-fullhistory`)
- AKG-backed retrieval or compaction behavior (`llm-akg-recent`)
- reusing Pi session logs as memory
- live-model integration tests in CI
- any alternate artifact layout outside the canonical session bundle

## Why this matters for later work

Future Pi-agent tasks can treat EPIC-7 as the stable foundation for:
- a runnable external LLM agent process that already fits the Go server contract
- a shared TS runtime layer for later memory strategies
- a strict separation between observability artifacts and strategic memory
- test seams that keep protocol and subprocess coverage deterministic while real-model behavior stays out of CI
