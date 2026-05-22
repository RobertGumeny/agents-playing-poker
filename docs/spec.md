---
title: Agents Playing Poker — v0 MVP Spec
status: draft
author: Robert Gumeny
date: 2026-05-21
---

# Agents Playing Poker — v0 MVP Spec

## 1. Thesis

A research harness in which multiple agents play heads-up no-limit Texas Hold'em against each other under identical rules and tools, differing only in their **memory strategy**. The experimental claim:

> Given the same model, same game rules, same tools, and same hand distribution, an LLM agent with AKG-backed structured memory and a deliberately minimal in-session context will outperform an LLM agent with the full hand history dumped into its context — and both will outperform the no-memory baseline.

v0 is single-machine, single-operator. Everything in this spec serves that comparison.

## 2. Non-goals

- Real money, wagers, or anything resembling regulated gambling
- Public multiplayer, BYO-SDK entry, leaderboards, spectator UI
- Running arbitrary third-party agent code
- 6-max or full-ring poker (extension points preserved, not built)
- Side pots (heads-up doesn't need them)
- Perfect-play / GTO solving
- A general-purpose poker engine library
- Ingestion DSL or query language in the SDK

## 3. What we build in this repo

- **Rules engine** (Go): heads-up NLHE hand evaluation, betting rounds, deck dealer
- **Game server + match orchestrator** (Go): spawns agent processes, runs the message loop, writes session output
- **Wire protocol**: JSON-lines over stdio — the contract between server and agents
- **Go agents**: `random` and `heuristic` (scripted, no LLMs, no AKG)
- **Pi agents**: `llm-nomemory`, `llm-fullhistory`, `llm-akg` (TypeScript, live inside Pi sessions)
- **AKG-aware Pi compaction extension**: lives here for v0; hooks into Pi's `session_before_compact` to replace verbose hand observations with compact AKG references

## 4. Dependencies

- **AKG Go SDK** — imported as a Go module from the `akg` repo; used by Go agents
- **AKG TypeScript SDK** — consumed via npm; used by Pi agents. Location and packaging decided in the AKG repo.
- **Pi harness** — Pi's RPC mode (stdin/stdout JSONL) drives the LLM agents; Pi session logs provide token-count metadata automatically
- **AKG compaction extension** — likely its own Pi package; details TBD in the AKG repo. For v0 it lives here.

## 5. Game model

### Variant

Heads-up (2-player) no-limit Texas Hold'em. Standard betting: preflop, flop, turn, river, with small blind / big blind alternating each hand.

### Match structure

A **match** is N hands between two agents (default N=200). Cash-game model with auto-rebuy: stacks carry across hands, starting stack configurable (default 200bb). A busted agent rebuys to starting stack at the next hand start. Blinds rotate every hand. Metric is total chip delta across all N hands; mean chips-per-hand with 95% CI is the canonical comparison number.

The match seed deterministically produces the deal sequence. The same seed produces the same cards regardless of which strategies are playing.

### Information realism

Configurable per match, default `showdown-only`:

- **`showdown-only`** (default): agent learns opponent's hole cards only at showdown. Mirrors real poker.
- **`perfect-info`**: agent sees opponent's hole cards every hand. For debugging memory strategies in isolation.

### Odd-chip policy

This repo uses integer chip units. If a pot must be split and cannot be divided evenly, the single odd chip is awarded to the first tied seat clockwise from the button.

For heads-up play, that means the odd chip goes to:

- the **big blind** when both players tie,
- because the big blind is the first seat clockwise from the button.

This is a project policy for deterministic settlement. It overrides the generic domain docs' intentional omission of house-rule-dependent odd-chip behavior.

### Extension points

The rules engine and wire protocol must not hard-assume two players. `your_turn` includes a list of all seated players; the data model leaves room for side pots. This costs little now and avoids a rewrite for 6-max later.

## 6. Wire protocol

All messages are JSON, one object per line, over the agent process's stdin/stdout. Server writes to agent stdin; agent writes to stdout. Stderr is captured to the session dir as a debug log.

### Message envelope

```json
{ "v": 1, "type": "<message_type>", "id": "<uuid>", "payload": { ... } }
```

`id` is a monotonic per-session message ID. Responses reference the originating `id`.

### Server → agent

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
      "blinds": {"sb": 1, "bb": 2},
      "decision_deadline_ms": 30000
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
    ]
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
If no showdown, `showdown` contains only the winner's cards. For `perfect-info`, both hole cards are always included.

#### `session_end`
Final message; gives the agent a chance to flush memory writes and exit cleanly.

### Agent → server

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
Free-form structured logging the agent wants captured server-side.

