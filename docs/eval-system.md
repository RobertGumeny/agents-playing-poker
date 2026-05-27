# Eval System Design

## What the current process actually requires

This is a candid account of what it took to produce the Test 2 analysis comparing
`akg-durable-prompt-test-{1..5}` against `akg-durable-vs-stateless-test-{1..5}`.
Every step below happened interactively in the LLM conversation — which means waiting
for tool calls, writing throwaway Python, re-probing file formats, and burning context.

### Step 1 — discover the sessions

```
ls sessions/ | grep <pattern>
```

Manual. No experiment registry. The session names are typed free-form at run time and
inferred from convention. If you name something slightly wrong, it just doesn't appear.

### Step 2 — read chips_delta and duration

```
cat sessions/*/manifest.json   (×10 files)
```

`manifest.json` has the result but not duration — duration is `ended_at - started_at`,
which requires arithmetic. Chips-per-hand requires dividing by `hand_count`. Neither is
pre-computed. Both have to be derived every time.

### Step 3 — read per-hand log and stats

```
cat sessions/*/report.md   (×10 files)
```

`report.md` is generated and useful, but the LLM still has to read all 10 files into
context to compare them. There is no cross-session aggregation anywhere. The showdown
rate, preflop-only rate, and biggest pot are in the report, but nothing computes a
mean across sessions.

### Step 4 — count memory file size

```
wc -l sessions/*/agents/llm-akg-durable/memory.akg
```

This gives line count as a rough proxy for memory density. The actual node/edge counts
and graph content require parsing the binary AKG file, which no tooling here does.

### Step 5 — count tool calls from pi-session.jsonl

This was the most expensive step. The `pi-session.jsonl` format is not documented in
this repo. I had to probe it first:

```
head -c 500 sessions/.../pi-session.jsonl
```

Then infer the schema: events have `type: "message"`, messages have `content: []`,
content blocks have `type: "toolCall"` (not `"tool_use"` — easy to get wrong). Then
write a Python one-liner to count by name:

```python
for line in f:
    obj = json.loads(line)
    if obj["type"] == "message":
        for c in obj["message"]["content"]:
            if c["type"] == "toolCall":
                counts[c["name"]] += 1
```

Ran that script 10 times across both groups. No pre-existing tooling for this anywhere
in the repo.

### Step 6 — dig into a specific hand for qualitative evidence

To confirm what the agent was actually doing in a big winning hand, I had to:

1. Identify the hand number from `report.md`
2. Scan `pi-session.jsonl` for the matching user prompt (grepping for `"Hand: 24"`)
3. Read the assistant's thinking block and tool calls for that turn

This took 4–5 separate tool calls, all throwaway, outputting hundreds of lines of raw
JSON into the conversation context.

---

## The cost

For a 10-session, 2-group comparison:

- ~20 file reads (manifests + reports)
- ~10 custom Python scripts written and discarded
- ~5 format-probing tool calls
- unknown number of tokens wasted parsing raw JSONL in the LLM context
- no artifact produced that can be reused for the next experiment

The LLM is doing ETL work that a small CLI could do in milliseconds offline.

---

## Proposed eval system

The system has three parts: an **experiment definition**, a **server-side memory
export** (new session artifact), and two **eval CLI commands**. All post-run analysis
is runnable with no LLM involvement.

---

### Part 1 — Experiment definition file

A YAML file checked into the repo alongside the session output. Created before the
sessions are run.

```yaml
# experiments/test-2b-retrieval-throttle.yaml
id: test-2b-retrieval-throttle
hypothesis: >
  Throttling memory retrieval to once per hand reduces akg_get_opponent calls by ~65%,
  shortens session duration by ~35%, and recovers chips/hand toward baseline.
treatment:
  session_base: akg-durable-throttle-test
  sessions_count: 5
  agent: llm-akg-durable@exp-0.1.3-throttle
  opponent: llm-stateless
  seeds: []
control:
  session_base: akg-durable-vs-stateless-test
  sessions_count: 5
  agent: llm-akg-durable/0.1.0
  opponent: llm-stateless
  seeds: []
expected_direction:
  chips_per_hand: increase
  akg_get_opponent_per_session: decrease
  session_duration_s: decrease
  preflop_only_rate: decrease
hands_per_session: 25
```

`session_base` and `sessions_count` define the expected session IDs:
`<session_base>-1`, `<session_base>-2`, and so on through `sessions_count`.
`seeds` is optional. If omitted or empty, the eval runner treats the session index as
the seed, so the five sessions above use seeds `1..5`. If `seeds` is present, its
length must match `sessions_count`.

`opponent` is optional. When present, it records the intended opposing agent for that
group. When omitted, the comparison tooling derives opponents from each session's
`manifest.json`; if the manifests disagree inside a group, the report calls that out.

For already-run sessions whose names do not follow this convention, the group can use
an explicit session list instead:

```yaml
treatment:
  sessions: [akg-durable-throttle-a, akg-durable-throttle-b]
  agent: llm-akg-durable@exp-0.1.3-throttle
  seeds: [17, 23]
```

