---
title: Agents Playing Poker — v0 MVP Spec
status: draft
author: Robert Gumeny
date: 2026-05-21
---

# Agents Playing Poker — v0 MVP Spec

## 1. Overview

A research harness in which multiple agents play heads-up no-limit Texas Hold'em against each other under identical rules and tools, differing only in their **memory strategy**. The goal is to produce a measurable, inspectable demonstration that durable structured memory (via AKG) gives an LLM agent a competitive advantage over (a) no memory and (b) the naive "stuff history into the prompt" approach.

v0 is single-machine, single-operator, no public-facing tournament. Future versions add multiplayer / BYO-SDK / leaderboard, but those are explicitly out of scope here.

## 2. Thesis (what v0 must prove)

The headline experimental claim:

> Given the same model, the same game rules, the same tools, and the same hand distribution, an LLM agent with AKG-backed structured memory and a deliberately minimal in-session context will outperform an LLM agent with the full hand history dumped into its context — and both will outperform the no-memory baseline.

If this comparison produces a clean chart, v0 has done its job. Everything in this spec serves that.

## 3. Non-goals

- Real money, peer-to-peer wagers, or anything that resembles regulated gambling.
- Public multiplayer, BYO-SDK entry, leaderboards, spectator UIs.
- Running arbitrary third-party agent code in our process (agents are our own for v0).
- 6-max / full-ring poker (extension point preserved; not built).
- Perfect-play / GTO solving. We don't need to *solve* poker; we need to *measure* memory.
- A general-purpose poker engine library. We build the minimum rules engine to play heads-up.

## 4. System architecture

```
                  ┌───────────────────────────────┐
                  │       Game Server (Go)        │
                  │                               │
                  │  ┌───────────────────────┐    │
                  │  │ Rules engine          │    │
                  │  │ Match orchestrator    │    │
                  │  │ Seeded RNG / dealer   │    │
                  │  │ Session logger        │    │
                  │  └──────────┬────────────┘    │
                  │             │                 │
                  │             │ wire protocol   │
                  │             │ (JSON lines     │
                  │             │  over stdio)    │
                  └─────────────┼─────────────────┘
                                │
            ┌───────────────────┼───────────────────┐
            │                   │                   │
   ┌────────▼────────┐ ┌────────▼────────┐ ┌────────▼────────┐
   │  Agent A        │ │  Agent B        │ │  Agent C        │
   │  (Pi/TS LLM)    │ │  (Pi/TS LLM)    │ │  (Go heuristic) │
   │                 │ │                 │ │                 │
   │  AKG SDK (TS)   │ │  AKG SDK (TS)   │ │  AKG SDK (Go)   │
   │  ↓              │ │  ↓              │ │  ↓              │
   │  memory.akg     │ │  memory.akg     │ │  memory.akg     │
   │  pi-session.    │ │  pi-session.    │ │  (no Pi log)    │
   │    jsonl        │ │    jsonl        │ │                 │
   └─────────────────┘ └─────────────────┘ └─────────────────┘
```

- **Game server** owns rules, RNG, match orchestration, and per-session logging. Written in Go.
- **Agents** are independent processes the server spawns. Each owns its working directory and its `.akg` file. Communication is JSON lines over the agent's stdin/stdout — the server writes prompts, the agent writes actions back.
- **LLM agents** run inside the Pi harness; they get Pi's session log, compaction, and tool system for free.
- **Heuristic / scripted agents** are plain Go binaries that speak the same wire protocol — no Pi involvement.

## 5. Repo layout (proposed)

```
agent-poker/
├── README.md
├── go.mod
├── cmd/
│   ├── poker-server/         # the game server binary
│   └── agents/
│       ├── random/           # Go agent: always-call / random
│       └── heuristic/        # Go agent: scripted hand-strength + pot-odds
├── internal/
│   ├── rules/                # heads-up NLHE rules, hand eval, betting rounds
│   ├── deck/                 # seeded deal sequence
│   ├── match/                # match orchestrator, pairing logic
│   ├── wire/                 # JSON wire protocol types
│   └── sessionlog/           # hands.jsonl, manifest.json writers
├── pi-agents/
│   ├── llm-nomemory/         # Pi agent: LLM, no memory, just current state
│   ├── llm-fullhistory/      # Pi agent: LLM, full history dumped into context
│   └── llm-akg/              # Pi agent: LLM + AKG-backed structured memory
├── docs/
│   ├── wire-protocol.md
│   ├── strategies.md
│   └── experiment-design.md
└── sessions/                 # gitignored — output of runs
```