### Timeouts and misbehavior

- Decisions must arrive within `decision_deadline_ms` (configurable per match, default 30s). On timeout the server records `auto_fold` and proceeds.
- If the agent process dies mid-match, the match is aborted and marked `incomplete`; partial results are still persisted.
- The server never trusts the agent for state — it computes pot, legal actions, and showdown winners itself.

## 7. Agent lifecycle

One Pi session per match per agent. The agent is spawned at match start, receives `session_init`, then drives through all N hands in a single long-lived process, and exits on `session_end`.

Go agents (random, heuristic) follow the same lifecycle in a plain `for { read; respond }` loop.

## 8. Strategy lineup

| Strategy | Process | Memory | Purpose |
|---|---|---|---|
| `random` | Go | none | Sanity floor |
| `heuristic` | Go | none | Scripted hand-strength + pot-odds baseline |
| `llm-nomemory` | Pi/TS | none | LLM with only current hand state |
| `llm-fullhistory` | Pi/TS | full hand history in context | Naive "stuff it in the prompt" comparison |
| `llm-akg` | Pi/TS | AKG-backed structured memory + compaction | **The thesis** |

All three LLM strategies use the same model, same temperature, same base system prompt. The only differences are what's in the context window and what tools the agent has.

## 9. AKG-aware Pi compaction extension

When compaction fires, the extension:

1. Scans messages about to be compacted
2. Identifies AKG write tool calls (e.g. `akg_put_node`, `akg_put_edge`) and their returned references (e.g. `{type: "Hand", id: "h_47"}`)
3. Replaces verbose observations with a compact summary pointing at AKG:
   > *"Observations during hands 23–47 written to AKG. Query `tag:opponent_behavior, hand:23-47` to retrieve. Recent context: [tight 1-paragraph summary]."*

Per-project `.pi/settings.json` sets aggressive `keepRecentTokens` so compaction kicks in early and the AKG-references summary dominates the working context.

This is what makes the headline comparison quantitative: both agents' Pi session logs include token-count metadata, so context-usage vs. win-rate writes itself into the chart.

## 10. Session output format

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
            ├── memory.akg
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

One JSON object per line, one per hand. Fields: `match_id`, `hand_number`, `dealer_seat`, `stacks_start`, `blinds_posted`, `hole_cards` (both seats — server's record, not what each agent saw), `board`, `actions`, `showdown_reached`, `result`.

### `agents/<name>/memory.akg`

The agent's final `.akg` file. WAL intact so the full mutation history is recoverable for analysis.

### `agents/<name>/pi-session.jsonl`

Pi's native session log: messages, thinking, tool calls, tool results, compaction events, token counts.

## 11. Experimental matrix

Round-robin tournament; each pairing plays K=3 matches with different seeds, N=200 hands each (600 hands per pairing).

| | random | heuristic | llm-nomem | llm-fullhist | llm-akg |
|---|---|---|---|---|---|
| **random** | — | • | • | • | • |
| **heuristic** | • | — | • | • | • |
| **llm-nomem** | • | • | — | • | • |
| **llm-fullhist** | • | • | • | — | • |
| **llm-akg** | • | • | • | • | — |

Metric per pairing: mean chips-per-hand delta with 95% CI. Headline number: `llm-akg` vs `llm-fullhistory` chip delta and context-tokens-used comparison.

The runner must have a **cost ceiling / budget gate** to prevent a tournament from accidentally burning unbounded API spend.

Matches run **sequentially** for v0.

## 12. Build order

1. **Rules engine** (Go): heads-up NLHE betting rounds, hand evaluator, deck dealer. Pure functions, table-driven unit tests.
2. **Wire protocol**: Go types, JSON schemas, `docs/wire-protocol.md`.
3. **Game server + match orchestrator**: spawns agents, runs message loop, writes `hands.jsonl` and `manifest.json`.
4. **`random` + `heuristic` agents**: proves wire protocol end-to-end. At this point you have a working "agents play poker" demo with no LLMs, no AKG.
5. **Go AKG SDK**: thin helper surface in the `akg` repo, consumed by Go agents.
6. **TS AKG SDK + conformance corpus**: the big one — also the AKG spec-hardening pass.
7. **`llm-nomemory` Pi agent**: simplest LLM player; validates Pi + wire protocol.
8. **`llm-fullhistory` Pi agent**: naive history-in-context strategy.
9. **AKG Pi compaction extension**.
10. **`llm-akg` Pi agent**: the thesis, end-to-end.
11. **First round-robin tournament + chart**.

Stop and check in after step 4 before proceeding to step 5+.