This file is the source of truth for what the experiment is testing. It also means the
comparison can be re-run at any time without reconstructing context.

---

### Part 2 — Server-side memory export (session artifact)

This is a change to the **match runner**, not an eval tool. At session teardown, after
agents are closed and `manifest.json` is written, the runner calls `WriteMemoryExport`
for each agent directory. This produces `memory-export.json` alongside the existing
`memory.akg`.

**`memory.akg` is kept as-is** — it is the authoritative persistent store and stays
for posterity, reproducibility, and any future tooling that needs to re-open the live
store (e.g. resumable sessions). `memory-export.json` is a read-only snapshot for
analysis.

#### Teardown hook in `internal/match/runner.go`

```go
defer func() {
    for _, agent := range agents {
        shutdownCtx, cancel := context.WithTimeout(context.Background(), r.config.ShutdownGracePeriod)
        _ = agent.Close(shutdownCtx)
        cancel()
    }
    manifestErr := writer.WriteManifest(r.buildManifest(...))
    if runErr == nil && manifestErr != nil {
        runErr = manifestErr
    }
    // Export each agent's memory graph to JSON for offline analysis.
    // No-op (logged, not fatal) if memory.akg is absent or unreadable.
    for _, spec := range r.config.AgentSpecs {
        agentDir, dirErr := writer.AgentDir(spec.Name)
        if dirErr == nil {
            _ = sessionlog.WriteMemoryExport(agentDir)
        }
    }
}()
```

#### `sessionlog.WriteMemoryExport`

Lives in `internal/sessionlog/`. Depends on `github.com/RobertGumeny/akg/sdk/akg-go`.

```go
// WriteMemoryExport opens memory.akg in agentDir (if present), walks all nodes
// and edges, and writes memory-export.json alongside it. Errors are non-fatal —
// a missing or unreadable memory.akg simply produces no export file.
func WriteMemoryExport(agentDir string) error
```

Signature notes:
- Returns an error for the caller to log; the runner never fails a session over it.
- Does nothing if `memory.akg` does not exist (stateless agents produce none).

#### Output: `sessions/<id>/agents/<name>/memory-export.json`

Full graph dump — all nodes with complete body/meta/tags, all edges with relation and
meta. The schema is stable by construction: it mirrors the AKG node/edge types directly.

```json
{
  "nodes": [
    {
      "type": "opponent",
      "id": "villain",
      "title": "villain",
      "body": "Villain is loose-passive (VPIP 74%, PFR 22%) and folds to hero flop c-bets 4/7 times (57%). River aggression shows up in 1/23 hands (4%). Villain has won 6 of 8 showdowns.",
      "meta": {
        "hands_played": 23,
        "vpip": 17,
        "pfr": 5,
        "fold_to_bet": 14,
        "cbet_opportunities": 7,
        "cbet_folds": 4
      },
      "tags": ["opponent"]
    },
    {
      "type": "pattern",
      "id": "folds-to-cbet",
      "title": "Folds to flop c-bets",
      "body": "Villain has folded to hero flop c-bet 4 times across 7 c-bet opportunities.",
      "meta": { "count": 4, "opportunities": 7 },
      "tags": ["pattern"]
    },
    {
      "type": "hand",
      "id": "0a6761eab5f6f19b",
      "title": "Hand 1",
      "body": "Hand 1: hero sb, hole [8c Qc], board [-]. preflop: hero raise 6, villain fold. Reached preflop. Net: +2.",
      "meta": {
        "hand_number": 1,
        "hero_position": "sb",
        "hero_net": 2,
        "street_reached": "preflop",
        "villain_fold": true,
        "villain_vpip": false
      },
      "tags": ["hand", "villain_fold"]
    }
  ],
  "edges": [
    {
      "from": { "type": "opponent", "id": "villain" },
      "relation": "shows_pattern",
      "to": { "type": "pattern", "id": "folds-to-cbet" },
      "meta": { "count": 4, "opportunities": 7 }
    },
    {
      "from": { "type": "pattern", "id": "folds-to-cbet" },
      "relation": "supported_by",
      "to": { "type": "hand", "id": "0a6761eab5f6f19b" },
      "meta": { "hand_number": 1 }
    }
  ]
}
```

After this, the session artifact layout for a memory-capable agent is:

```
sessions/<id>/
  manifest.json
  hands.jsonl
  report.md
  agents/
    llm-akg-durable/
      memory.akg           ← binary store, kept for posterity
      memory-export.json   ← NEW: full graph dump, human- and machine-readable
      pi-session.jsonl
      stderr.log
      stdout.log
    llm-stateless/
      pi-session.jsonl
      stderr.log
      stdout.log
```

---

### Part 3 — Metrics collector (`poker-eval collect`)

A CLI command that reads session artifacts (including `memory-export.json`) and writes
a structured `eval.json` into each session directory. Runs post-session, completely
offline, no LLM, no AKG SDK dependency.

```
poker-eval collect sessions/akg-durable-throttle-test-{1..5}
```