The Go AKG SDK lives in the `akg` repo and is imported as a Go module. The TS AKG SDK lives in its own repo (or under `akg/sdks/typescript`, TBD) and is consumed via npm/pnpm.

## 6. Game model

### 6.1 Variant

Heads-up (2-player) no-limit Texas Hold'em. Standard betting structure: preflop, flop, turn, river, with small blind and big blind alternating each hand.

### 6.2 Match structure

A **match** is `N` hands between two specific agents. `N` is configurable; default 200.

- **Cash-game model with auto-rebuy.** Stacks carry across hands. Starting stack at match start is configurable (default 200 big blinds). If an agent busts during a hand, at the start of the next hand they automatically rebuy to the starting stack. This preserves the full range of stack-depth strategy (short-stack push/fold, mid-stack maneuvering, deep-stack play) while ensuring all N hands play out regardless of variance. It is how online cash games actually work.
- **Blinds rotate every hand** so each agent plays each position equally.
- **Metric is total chip delta across all N hands.** Rebuys are notional draws from each agent's bankroll, not free chips: the chips one agent wins came from the other agent's stack at the moment of the hand. Sum of chip deltas across hands is the canonical match outcome. Mean chips-per-hand (with CI) is the canonical comparative number across matches.
- The match's seed deterministically produces the deal sequence for all N hands. The same seed produces the same deal regardless of which strategies are pitted against each other.

### 6.3 Information realism

Configurable per match, defaulting to **showdown-only**:

- **`showdown-only` (default)**: agent only learns opponent's hole cards when a hand reaches showdown. Mirrors real poker. The intended setting for headline experiments.
- **`perfect-info`**: agent sees opponent's hole cards every hand. For debugging memory strategies in isolation; not used for headline numbers.

### 6.4 Extension points for 6-max

v0 only builds heads-up, but the rules engine and wire protocol must not assume two players. Specifically:
- The `your_turn` message includes a list of all seated players, their stacks, and the action history of the current hand.
- Side-pot mechanics are not implemented in v0 (heads-up doesn't need them); the data model leaves room for them.
- Match orchestration is "list of seats" not "player A vs player B".

This costs little in v0 and saves a rewrite for 6-max.

## 7. Wire protocol

All messages are JSON, one object per line, over the agent process's stdin/stdout. The server writes to the agent's stdin; the agent writes to its stdout. Stderr is captured to the agent's session dir as a debug log.

For Pi-based agents, we use Pi's existing RPC mode (stdin/stdout JSONL). For Go agents, we implement the same protocol directly.

### 7.1 Message envelope

```json
{ "v": 1, "type": "<message_type>", "id": "<uuid>", "payload": { ... } }
```

`id` is a monotonic per-session message ID for traceability. Responses reference the originating `id`.

### 7.2 Server → agent messages

#### `session_init`
Sent once at agent startup.
```json
{
  "type": "session_init",
  "payload": {
    "session_id": "ses_2026-05-21_001",
    "agent_name": "llm-akg",
    "match": {
      "match_id": "mat_001",
      "seed": 12345,
      "hand_count": 200,
      "variant": "heads-up-nlhe",
      "info_realism": "showdown-only",
      "starting_stack": 200,
      "blinds": {"sb": 1, "bb": 2}
    },
    "seats": [
      {"seat": 0, "name": "llm-akg"},
      {"seat": 1, "name": "llm-fullhistory"}
    ],
    "your_seat": 0,
    "memory_dir": "/abs/path/to/agent/dir"
  }
}
```
Agent acknowledges with `session_ready`.

#### `hand_start`
```json
{
  "type": "hand_start",
  "payload": {
    "hand_number": 47,
    "dealer_seat": 1,
    "stacks": {"0": 200, "1": 200},
    "blinds_posted": [{"seat": 0, "amount": 1}, {"seat": 1, "amount": 2}],
    "your_hole_cards": ["As", "Kh"]
  }
}
```

#### `your_turn`
```json
{
  "type": "your_turn",
  "payload": {
    "hand_number": 47,
    "street": "flop",
    "board": ["Td", "9h", "2c"],
    "pot": 6,
    "to_call": 2,
    "stacks": {"0": 197, "1": 197},
    "action_history": [
      {"seat": 1, "action": "call", "amount": 1, "street": "preflop"},
      {"seat": 0, "action": "check", "street": "preflop"},
      {"seat": 0, "action": "bet", "amount": 2, "street": "flop"}
    ],
    "legal_actions": [
      {"action": "fold"},
      {"action": "call", "amount": 2},
      {"action": "raise", "min": 4, "max": 197}
    ],
    "decision_deadline_ms": 30000
  }
}
```

#### `hand_end`
```json
{
  "type": "hand_end",
  "payload": {
    "hand_number": 47,
    "board": ["Td", "9h", "2c", "5s", "Kc"],
    "showdown": {
      "0": {"hole_cards": ["As", "Kh"], "rank": "two pair, kings and tens"},
      "1": {"hole_cards": ["9s", "9d"], "rank": "three of a kind, nines"}
    },
    "result": [{"seat": 1, "chips_delta": 14}, {"seat": 0, "chips_delta": -14}]
  }
}
```
If the hand did not reach showdown, `showdown` contains only the winner's cards by convention (real poker: cards aren't mucked face-up unless the player chooses to show). For `info_realism: perfect-info`, both hole cards are always included.

