# Rules Engine Foundation

EPIC-1 implemented the deterministic Go rules engine foundation for v0 heads-up no-limit Texas Hold'em.

## Scope delivered

The current foundation lives primarily in:
- `internal/deck`
- `internal/rules`

It now provides:
- deterministic card parsing and full-deck construction
- deterministic Hold'em dealing for a given match seed and hand number
- core heads-up NLHE domain types for streets, actions, legal actions, players, and hand results
- hand start / blind posting / dealer-button rotation for repeated cash-game hands
- server-owned legal-action generation for check, call, fold, bet, and raise
- betting-round progression across preflop, flop, turn, river, and showdown
- fold settlement, showdown resolution, split-pot handling, and the heads-up odd-chip policy
- table-driven tests for the rules behaviors called out in the repository instructions

## Normative sources

The code is intentionally anchored to the repository docs rather than ad hoc poker logic:
- [`../domain/texas-holdem.md`](../domain/texas-holdem.md)
- [`../domain/heads-up-nlhe.md`](../domain/heads-up-nlhe.md)
- [`../research.md`](../research.md) for current match framing and research constraints

Package docs in `internal/deck` and `internal/rules` point back to those sources.

## `internal/deck`

`internal/deck` is responsible only for deterministic card and dealing primitives.

Key details:
- `ParseCard` and `Card.String()` use two-character poker card notation such as `As` and `Td`.
- `Full()` returns a standard 52-card deck.
- `Dealer` derives each hand shuffle from `matchSeed` plus `handNumber`, so the same match seed reproduces the same deal sequence across strategy matchups.
- The shuffle logic uses a local SplitMix64-backed Fisher-Yates flow instead of depending on Go stdlib RNG stability.
- `DealHoldemHand` returns per-seat hole cards, burn cards, the full board, the remaining stub, and the full deck order for replay/debugging.

## `internal/rules`

`internal/rules` currently models a two-player v0 cash-game hand lifecycle.

### Match and hand state

- `NewHeadsUpMatch` creates a v0 match state with two seats and configured starting stacks/blinds.
- `MatchState.StartHand`:
  - rotates the button every hand
  - posts small and big blinds
  - auto-rebuys busted players at hand start, per the repository's cash-game model
  - loads deterministic hole cards and the precomputed full board from the dealer
- `MatchState.FinalizeHand` persists end-of-hand stacks back into match state once the hand is complete and showdown is resolved if needed.

### Betting behavior

`HandState` owns the in-hand state machine.

Implemented behavior includes:
- legal-action generation from server state, not agent claims
- correct heads-up preflop turn order with the small blind acting first preflop
- big blind first to act on later streets
- no-limit bet/raise ranges with explicit min/max totals
- short all-in calls and raises
- non-reopening short all-in raise handling via `raiseOptionOpened`
- automatic street advancement when betting rounds close
- automatic showdown transition when no further action is possible

### Settlement behavior

The engine resolves both major hand endings:
- **folds**: uncontested pot settlement with unmatched-chip refund before awarding the pot
- **showdowns**: best-hand evaluation from hole cards plus board, winner selection, split-pot handling, and odd-chip award to the first tied seat clockwise from the button

For heads-up ties, that odd chip goes to the big blind, matching the implemented project policy.

### Hand evaluation

`ResolveShowdown` currently supports representative five-card category comparison over seven available cards:
- high card
- one pair
- two pair
- three of a kind
- straight
- flush
- full house
- four of a kind
- straight flush

Results are captured in `HandResult`, including winning seats, per-seat chip deltas, showdown hands, and odd-chip metadata.

## Test coverage

EPIC-1 added table-driven automated coverage for:
- deterministic deck behavior and reproducible dealing
- parse/string round-trips for cards
- blind rotation and button/sb/bb assignment across hands
- auto-rebuy at hand start
- preflop and postflop legal-action generation
- short all-in raise behavior and raise reopening constraints
- betting-round progression through all streets
- contested and uncontested pot accounting
- representative hand-evaluation ordering and tiebreak cases
- showdown winner resolution, split pots, and odd-chip assignment

This gives the repository a pure-Go validation surface before protocol and server work begin.

## Current boundaries

This foundation is intentionally still v0-scoped.

Not implemented here:
- side pots
- multiplayer / 6-max table logic
- wire protocol types
- server process orchestration
- session artifact writing
- LLM or AKG integration

Those belong to later subsystem work and should be grounded in the focused docs for that layer.

## Why this matters for later work

Future protocol, server, and agent tasks can treat the rules engine as the server-authoritative source for:
- legal actions
- stack and pot accounting
- blind/button rotation
- deterministic replay from seed + hand number
- final hand outcomes

That separation is important because the server must never trust agent-reported game state.
