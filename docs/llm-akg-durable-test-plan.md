# LLM AKG Durable Short-Session Test Plan

## Goal

Design short-session experiments to test the main hypotheses from the prior `akg-durable-vs-stateless-test-1` analysis using **25–50 hand sessions**.

These tests are intended to detect **process and retrieval behavior**, not to prove durable-agent win rate in small samples.

## Prior baseline to anchor the tests

From `sessions/akg-durable-vs-stateless-test-1`:

- tool use was mostly shallow summary retrieval
  - `akg_get_opponent`: 33 calls
  - `akg_list_patterns`: 6 calls
  - `akg_get_pattern`: 0 calls
  - `akg_list_hands`: 0 calls
  - `akg_get_hand`: 0 calls
- memory persistence was valid but sparse
  - 25 hand nodes
  - 1 opponent node
  - 1 pattern node
  - 5 total edges
- useful memory did appear within 25 hands
  - hand 6: respected a rare raise based on 0% PFR over 5 hands
  - hand 25: used stored `folds-to-cbet` tendency
- formatting friction was real
  - malformed commentary+JSON outputs in `stderr.log`
  - 59 decision prompts for 47 unique spots, implying retries/fallbacks
- current runtime/prompt shape biases toward summary lookup
  - `runtime.ts` explicitly says to call `akg_get_opponent` first
  - deeper tools are framed as optional follow-ups
  - `tools.ts` makes deeper evidence retrieval more awkward than summary lookup

## Variant naming convention

Treat each tested agent build as an **experiment variant**, not a normal product semver release.

Recommended naming style:

- `llm-akg-durable-baseline`
- `llm-akg-durable@exp-0.1.1-formatting`
- `llm-akg-durable@exp-0.1.2-prompt`
- `llm-akg-durable@exp-0.1.3-tools`
- `llm-akg-durable@exp-0.1.4-rich-data`

For every run, record at minimum:

- agent variant name
- git commit SHA
- model/provider/version
- thinking level
- hypothesis under test
- opponent
- seed list
- hand count and session count

## Common experimental controls

Keep these fixed unless they are the explicit variable under test:

- same durable agent code except the tested variant
- same model/provider/version
- same thinking level
- same blinds, stacks, and match format
- same hand count within a given A/B
- same opponent
- same seat rotation policy
- same seed list where pairing is feasible
- same measurement script across all runs

## Core metrics to collect for every run

### From `pi-session.jsonl`

Per unique decision spot, collect:

- unique spots = unique `(hand, street, prompt hash)`
- duplicate prompts = repeated prompts for the same spot
- tool-use rate = spots with any AKG tool call / unique spots
- `akg_get_opponent` first-call rate
- deep-retrieval rate = spots with any of:
  - `akg_list_patterns`
  - `akg_get_pattern`
  - `akg_list_hands`
  - `akg_get_hand`
- evidence-path rate = spots with `akg_get_pattern` and/or `akg_get_hand`
- first hand where deep retrieval appears
- first hand where retrieval occurs after a pattern exists
- optional: token use and latency

### From `stderr.log`

- malformed-action count
- retry count
- retry rate = retrying spots / unique spots
- fallback count
- fallback rate

### From memory artifacts (`memory.akg`)

- hand node count
- pattern node count by slug
- edge counts:
  - `shows_pattern`
  - `supported_by`
- first hand number at which each pattern becomes available
- pattern availability window = `session_hands - first_pattern_hand`

---

## Test 1: formatting friction

### Hypothesis

Retries and malformed JSON outputs are suppressing memory use and muddying short-session behavior.

### Variant to run

- **A:** `llm-akg-durable-baseline` — current strict behavior
- **B:** `llm-akg-durable@exp-0.1.1-formatting` — lenient output acceptance variant

For B, accept a final valid JSON object even if commentary precedes it, instead of forcing a retry.

This isolates formatting friction without changing retrieval affordances.

### What stays controlled

- same prompt contract
- same tools
- same opponent
- same model and thinking level
- same hand counts and seeds where possible

### How many hands / sessions

- **8 paired sessions × 25 hands**
- if cheap, add **4 paired sessions × 50 hands**

### Metrics to collect

Primary:

- malformed-action count
- retry rate
- fallback count
- duplicate prompt count

Secondary:

- tool-use rate
- deep-retrieval rate
- total tool calls per unique spot

### Outcomes that support or reject the hypothesis

Supports formatting-friction hypothesis if:

- malformed outputs drop sharply in B
- retries and fallbacks drop sharply in B
- deep retrieval or total tool use rises meaningfully in B
- first-pass AKG reasoning is preserved more often in B

Rejects or weakens the hypothesis if:

- B removes retries but deep retrieval stays basically unchanged
- behavior remains mostly summary-only (`akg_get_opponent` almost exclusively)
- decision process does not materially change beyond output formatting

---

## Test 2: discoverability prompt A/B

### Hypothesis

The prompt contract makes AKG look like a summary lookup rather than an evidence graph.

### Variant to run

Run this after choosing the formatting-friction winner.

- **A:** winning formatting variant carried forward as control
- **B:** `llm-akg-durable@exp-0.1.2-prompt` — stronger retrieval-contract prompt

Prompt B should explicitly frame:

