# Agents Playing Poker — Research Log

**Authors:** Robert Gumeny  
**Repo:** [github.com/RobertGumeny/agent-poker](https://github.com/RobertGumeny/agent-poker)  
**Status:** Active research

---

## Thesis

Can structured, retrievable memory make an LLM agent measurably better at poker?

The specific claim:

> Given the same model, same game rules, same tools, and the same deal sequence, an LLM agent backed by a mature structured [AKG](https://github.com/RobertGumeny/akg) knowledge graph should match or outperform an agent that dumps the full hand history into its context, while using materially fewer tokens — and both memory strategies should outperform an agent with no memory at all.

The current AKG agent is intentionally treated as an intermediate baseline: `llm-akg` in the CLI is best understood as `llm-akg-recent`, a shallow bounded-memory strategy with recent hands plus a growing opponent profile. The expensive `llm-fullhistory` comparison is reserved for the next AKG strategy generation, where retrieval is more situation-aware and the graph carries more durable opponent modeling signal.

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

Two primary metrics:

- **Mean chips per hand** with 95% CI — total chip delta divided by hands played, reported per session in `sessions/<id>/manifest.json` and derivable from `hands.jsonl`. CIs must be non-overlapping to claim a result.
- **Chips per token** — total chip delta divided by total input tokens consumed across the match. This is the efficiency metric: `llm-fullhistory`'s token cost grows with every hand; bounded AKG strategies should stay comparatively flat. The gap widens as match length increases.

A match is N hands between two agents. The deal sequence is fixed by seed; each agent plays both sides of the same distribution across mirror runs to cancel positional variance.

---

## Agents

All LLM agents in a benchmark use the same model and temperature. They differ only in what they inject into the decision prompt.

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

### `llm-akg-recent` / current `llm-akg`

Structured memory via [AKG](https://github.com/RobertGumeny/akg), currently implemented under the CLI name `llm-akg`. Instead of replaying raw history, the agent maintains a binary knowledge graph file (`memory.akg`) that persists across the match in the server-provided `memory_dir`. Before each decision, it retrieves two things:

1. **An opponent profile** — a singleton node updated after every hand with running behavioral statistics and a generated prose summary
2. **Recent hand nodes** — the last 5 completed hands, stored compactly and retrieved by recency

The graph is written by the agent's `afterHandEnd` hook — not by the LLM itself. The LLM only reads the retrieved context injected into its prompt.

This is a phase-0 bounded-memory baseline, not the final thesis agent. It should be benchmarked primarily against `llm-stateless` to establish the cost and behavior of shallow durable memory before spending heavily on `llm-fullhistory` comparisons.

### Planned `llm-akg-durable`

The next AKG strategy should make memory retrieval situation-aware and materially graph-backed: opponent tendencies by street and position, bet-sizing patterns, showdown evidence, fold/call/raise tendencies in recurring spots, and retrieved observations keyed to the current decision context.

This is the intended `llm-fullhistory` challenger. It should beat or match `llm-fullhistory` because:
- The opponent model is structured and cumulative, not a raw log
- Retrieval is bounded and deterministic — the prompt size does not grow with match length
- The signal-to-noise ratio in the injected context is much higher
- The memory artifact is inspectable and can be evolved without rewriting the whole prompt strategy

---

## AKG Schema

The current `llm-akg-recent` strategy, implemented under the CLI name `llm-akg`, uses two node types. No edges in v0.

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

Compare to `llm-fullhistory`, which injects all 47 prior hands as raw pipe-delimited lines. The near-term research plan is to first benchmark this shallow bounded-memory strategy against `llm-stateless`, then evolve the AKG graph and retrieval policy before using `llm-fullhistory` as the expensive final comparison.

---

## Experiment Design

### Benchmark sequence

| Phase | Matchup | What it measures | Budget posture |
|---|---|---|---|
| 1 | `llm-akg-recent` / current `llm-akg` vs `llm-stateless` | Does shallow bounded AKG memory change behavior versus no memory, and what is its token profile? | Low-cost multi-seed baseline |
| 2 | `llm-fullhistory` vs `llm-stateless` | Does naive memory help at all, and how quickly does prompt cost grow? | Limited calibration; already expensive |
| 3 | planned `llm-akg-durable` vs `llm-fullhistory` | Final-boss comparison: can richer structured memory match or beat raw history with bounded context? | Spend only after the durable AKG strategy is implemented |

Each matchup is run as a mirror pair: agent0 and agent1 swap seats and replay the same deal sequence. Chip deltas are averaged across both runs to cancel positional variance (the small blind acts first preflop, which creates a structural edge that would otherwise contaminate the result).

Current research priority: run more seeds for current `llm-akg` against `llm-stateless`, keep `llm-fullhistory` spending controlled, then reserve the full `llm-fullhistory` benchmark for the next AKG memory strategy.

### Parameters

All research sessions use these unless noted:

| Parameter | Value |
|---|---|
| Hands per match | 200 (per mirror run, 400 total per matchup) |
| Starting stack | 200bb |
| Blinds | 1/2 |
| Seed | Varied across benchmark seeds; fixed within each a/b mirror pair for the same deal sequence |
| Info realism | `showdown-only` |
| Metrics | mean chips/hand (95% CI), chips/token |

### Running a session

Each matchup is two runs with seats swapped. Average the chip deltas across both to get the mirror-corrected result. Vary `-seed` and include the seed in the session id.

**Active low-cost phase: current `llm-akg` (`llm-akg-recent`) vs `llm-stateless`**
```bash
./poker-run -agent0 llm-akg -agent1 llm-stateless -hands 200 -seed 1 -model anthropic:claude-sonnet-4-6 -session-id akg-recent-vs-stateless-seed1-a
./poker-run -agent0 llm-stateless -agent1 llm-akg -hands 200 -seed 1 -model anthropic:claude-sonnet-4-6 -session-id akg-recent-vs-stateless-seed1-b
```

**Limited calibration: `llm-fullhistory` vs `llm-stateless`**
```bash
./poker-run -agent0 llm-fullhistory -agent1 llm-stateless -hands 200 -seed 1 -model anthropic:claude-sonnet-4-6 -session-id fullhistory-vs-stateless-seed1-a
./poker-run -agent0 llm-stateless -agent1 llm-fullhistory -hands 200 -seed 1 -model anthropic:claude-sonnet-4-6 -session-id fullhistory-vs-stateless-seed1-b
```

**Final-boss phase, after durable AKG strategy exists: `llm-akg-durable` vs `llm-fullhistory`**
```bash
./poker-run -agent0 llm-akg-durable -agent1 llm-fullhistory -hands 200 -seed 1 -model anthropic:claude-sonnet-4-6 -session-id akg-durable-vs-fullhistory-seed1-a
./poker-run -agent0 llm-fullhistory -agent1 llm-akg-durable -hands 200 -seed 1 -model anthropic:claude-sonnet-4-6 -session-id akg-durable-vs-fullhistory-seed1-b
```

Results land in `sessions/<session-id>/`. The AKG memory file for each AKG agent is at `sessions/<id>/agents/<name>/memory.akg`.

---

## Results

### Phase-0 exploratory baseline snapshot — 2026-05-26

These early 200-hand mirror pairs are exploratory only. Chip outcomes are dominated by seed, seat, and card assignment: seat 0 won all four completed sessions. Treat chip totals as a smoke-test signal, not evidence of strategic superiority. In this snapshot, `llm-akg` should be interpreted as `llm-akg-recent`: shallow bounded recency memory, not durable AKG-backed retrieval.

| Pair / agent of interest | Sessions | Hands | Agent chip Δ | Total tokens | Cost | Avg cost/hand | First-20 avg cost/hand | Last-20 avg cost/hand | Cost growth |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `llm-akg-recent` vs `llm-stateless` | `akg-vs-stateless-a`, `akg-vs-stateless-b` | 400 | +253 | 1,525,784 | $3.86 | $0.0097 | $0.0100 | $0.0118 | 1.2× |
| `llm-fullhistory` vs `llm-stateless` | `fullhistory-vs-stateless-a`, `fullhistory-vs-stateless-b` | 400 | +265 | 18,841,774 | $39.43 | $0.0986 | $0.0136 | $0.2277 | 16.7× |

Per-session usage for the memory-bearing agent:

| Agent/session | Decisions | Total tokens | Cost | Avg cost/hand | First-20 avg cost/hand | Last-20 avg cost/hand |
|---|---:|---:|---:|---:|---:|---:|
| `llm-akg-recent` in `akg-vs-stateless-a` | 409 | 688,188 | $1.83 | $0.0091 | $0.0104 | $0.0116 |
| `llm-akg-recent` in `akg-vs-stateless-b` | 459 | 837,596 | $2.03 | $0.0101 | $0.0095 | $0.0120 |
| `llm-fullhistory` in `fullhistory-vs-stateless-a` | 389 | 7,583,501 | $17.00 | $0.0850 | $0.0123 | $0.2149 |
| `llm-fullhistory` in `fullhistory-vs-stateless-b` | 477 | 11,258,273 | $22.43 | $0.1122 | $0.0149 | $0.2405 |

Completed session chip results:

| Session | Seat 0 agent | Seat 1 agent | Winner/result |
|---|---|---|---:|
| `akg-vs-stateless-a` | `llm-akg-recent` | `llm-stateless` | `llm-akg-recent` +1256 |
| `akg-vs-stateless-b` | `llm-stateless` | `llm-akg-recent` | `llm-stateless` +1003 |
| `fullhistory-vs-stateless-a` | `llm-fullhistory` | `llm-stateless` | `llm-fullhistory` +1202 |
| `fullhistory-vs-stateless-b` | `llm-stateless` | `llm-fullhistory` | `llm-stateless` +937 |

Interim takeaway: `llm-akg-recent` produced roughly comparable exploratory aggregate chip results against `llm-stateless` (+253 over 400 hands) to `llm-fullhistory` (+265 over 400 hands), but at about one tenth of full-history cost per hand. Full-history cost also grew sharply across each match, while bounded recency stayed mostly flat.

---

## Observations & Adjustments

*Dated entries. What we saw, what we changed, and why.*

---

*Research begins here.*
