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

## Eval system and current operator loop

The current system has six operator-facing commands around the same artifact chain:

1. `poker-eval init` — create a schema-valid experiment definition JSON file
2. `poker-eval ls` — discover checked-in experiment files and summarize coverage
3. `poker-eval status` — inspect one experiment's planned coverage in detail
4. `poker-eval run` — launch missing or incomplete planned sessions through `poker-run`
5. `poker-eval collect` — derive per-session `eval.json` summaries from session artifacts
6. `poker-eval compare` — render a markdown control-vs-treatment report from collected sessions

The normative JSON experiment contract lives in [`experiment-definition.md`](experiment-definition.md). The stable additive session-artifact schemas live in [`session-artifacts.md`](session-artifacts.md).

---

### Part 1 — Experiment definition file

The source of truth is a checked-in JSON plan created before the sessions run.

```json
{
  "id": "test-2b-retrieval-throttle",
  "hypothesis": "Throttling memory retrieval to once per hand reduces akg_get_opponent calls and session duration.",
  "treatment": {
    "session_base": "akg-durable-throttle-test",
    "sessions_count": 5,
    "agent": "llm-akg-durable@exp-0.1.3-throttle",
    "opponent": "llm-stateless",
    "seeds": []
  },
  "control": {
    "session_base": "akg-durable-vs-stateless-test",
    "sessions_count": 5,
    "agent": "llm-akg-durable/0.1.0",
    "opponent": "llm-stateless",
    "seeds": []
  },
  "expected_direction": {
    "chips_per_hand": "increase",
    "akg_get_opponent_per_session": "decrease",
    "session_duration_s": "decrease",
    "preflop_only_rate": "decrease"
  },
  "hands_per_session": 25
}
```

`session_base` and `sessions_count` define the expected session ids:
`<session_base>-1`, `<session_base>-2`, and so on through `sessions_count`.
`seeds` is optional. If omitted or empty, planned seeds default deterministically to
`1..sessions_count`.

`opponent` is optional at the file-format level. When present, it records the intended
opposing agent for that group. `poker-eval run` requires it for any planned session it
needs to launch. `poker-eval compare` can derive opponents from collected session data
when the definition omits it.

For already-run sessions whose names do not follow the `<session_base>-N` convention,
a group can use explicit session ids instead:

```json
{
  "treatment": {
    "sessions": ["akg-durable-throttle-a", "akg-durable-throttle-b"],
    "agent": "llm-akg-durable@exp-0.1.3-throttle",
    "seeds": [17, 23]
  }
}
```

This file is the durable plan for what the experiment is testing and what coverage
should exist on disk.

`poker-eval init` scaffolds this JSON contract directly:

```bash
go run ./cmd/poker-eval init \
  -out experiments/test-2b-retrieval-throttle.json \
  -hypothesis "Throttling retrieval should cut tool use and session time." \
  -control-agent llm-stateless \
  -control-opponent heuristic \
  -treatment-agent llm-akg-recent
```

---

### Part 2 — Coverage and execution (`poker-eval ls`, `status`, and `run`)

The experiment definition is the source of truth for what should exist. Coverage
commands inspect the filesystem only to classify whether each planned session is usable.

Useful commands:

```bash
go run ./cmd/poker-eval ls
go run ./cmd/poker-eval status -experiment experiments/test-2b-retrieval-throttle.json
go run ./cmd/poker-eval run -experiment experiments/test-2b-retrieval-throttle.json -dry-run
go run ./cmd/poker-eval run -experiment experiments/test-2b-retrieval-throttle.json -model anthropic:claude-sonnet-4-6
```

Coverage states:
- `present` — complete `manifest.json` plus matching `hands.jsonl`
- `missing` — no session directory yet
- `incomplete` — directory exists but primary artifacts do not match the planned session

Execution semantics:
- `poker-eval run` skips `present` sessions
- `poker-eval run` relaunches `missing` and `incomplete` sessions
- `poker-eval run` delegates each launch to `poker-run`
- `-model` and `-thinking-level` are runtime-only overrides forwarded into `poker-run`

This preserves `poker-run` as the low-level primitive for one real session while
`poker-eval` handles deterministic multi-session planning and coverage.

---

### Part 3 — Single-session execution artifacts (`poker-run`)

`poker-run` remains the primitive that actually executes one session.

```bash
go run ./cmd/poker-run \
  -agent0 llm-akg-recent \
  -agent1 llm-stateless \
  -hands 200 \
  -seed 3 \
  -session-id akg-recent-vs-stateless-seed3-a \
  -model anthropic:claude-sonnet-4-6
```

A completed session writes artifacts under `sessions/<id>/`:

```
sessions/<id>/
  manifest.json
  hands.jsonl
  report.md
  agents/
    <seat-name>/
      stdout.log
      stderr.log
      pi-session.jsonl      # when the seat is Pi-backed
      memory.akg            # when the agent owns durable memory
      memory-export.json    # optional server-generated teardown export
```

Artifact roles:
- `manifest.json` and `hands.jsonl` are the primary session authority
- `report.md` is a convenience single-session report
- `memory.akg` remains the authoritative durable memory store
- `memory-export.json` is an additive JSON snapshot for offline analysis

---

### Part 4 — Metrics collector (`poker-eval collect`)

`collect` reads the session bundle offline and writes `eval.json` into each named
session directory.

```bash
go run ./cmd/poker-eval collect sessions/akg-durable-throttle-test-{1..5}
```

Because `memory-export.json` is plain JSON, `collect` never opens `memory.akg` and does
not depend on the AKG SDK.

Output per session:

- `sessions/<id>/eval.json` — deterministic derived summary built from `manifest.json`,
  `hands.jsonl`, optional `pi-session.jsonl`, optional `memory-export.json`, and
  optional `stderr.log`

The stable contract now lives in [`session-artifacts.md`](session-artifacts.md#evaljson).

---

### Part 5 — Comparison report generator (`poker-eval compare`)

`compare` reads the checked-in experiment definition plus collected `eval.json` files
for every planned session and renders markdown to stdout.

```bash
go run ./cmd/poker-eval compare -experiment experiments/test-2b-retrieval-throttle.json
```

If you want a file, redirect stdout yourself:

```bash
go run ./cmd/poker-eval compare -experiment experiments/test-2b-retrieval-throttle.json \
  > reports/test-2b-retrieval-throttle.md
```

Current report contents include:
- summary metric table for control vs treatment
- tool-use table when tool-call metrics were observed
- warnings for mixed observed group metadata
- per-session results table
- `expected_direction` pass/fail checks from the experiment definition

---

## Scope and non-goals

This is intentionally minimal for v0. It currently supports the operator loop from
planned experiment definition through run, collect, and compare, but it does not try to:

- replace `poker-run` as the low-level single-session execution primitive
- aggregate across multiple experiment definitions in one report
- produce statistical significance tests
- become a dashboard or long-running service

The stable `eval.json` and `memory-export.json` contracts live in [`session-artifacts.md`](session-artifacts.md).

## Build-phasing note

Historically, this area landed in stages:

1. experiment-definition parsing and planning
2. additive `memory-export.json` teardown support
3. `poker-eval run` and `status`
4. offline `collect`
5. offline `compare`
6. `init` and `ls` operator ergonomics

That phased delivery explains why related implementation notes are split across the KB
articles in [`docs/kb/`](kb/README.md).