- `akg_get_opponent` as a starting index only
- deeper tools as expected when stats or patterns matter
- pattern inspection as something to verify before relying on
- hand inspection as appropriate for unusual or high-leverage spots

### What stays controlled

- same parser / output-acceptance behavior
- same tool surface
- same opponent
- same model and runtime settings
- same hand counts and seeds where possible

### How many hands / sessions

- **8 paired sessions × 25 hands**
- plus **4 paired sessions × 50 hands** if pattern opportunities are too sparse in 25-hand runs

### Metrics to collect

Primary:

- deep-retrieval rate
- first hand with deep retrieval
- spots using deeper tools after pattern availability
- nonzero use of `akg_get_pattern` and `akg_get_hand`

Secondary:

- malformed / retry rate stays flat or improves
- retrieval sequences become less shallow on qualitative review

### Outcomes that support or reject the hypothesis

Supports discoverability hypothesis if:

- B materially increases deep-tool usage
- deep retrieval starts earlier
- the agent uses deeper tools specifically once patterns become available
- `akg_get_pattern` and/or `akg_get_hand` move from zero to nontrivial usage

Rejects or weakens the hypothesis if:

- even with explicit prompt guidance, behavior remains mostly `akg_get_opponent`
- deeper tools stay near-zero despite patterns existing in memory

---

## Test 3: tool ergonomics A/B

### Hypothesis

The deeper retrieval path is too awkward, so the agent stops at summaries.

### Variant to run

Run this after picking the better prompt.

- **A:** winning prompt variant carried forward as control
- **B:** `llm-akg-durable@exp-0.1.3-tools` — more ergonomic deep-retrieval surface

Good B candidates include:

- a pattern tool that returns pattern plus supporting hand summaries, not just hand IDs
- a single tool that loads pattern evidence directly
- a relevant-hands tool keyed by pattern or tag

The goal is to reduce the current multi-hop path:

`akg_get_opponent -> akg_list_patterns -> akg_get_pattern -> akg_get_hand`

### What stays controlled

- same prompt
- same parser / output-acceptance behavior
- same opponent
- same model and runtime settings

### How many hands / sessions

Prefer higher evidence density here:

- **6 paired sessions × 50 hands**

If only shorter runs are practical:

- **10 paired sessions × 25 hands**

Use an opponent likely to generate repeated tendencies quickly if possible.

### Metrics to collect

Primary:

- nonzero use of evidence tools
- evidence-path rate
- tool hops per evidence retrieval
- fraction of post-pattern spots that inspect support

Secondary:

- total deep-retrieval rate
- retry behavior remains stable

### Outcomes that support or reject the hypothesis

Supports ergonomics hypothesis if:

- B shifts behavior from summary-only usage toward evidence usage
- `akg_get_pattern` / hand-level evidence becomes common enough to observe in 25–50 hands
- fewer tool hops are needed per useful retrieval

Rejects or weakens the hypothesis if:

- even easier deep tools remain mostly unused
- the agent still stops at opponent summary despite accessible evidence

---

## Test 4: richer data within 25–50 hands

### Hypothesis

Short runs may not naturally produce enough usable pattern evidence, so retrieval looks shallow mainly because the graph is still sparse.

### Variant to run

This is primarily a session-design test rather than a code A/B.

Use `llm-akg-durable@exp-0.1.4-rich-data` as the label for the best parser/prompt/tool combination carried forward into the richer-data runs.

Run that winning build under three conditions:

- **A:** 25 hands, current opponent
- **B:** 50 hands, same opponent
- **C:** 25 hands, higher-signal opponent profile

For C, prefer an opponent that produces repeated exploitable tendencies quickly.

### What stays controlled

- same durable agent build
- same runtime settings
- same measurement process

### How many hands / sessions

- **6 sessions per condition**
- if the opponent is noisy, increase to **8 sessions per condition**

### Metrics to collect

Primary:

- pattern count by session end
- first pattern hand number
- number of spots after first pattern exists
- deep retrieval in post-pattern spots
- whether a stored pattern is referenced in later decision behavior

Secondary:

- `supported_by` edge counts
- hand-node and evidence density at 25 vs 50 hands

### Outcomes that support or reject the hypothesis

Supports lack-of-data hypothesis if:

- 50-hand or higher-signal 25-hand runs create patterns earlier or more often
- retrieval/use rises once those patterns exist
- shallow behavior weakens under richer short-run data without needing large prompt/tool changes

Rejects or weakens the hypothesis if:

- richer short-run data appears in memory but the agent still does not inspect it
- pattern availability changes little between 25 and 50 hands

---

## Recommended test order

1. formatting friction
2. discoverability prompt
3. tool ergonomics
4. richer-data session design

This order matters because retries are a confound for the later tests.

## What short sessions can realistically detect

### Good fits for 25-hand runs

- malformed-output and retry problems
- whether prompt wording changes tool adoption
- whether summary-only behavior persists
- whether patterns appear at all

### Better fits for 50-hand runs

- whether deeper tools get used once evidence exists
- whether ergonomics changes unlock pattern/hand inspection
- whether sparse data was the real blocker

### Not reliable in 25–50 hands

- win rate
- strong EV conclusions
- broad strategic superiority claims
