# LLM Fullhistory Baseline

## Purpose

`llm-fullhistory` is the v0 comparison point between:
- `llm-stateless`, which exposes no prior-hand strategic context
- `llm-akg-recent`, which exposes structured memory through AKG-backed retrieval

It keeps the shared runtime's protocol handling, legality validation, retry budgeting, and safe fallback behavior while changing only:
- the memory policy, which accumulates prior-hand summaries
- the decision engine session scope, which resets Pi context once per hand instead of once per decision

## Shared runtime seam

`pi-agents/shared/` now splits strategy variation across two interfaces:
- **memory policy**: owns cross-hand retained state and prompt-history formatting
- **decision engine**: owns model invocation plus Pi session lifecycle

The shared runner remains strategy-agnostic. It still owns:
- stdio JSONL protocol handling
- prompt assembly
- reply correlation
- legal-action validation
- retry budgeting
- safe fallback behavior
- hand/session lifecycle notifications

This seam keeps `llm-stateless` unchanged in behavior while allowing `llm-fullhistory` to reuse a fresh Pi session for every hand.

## Prior-hand history format

`llm-fullhistory` stores all prior hands by default and injects them as compact one-line summaries in fixed field order:
- hand number
- hero position
- hero hole cards
- final board
- final action summary
- showdown flag
- revealed showdown cards, if any
- hero chip result

The line format is human-readable and derived from protocol-visible hand data. It retains showdown-revealed hole cards and omits unrevealed opponent hole cards in `showdown-only` mode.

## Protocol support added for this baseline

To avoid guessing final actions or whether a pot truly reached showdown, `hand_end` now carries:
- `action_history`: final server-authoritative completed actions for the hand
- `showdown_reached`: explicit showdown flag

That makes prompt-history formatting deterministic and server-authoritative.

## Main code paths

- `pi-agents/shared/src/runner.ts`
- `pi-agents/shared/src/strategy.ts`
- `pi-agents/shared/src/pi-session.ts`
- `pi-agents/llm-fullhistory/src/main.ts`
- `pi-agents/llm-fullhistory/src/history.ts`
- `cmd/poker-run/main.go`

## Verification highlights

Coverage now includes:
- formatter stability for prior-hand lines
- history accumulation and prompt injection order
- per-hand Pi session reset behavior
- seam compatibility with `llm-stateless`
- subprocess wiring for the published `poker-agent-llm-fullhistory` command

## Related references

- [`../research.md`](../research.md)
- [`../wire-protocol.md`](../wire-protocol.md)
- [`llm-stateless-pi-baseline.md`](llm-stateless-pi-baseline.md)