Because `memory-export.json` is plain JSON, `collect` never needs to touch `memory.akg`
or the AKG SDK directly.

Output per session — `sessions/<id>/eval.json`:

```json
{
  "session_id": "akg-durable-throttle-test-1",
  "agent": "llm-akg-durable@exp-0.1.3-throttle",
  "agent_seat": 0,
  "hands": 25,
  "seed": 1,
  "duration_s": 541,
  "chips_delta": 56,
  "chips_per_hand": 2.24,
  "preflop_only_rate": 0.52,
  "showdown_rate": 0.08,
  "biggest_pot": 19,
  "tool_calls": {
    "akg_get_opponent": 28,
    "akg_list_patterns": 6,
    "akg_get_pattern": 3,
    "akg_list_hands": 0,
    "akg_get_hand": 0
  },
  "tool_calls_per_hand": {
    "akg_get_opponent": 1.12,
    "akg_list_patterns": 0.24
  },
  "memory": {
    "hand_nodes": 25,
    "pattern_nodes": 2,
    "opponent_nodes": 1,
    "shows_pattern_edges": 2,
    "supported_by_edges": 8,
    "opponent_summary": "Villain is loose-passive (VPIP 74%, PFR 22%)...",
    "patterns": [
      {
        "slug": "folds-to-cbet",
        "title": "Folds to flop c-bets",
        "body": "Villain has folded to hero flop c-bet 4 times across 7 c-bet opportunities.",
        "count": 4,
        "opportunities": 7,
        "first_hand": 9,
        "supporting_hand_count": 4
      }
    ]
  },
  "decision_attempts": 47,
  "unique_decision_spots": 45,
  "retry_count": 2,
  "retry_rate": 0.04
}
```

Sources for each field:
- `chips_delta`, `duration_s`, `seed`, `hands` — `manifest.json`
- `preflop_only_rate`, `showdown_rate`, `biggest_pot` — `hands.jsonl`
- `tool_calls` — `pi-session.jsonl` (parse `content[].type == "toolCall"`)
- `memory.*` — `memory-export.json` (plain JSON, no special parser)
- `decision_attempts`, `retry_count` — `pi-session.jsonl`

---

### Part 4 — Comparison report generator (`poker-eval compare`)

Takes an experiment definition and produces a markdown comparison table.

```
poker-eval compare experiments/test-2b-retrieval-throttle.yaml
```

Output — `experiments/test-2b-retrieval-throttle-report.md`:

```markdown
# Experiment: test-2b-retrieval-throttle

**Hypothesis:** Throttling memory retrieval to once per hand...

## Summary

| Metric | Control (n=5) | Treatment (n=5) | Δ | Direction |
|---|---|---|---|---|
| chips/hand (mean) | +5.12 | +2.24 | -2.88 | ❌ wrong direction |
| akg_get_opponent/session | 33.4 | 27.2 | -6.2 (−19%) | ✅ |
| session duration (s) | 465 | 398 | -67 (−14%) | ✅ |
| preflop-only rate | 46.4% | 53.1% | +6.7pp | ❌ wrong direction |

## Per-Session Results
...

## Tool Use
...

## Memory Density
...

## Memory Graph (Treatment, Session 1)

**Opponent model:** Villain is loose-passive (VPIP 74%, PFR 22%)...

**Patterns identified:**
- folds-to-cbet: 4/7 cbet opportunities, first observed hand 9
```

The `✅` / `❌` direction check comes directly from `expected_direction` in the
experiment YAML. The "Memory Graph" section is populated directly from
`memory-export.json` — no parsing required.

---

## Scope and non-goals

This is intentionally minimal for v0. It does not need to:

- run sessions (that's `poker-server` / `doug`)
- aggregate across experiments (nice-to-have, not urgent)
- produce statistical significance tests (sample sizes are too small for that to matter)
- be a web dashboard

The whole thing is probably 500–700 lines of Go. The `eval.json` schema and the
`memory-export.json` schema should both be added to a focused project doc once they stabilize.

## Where this fits in the build phasing

The **server-side memory export** (`WriteMemoryExport`) is a small, self-contained
change to the match runner and `sessionlog` package. It can land as part of any sprint
because it only adds an artifact — it does not change session behavior.

The **eval CLI** (`poker-eval collect` + `compare`) is a standalone epic. It does not
block any experiment; the experiments can run without it. But the first time you run a
3-way comparison across 15 sessions, you'll want it badly.

## Suggested epic scope

```
server change:    sessionlog.WriteMemoryExport     — dumps memory.akg → memory-export.json at session end  (small, land early)
poker-eval collect — parse session artifacts into eval.json                                                  (core)
poker-eval compare — diff two session groups from an experiment YAML                                         (core)
poker-eval init    — scaffold a new experiment YAML from a template                                          (nice-to-have)
poker-eval ls      — list experiments and their session coverage                                              (nice-to-have)
```

Land `WriteMemoryExport` early so future sessions automatically produce the export.
Start `collect` and `compare` when the experiment backlog justifies the investment.