#### `session_end`
Final message; gives the agent a chance to flush memory writes and exit cleanly.

### 7.3 Agent → server messages

#### `session_ready`
Acknowledges `session_init`. Includes agent version metadata for the manifest.

#### `action`
Response to `your_turn`. Must reference the originating message `id`.
```json
{
  "type": "action",
  "in_reply_to": "<your_turn id>",
  "payload": {"action": "call", "amount": 2}
}
```

#### `log` (optional)
Free-form structured logging the agent wants captured server-side. Optional; Pi already logs to its own session file.

### 7.4 Timeouts and misbehavior

- Decisions must arrive within `decision_deadline_ms` (default 30s, configurable per match). On timeout the server records `auto_fold` and proceeds.
- If the agent process dies mid-match, the match is aborted and marked `incomplete` in the manifest; the partial results are still persisted.
- The server never trusts the agent for state; it computes pot, legal actions, and showdown winners itself.

## 8. Agent lifecycle

One **Pi session per match per agent**. The agent is spawned at match start, receives `session_init`, then drives through all N hands of the match in a single long-lived process. At match end it receives `session_end` and exits.

The Pi session model handles working memory; AKG handles durable memory. The AKG-aware compaction extension (see §10) is what keeps the working window small.

For non-Pi agents (Go heuristic, random), the same lifecycle applies but the process is just a Go binary in a `for { read msg; respond }` loop.

## 9. Strategy lineup for v0

v0 ships five strategies, each as its own binary / Pi project:

| Strategy | Process | Memory | Purpose |
|---|---|---|---|
| `random` | Go | none | Sanity floor. If anything loses to this, something is broken. |
| `heuristic` | Go | none (stateless) | Scripted hand-strength + pot-odds. The "reasonable non-LLM" baseline. |
| `llm-nomemory` | Pi/TS | none | LLM agent that sees only current hand state. Control for "does memory help at all." |
| `llm-fullhistory` | Pi/TS | full hand history dumped into context each turn | The "naive RAG / just stuff it in the prompt" comparison. |
| `llm-akg` | Pi/TS | AKG-backed structured memory + AKG-aware compaction | **The thesis.** Should beat `llm-fullhistory` on equivalent or smaller context budget. |

All three LLM strategies use the **same model, same temperature, same base system prompt**. The only differences are: what's in the context window, and what tools the agent has.

## 10. SDK helper surface (v0)

The Go and TypeScript SDKs both provide the same minimal helper surface above AKG core. Helpers are language-idiomatic but conceptually identical.

### 10.1 Shared contract

```
Open(path)                              -> Store
Close(store)
Commit(store)

PutNode(typeName, id, fields, tags)     -> NodeRef
GetNode(typeName, id)                   -> Node | null
ListNodesByTag(tag)                     -> Node[]

PutEdge(fromRef, relation, toRef, fields)
OutboundEdges(nodeRef, relation?)       -> Edge[]
InboundEdges(nodeRef, relation?)        -> Edge[]
```

That's it. No traversal language, no query planner, no ingestion DSL. Those are explicitly above-SDK concerns and out of scope for v0.

