# LLM Fullhistory Baseline

This note captures the intended repository-aligned design for `llm-fullhistory` before implementation lands.

## Purpose

`llm-fullhistory` is the naive full-history-in-context baseline for v0.

It exists to answer a narrow experimental question:
- what happens if the model gets prior hand history stuffed back into context
- while everything else stays as close as possible to the shared Pi runtime baseline

It is intentionally distinct from both:
- `llm-stateless`, which exposes no prior-hand strategic context
- a possible future long-running single-session strategy, which would test session-managed context carryover as its own approach

## Strategy boundary

The current intended behavior is:
- one long-lived poker agent process per match, following the standard wire protocol lifecycle
- one fresh Pi SDK session per hand
- no conversational carryover across hands inside Pi session state
- prior-hand memory exposed explicitly through prompt injection

This makes the comparison cleaner:
- `llm-stateless` differs by exposing no prior-hand history
- `llm-fullhistory` differs by exposing naive explicit history
- `llm-akg` will differ by exposing structured AKG-backed memory with compaction-aware retrieval

## Prior-hand history format

The prompt history should be:
- compact
- human-readable
- line-oriented
- derived from server-authoritative hand history semantics rather than free-form prose

The shape should be inspired by diluted `hands.jsonl` records, but it does not need to mirror on-disk JSON exactly.

Desired contents per prior hand:
- hand number
- position/dealer context if useful
- board
- compact action summary
- showdown reached or not
- result / chip delta
- hole cards only when actually revealed at showdown

Do not include unrevealed opponent hole cards in `showdown-only` mode.

## Implementation constraints

When implementation begins, preserve these boundaries:
- reuse `pi-agents/shared/` rather than building a second protocol stack
- keep legality validation, retry budgeting, and safe fallback in the shared runtime
- keep Pi session logs as observability artifacts, not strategic memory
- avoid over-generalizing abstractions for hypothetical future memory strategies

## Normative references

Use these as the canonical sources when implementation begins:
- [`../spec.md`](../spec.md)
- [`../wire-protocol.md`](../wire-protocol.md)
- [`llm-stateless-pi-baseline.md`](llm-stateless-pi-baseline.md)
