# Agents Playing Poker — Research Log

**Authors:** Robert Gumeny  
**Repo:** [github.com/RobertGumeny/agent-poker](https://github.com/RobertGumeny/agent-poker)  
**Status:** Active research

---

## Thesis

Can structured, retrievable memory make an LLM agent measurably better at poker?

The specific claim:

> Given the same model, same game rules, same tools, and the same deal sequence, an LLM agent backed by a structured [AKG](https://github.com/RobertGumeny/akg) knowledge graph will outperform an agent that dumps the full hand history into its context — and both will outperform an agent with no memory at all.

Poker is a good thesis vehicle because it demands two things that memory directly addresses: **opponent modeling** (what does this player tend to do?) and **pattern recognition** (what has happened in hands like this one?). It also produces a clean, objective metric — chip delta — that doesn't require human judgment to interpret.

---

## The Setup

### Game format

Heads-up (2-player) no-limit Texas Hold'em, cash-game model with auto-rebuy.

- **Starting stack:** 200bb
- **Blinds:** 1/2
- **Information realism:** `showdown-only` — each agent sees the opponent's hole cards only at showdown, mirroring real poker
- **Match seed:** deterministic — the same seed produces the same deal sequence regardless of which strategies are playing, so any chip-delta difference is purely a function of decision quality

### Measuring results

Canonical metric: **mean chips per hand** with 95% CI. Reported per session in `sessions/<id>/manifest.json` and derivable from `hands.jsonl`.

A match is N hands between two agents. The deal sequence is fixed by seed; each agent plays both sides of the same distribution across mirror runs to cancel positional variance.

---

## Agents

All three LLM agents use the same model and temperature. They differ only in what they inject into the decision prompt.

### `llm-stateless`

No memory. Each decision sees only the current hand state: hole cards, board, pot, stacks, action history for this street. Nothing carries across hands.

This is the floor. Any memory strategy should beat it over enough hands, because it cannot learn anything about the opponent across the match.

### `llm-fullhistory`

Naive memory baseline. Before each decision, the full history of completed hands is serialized as compact human-readable summary lines and injected into the prompt. The list grows with every hand.

Example prompt injection after hand 12:
```
Prior hands:
hand=1 | hero_pos=sb/button | hero_hole=8c Qc | board=- | actions=preflop:hero raise 6, villain fold | showdown=no | revealed=none | hero_result=+2
hand=2 | hero_pos=bb | hero_hole=Kd 9h | board=Ah 7c 2d | ...
...
```

This gives the LLM access to everything, but the prompt grows without bound and mixes relevant and irrelevant history indiscriminately. Context eventually fills up, and the model gets no help prioritizing what actually matters.

### `llm-akg`

Structured memory via [AKG](https://github.com/RobertGumeny/akg). Instead of replaying raw history, the agent maintains a binary knowledge graph file (`memory.akg`) that persists across the match in the server-provided `memory_dir`. Before each decision, it retrieves two things:

1. **An opponent profile** — a singleton node updated after every hand with running behavioral statistics and a generated prose summary
2. **Recent hand nodes** — the last 5 completed hands, stored compactly and retrieved by recency

The graph is written by the agent's `afterHandEnd` hook — not by the LLM itself. The LLM only reads the retrieved context injected into its prompt.

This is the thesis agent. It should beat `llm-fullhistory` because:
- The opponent profile is structured and cumulative, not a raw log
- Retrieval is bounded and deterministic — the prompt size does not grow with match length
- The signal-to-noise ratio in the injected context is much higher

---

## AKG Schema

The `llm-akg` agent uses two node types. No edges in v0.

### `opponent` node (singleton)

One node per match, upserted after every hand. Tracks behavioral statistics and a generated prose summary of what's been observed.

```
type:  "opponent"
id:    "villain"
title: <villain agent name>
body:  "47 hands played. VPIP 49% (23/47), PFR 38% (18/47).
        Folded to hero bet/raise 4 times (9% of hands).
        Aggression: 12 aggressive streets total.
        Showdown: 2/5 won (40%)."
meta: {
  hands_played:   47,
  vpip:           23,   // voluntarily put $ in preflop (calls/bets/raises)
  pfr:            18,   // preflop raises
  fold_to_bet:     4,   // times villain folded to a hero bet or raise
  aggr_streets:   12,   // streets where villain bet or raised
  showdown_count:  5,
  showdown_win:    2,
}
tags: ["opponent"]
```

The `body` is generated programmatically from the counters — not by an LLM. The playing LLM reads the body and meta, reasons about what they imply, and acts accordingly.

### `hand` node (one per completed hand)

Written once, never mutated. Captures the essential facts of the hand in a single-line body.

```
type:  "hand"
id:    <generated>
title: "Hand 47"
body:  "Hand 47: hero sb, hole [Ah Kd], board [Qc 7h 2s Jd 5c].
        preflop: villain raise 6, hero call. flop: hero check,
        villain bet 120, hero fold. Reached flop. Net: -165."
meta: {
  hand_number:    47,
  hero_position:  "sb",   // "sb" | "bb"
  hero_net:       -165,
  street_reached: "flop", // preflop | flop | turn | river | showdown
  showdown:       false,
}
tags: ["hand"]
```

### What the LLM sees before each decision

```
Opponent profile:
47 hands played. VPIP 49% (23/47), PFR 38% (18/47). Folded to hero
bet/raise 4 times (9% of hands). Aggression: 12 aggressive streets total.
Showdown: 2/5 won (40%).
Stats: hands=47, vpip=23, pfr=18, fold_to_bet=4, aggr_streets=12, showdown=2/5

Recent hands (last 5):
Hand 43: hero bb, hole [Tc 9c], board [Ah Kd 2c]. preflop: villain raise 6, hero call. flop: hero check, villain bet 40, hero fold. Reached flop. Net: -46.
Hand 44: ...
...
```

Compare to `llm-fullhistory`, which injects all 47 prior hands as raw pipe-delimited lines.

---

## Experiment Design

### Matchup matrix

| Matchup | What it measures |
|---|---|
| `llm-akg` vs `llm-fullhistory` | The headline — does structured retrieval beat naive history? |
| `llm-akg` vs `llm-stateless` | Does AKG memory beat no memory? |
| `llm-fullhistory` vs `llm-stateless` | Does naive memory help at all? (sets up AKG as top) |

### Parameters

All research sessions use these unless noted:

| Parameter | Value |
|---|---|
| Hands per match | 200 |
| Starting stack | 200bb |
| Blinds | 1/2 |
| Seed | 1 (fixed across all sessions for same deal sequence) |
| Info realism | `showdown-only` |

### Running a session

```bash
cd pi-agents && npm run build && cd ..
./poker-run -agent0 llm-akg -agent1 llm-fullhistory -hands 200 -model <provider:model>
```

Results land in `sessions/<session-id>/`. The AKG memory file for each agent is at `sessions/<id>/agents/<name>/memory.akg`.

---

## Results

*Sessions are appended here as they complete.*

| Session | Agent 0 | Agent 1 | Hands | Seed | Agent 0 Δ | Agent 1 Δ | chips/hand (0) | Notes |
|---|---|---|---|---|---|---|---|---|
| — | — | — | — | — | — | — | — | No sessions run yet |

---

## Observations & Adjustments

*Dated entries. What we saw, what we changed, and why.*

---

*Research begins here.*