These map onto AKG core's existing derived keys (`t:`, `e:`, `ei:`) — the SDK is a thin readable wrapper, not a new layer of semantics.

### 10.2 Go SDK

Lives in the `akg` repo (probably `github.com/RobertGumeny/akg-format/sdk` or a sibling module). Consumed by:
- the Go heuristic agent (which currently doesn't use memory but should still link the SDK to keep the door open)
- doug (eventually)
- future Go agents

### 10.3 TypeScript SDK

Lives in its own repo (or `akg/sdks/typescript`). Consumed by:
- all Pi-based poker agents
- doug's Pi child sessions (eventually)

**Crucially: the TS SDK reimplements AKG core in TypeScript**, validated against the conformance corpus. This is the spec-hardening pass before AKG v1 ships. The TS SDK is "done" when (a) the conformance corpus passes and (b) poker agents can read/write `.akg` files through it.

Any ambiguities found during TS implementation get folded back into the AKG spec before v1.

## 11. AKG-aware Pi compaction extension

Provided by the TS SDK as `@akg/sdk/pi-extension` (name TBD).

### 11.1 Mechanism

1. AKG write tools (`akg_put_node`, `akg_put_edge`) return a stable reference (e.g., `{type: "Hand", id: "h_47"}`) in their tool result.
2. The extension registers a `session_before_compact` hook in the Pi session.
3. When compaction fires, the hook:
   - Scans messages about to be compacted
   - Identifies AKG write tool calls and their returned references
   - Returns a custom `CompactionEntry` whose summary replaces verbose observations with AKG references:
     > *"Observations during hands 23–47 written to AKG. Query `tag:opponent_behavior, hand:23-47` to retrieve. Recent context: [tight 1-paragraph summary]."*
4. Per-project `.pi/settings.json` ships aggressive `keepRecentTokens` so compaction kicks in early and the AKG-references summary dominates.

### 11.2 Why this matters for the experiment

This extension is what makes the headline comparison fair. Without it, `llm-akg` would still be a meaningful test but would lose the "smaller working context" dimension. With it, we can claim:

> The AKG agent used X% of the context the full-history agent used, and won.

That's a quantitative claim the spec produces automatically — both agents' Pi session logs include token-count metadata, so the chart writes itself.

### 11.3 Reusable beyond poker

This extension belongs in the TS SDK because every AKG-backed Pi agent benefits from it, not just poker. Doug's Pi subagents will use the same extension.

## 12. Determinism & replay

- **Seed → deal sequence**: deterministic. Same seed reproduces the same cards dealt across all N hands, in the same order, with the same blind rotation.
- **LLM agents are not deterministic** (temperature, model nondeterminism). Replay reproduces the *game state* exactly but not the *agent decisions*.
- **Scripted agents (random, heuristic) are deterministic given seed** — random uses the same seeded RNG, heuristic is pure function of state.
- **Replay tool** (v0.1, post-MVP): given a `manifest.json` + `hands.jsonl`, re-run the deal sequence against the same or different agent lineup. Useful for "what would `llm-akg` have done on this seed?" analysis.

## 13. Session output format

```
sessions/
└── ses_2026-05-21_001/
    ├── manifest.json
    ├── hands.jsonl
    └── agents/
        ├── llm-akg/
        │   ├── memory.akg
        │   ├── pi-session.jsonl
        │   ├── stdout.log
        │   └── stderr.log
        └── llm-fullhistory/
            ├── memory.akg            # exists but trivial / empty
            ├── pi-session.jsonl
            ├── stdout.log
            └── stderr.log
```

### `manifest.json`

```json
{
  "session_id": "ses_2026-05-21_001",
  "started_at": "2026-05-21T18:32:11Z",
  "ended_at": "2026-05-21T19:04:42Z",
  "seed": 12345,
  "hand_count": 200,
  "variant": "heads-up-nlhe",
  "info_realism": "showdown-only",
  "starting_stack": 200,
  "blinds": {"sb": 1, "bb": 2},
  "matches": [
    {
      "match_id": "mat_001",
      "seats": [
        {"seat": 0, "name": "llm-akg", "version": "..."},
        {"seat": 1, "name": "llm-fullhistory", "version": "..."}
      ],
      "result": {"0": {"chips_delta": 412}, "1": {"chips_delta": -412}},
      "completed": true
    }
  ],
  "server_version": "<git sha>",
  "akg_spec_version": "v1-draft-2"
}
```

### `hands.jsonl`

One JSON object per line, one line per hand played in the session. Schema includes: `match_id`, `hand_number`, `dealer_seat`, `stacks_start`, `blinds_posted`, `hole_cards` (both seats — this is the server's record, not what each agent saw), `board`, `actions` (full ordered action log), `showdown_reached` (bool), `result`.

### `agents/<name>/memory.akg`

The agent's final `.akg` file. WAL intact (no compaction triggered during session) so the full mutation history is recoverable for analysis.

### `agents/<name>/pi-session.jsonl`

Pi's native session log. Contains everything Pi already tracks: messages, thinking, tool calls, tool results, compaction events, token counts. No additional work for us.

## 14. Experimental matrix for the v0 launch

The first "real" run that produces the v0 chart is a round-robin tournament:

| | random | heuristic | llm-nomem | llm-fullhist | llm-akg |
|---|---|---|---|---|---|
| **random** | — | • | • | • | • |
| **heuristic** | • | — | • | • | • |
| **llm-nomem** | • | • | — | • | • |
| **llm-fullhist** | • | • | • | — | • |
| **llm-akg** | • | • | • | • | — |

- Each pairing plays K matches (default K=3) with different seeds.
- Each match is N=200 hands.
- Total hands per pairing: 600.
- The metric per pairing is mean chips-per-hand delta with 95% CI.

The headline number is the `llm-akg` vs `llm-fullhistory` cell, and the context-tokens-used comparison from Pi's session logs.

A second analysis worth doing: walk each `llm-akg/memory.akg` file post-tournament and show concrete examples of what the agent wrote about opponents and how it queried that during play. Inspectability is half the demo.

## 15. Out of scope for v0

- Multiplayer / public access / BYO-SDK entry
- Authentication, accounts, persistent leaderboards
- 6-max or full-ring poker (extension points preserved)
- Side pots (heads-up doesn't need them)
- Spectator UI (a CLI replay tool is the v0.1 ambition)
- Sandboxing / running untrusted agent code
- LLM provider abstraction beyond config (start with one model, expand later)
- Tournament chip-tracking / bust-out mechanics
- Hand-history import from PokerStars / similar (no reason)
- Ingestion DSL or query language in the SDK

## 16. Open questions

- **TS SDK repo location**: standalone repo, or `akg/sdks/typescript`? Punted until we start building.
- **Pi compaction extension distribution**: published to npm as a sibling of the TS SDK, or bundled inside it? Lean toward bundled for v0; split later if other consumers want only the extension.
- **Decision deadline default**: 30s is generous for LLM agents on flop+ decisions; might need to be larger for thinking models. Reassess after first end-to-end run.
- **Match concurrency**: v0 runs matches sequentially. Parallelizing across matches would 5x throughput on the round-robin but introduces process / model-quota contention; defer.
- **Cost ceiling**: a 600-hand pairing with two LLM agents could be expensive. Budget gate on the runner so a tournament can't accidentally burn $500.

## 17. Build phasing (rough)

A defensible order of operations, roughly two weeks if nothing rabbit-holes:

1. **Rules engine** (Go): heads-up NLHE betting rounds, hand evaluator, deck dealer. Pure functions. Unit tests against well-known hand-ranking corner cases.
2. **Wire protocol** (Go types + JSON schemas + docs).
3. **Game server + match orchestrator** (Go): spawns agent processes, runs the message loop, writes `hands.jsonl` and `manifest.json`.
4. **Random + heuristic agents** (Go): proves the wire protocol end-to-end against trivial opponents. Should produce a working session at this point.
5. **Go AKG SDK** (in `akg` repo): the small helper surface from §10.
6. **TS AKG SDK + conformance corpus passing**: the big one. Doubles as spec hardening.
7. **`llm-nomemory` Pi agent**: simplest LLM player. Validates Pi + wire protocol.
8. **`llm-fullhistory` Pi agent**: adds the naive history-in-context strategy.
9. **AKG Pi compaction extension** (in TS SDK).
10. **`llm-akg` Pi agent**: the thesis. End-to-end.
11. **First round-robin tournament + chart**.

Steps 1–4 stand on their own — you have a working "agents play poker against each other" demo with no AKG, no LLMs, no Pi. Everything after that adds the research dimension. That ordering lets you ship something visible early and adds complexity in a way each step is independently de-risked.
