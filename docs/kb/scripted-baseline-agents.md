# Scripted Baseline Agents

## Scope

The scripted baseline surface lives in:

- `cmd/random-agent`
- `cmd/heuristic-agent`
- `internal/randomagent`
- `internal/heuristicagent`
- `cmd/poker-run`
- `cmd/poker-demo` for a local smoke-test wrapper

These agents are sanity-check baselines. They validate protocol handling, rules-engine integration, legal-action enforcement, and artifact generation without live LLM calls. They are not the primary evidence surface for memory-strategy research claims.

## Normative sources

- [`../research.md`](../research.md) for the current strategy lineup
- [`../wire-protocol.md`](../wire-protocol.md) for message flow and action payloads
- [`../domain/texas-holdem.md`](../domain/texas-holdem.md)
- [`../domain/heads-up-nlhe.md`](../domain/heads-up-nlhe.md)
- [`../../README.md`](../../README.md) for current operator commands

## `random`

`internal/randomagent` is intentionally minimal.

Current behavior:

- responds to `session_init` with `session_ready`
- reports version `random/0.1.0`
- ignores notification-only `hand_start` and `hand_end` messages
- exits cleanly on `session_end`
- samples one server-provided legal action
- preserves exact `call` amounts
- samples legal integer totals for `bet` and `raise`

It is deliberately stateless and does not evaluate hand strength. Its job is to prove the server can complete hands against a legal but weak policy.

## `heuristic`

`internal/heuristicagent` is a deterministic no-memory scripted policy.

Current behavior:

- responds to `session_init` with `session_ready`
- reports version `heuristic/0.1.0`
- stores current hole cards
- builds a lightweight decision profile from hole cards and board
- uses deterministic thresholds rather than RNG, memory, search, or solver logic

Current heuristic shape:

- preflop rank/pair/suitedness/connectivity/broadway scoring
- postflop rough made-hand category scoring
- draw bonuses for flush draws, straight draws, and some overcard value
- pot-odds-aware calls/folds when facing a wager
- minimum legal aggression in sufficiently strong spots

It is meant to behave differently from `random`, not to approximate strong poker play.

## Running scripted sanity checks

Prefer the experiment workflow when comparing strategies. For local smoke tests, either run an ad hoc match:

```bash
go run ./cmd/poker-run -agent0 heuristic -agent1 random -hands 25 -seed 17
```

or use the convenience wrapper:

```bash
go run ./cmd/poker-demo
```

`poker-demo` is only a smoke-test wrapper around the same server/match stack. It should not be treated as the central research workflow.

## Interaction with rules and orchestration

Both scripted agents trust `legal_actions` from the server instead of recomputing betting legality. The server remains authoritative for blinds, action order, all-in handling, timeout fallbacks, pot settlement, and artifact writing.

One important rules detail to preserve: if a player cannot cover a blind, the engine allows a short all-in blind, closes betting once only matched all-in states remain, and refunds unmatched commitments before final settlement. Domain semantics live in [`../domain/heads-up-nlhe.md`](../domain/heads-up-nlhe.md); orchestration behavior is covered in [`server-orchestration.md`](server-orchestration.md).

## Test coverage

Relevant deterministic coverage lives in:

- `internal/randomagent/agent_test.go`
- `internal/heuristicagent/agent_test.go`
- `cmd/poker-server/main_test.go`
- `cmd/poker-demo/main_test.go`
- `internal/match/runner_test.go`

Coverage proves that the shipped agents speak the wire protocol, complete sessions, produce valid session bundles, and allow timeout behavior to be tested without live LLM credentials.

## Boundaries

Out of scope for the scripted baselines:

- LLM-backed decision quality
- AKG memory behavior
- prompt-history comparisons
- tournament scheduling
- multiplayer experiments

Those belong to the experiment-first LLM and memory-strategy workflow.
