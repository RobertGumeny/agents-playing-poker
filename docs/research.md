# Agents Playing Poker — Research State

**Authors:** Robert Gumeny  
**Repo:** [github.com/RobertGumeny/agent-poker](https://github.com/RobertGumeny/agent-poker)  
**Status:** Active research

## Thesis

Can structured, retrievable memory make an LLM agent measurably better — or measurably cheaper — at poker?

The specific claim:

> Given the same model, same game rules, and same tools, an LLM agent backed by a durable structured [AKG](https://github.com/RobertGumeny/akg) graph should hold an accurate, queryable model of its opponent at roughly flat per-decision cost as a session grows — while the unstructured baselines (raw full history, a single markdown file, a linked markdown "wiki") are forced to trade fidelity for cost as they scale.

Poker is the vehicle because it demands **opponent modeling** and **pattern recognition**, and because the engine logs every card and action — so what an agent claims to remember can be checked against what actually happened. See [`vision.md`](vision.md) for the framing; this document is the operational state.

## Current workflow

Research runs are experiment-first: create or edit a JSON experiment definition under `research/experiments/`, then run it with `poker experiment go <experiment-id>`. The operator workflow, CLI reference, and artifact authority split are in [`eval-system.md`](eval-system.md). The experiment JSON contract is in [`experiment-definition.md`](experiment-definition.md). Session artifact schemas are in [`session-artifacts.md`](session-artifacts.md).

## Game setup

The current harness runs heads-up no-limit Texas Hold'em.

- **Players:** 2
- **Starting stack:** 200bb by default in match runners unless overridden
- **Blinds:** 1/2 by default
- **Information realism:** showdown-only; opponent hole cards are visible to an agent only when a hand reaches showdown
- **Deal sequence:** deterministic by seed
- **Rules authority:** the server/rules engine, not the agents

The experiment definition pins hand count, seeds, strategies, and model, making each comparison reproducible from the checked-in plan.

## Strategy lineup

All LLM agents in a benchmark use the same model and runtime settings unless those settings are the explicit variable under test. The difference under test is **memory policy**. The lineup is a ladder of increasing structure (see [`vision.md`](vision.md) for the conceptual argument); each rung is expected to fail differently, and that failure mode is what we measure.

| Strategy | Structure | Predicted failure mode | Build status |
|---|---|---|---|
| `llm-stateless` | current hand only | no opponent model (floor) | built |
| `llm-fullhistory` | raw prior hands in prompt | context ceiling + O(N) token growth | built |
| `llm-md-single` | one freeform markdown file, rewritten | maintenance fidelity + full-rewrite cost | built |
| `llm-md-wiki` | linked markdown pages (`[[links]]`), a graph faked in prose | query/aggregation + link integrity | built |
| `llm-akg-durable` | typed, queryable AKG graph, model-maintained via write tools | maintenance drift in a typed graph (the thesis contestant) | built |

Supporting agents:

- **`llm-akg-recent`** — shallow bounded AKG memory: an opponent profile plus recent-hand summaries injected at decision time. A wiring and token-efficiency control, and an *intentionally weak* baseline — not proof of the structured-memory thesis.
- **`random` / `heuristic`** — scripted non-LLM agents for validating protocol, rules, artifacts, and local execution without model calls. Not part of the memory-strategy comparison.

The headline comparison the lineup is built toward is **`llm-md-wiki` vs `llm-akg-durable`**: if a linked pile of markdown matches AKG, the typed graph is overhead. Both are now **model-maintained** (the model writes its own memory after each hand), so the comparison cleanly isolates *representation* — typed graph vs linked prose — rather than extraction method. `llm-fullhistory` and `llm-md-single` bracket the cost and fidelity failure modes below it.

## Metrics and interpretation

**Primary lens — the fidelity-vs-cost frontier.** The thesis lives on two low-variance curves, both extractable from existing artifacts. These are what experiments should report first.

1. **Profile fidelity vs. hand count.** Cross-validate what the agent *states* it knows about the opponent against engine ground truth in `hands.jsonl` (VPIP, PFR, fold-to-c-bet, river aggression, 3-bet counts, showdown record). The "matches to the digit at 500 hands" result belonged to the *former code-computed* `llm-akg-durable` and was tautological (the extractor and the validation read the same action log — see [`research/llm-akg-durable-rework.md`](research/llm-akg-durable-rework.md)). Now that the durable agent is model-maintained, it is expected to *drift* like the prose rungs, and this check measures that drift against ground truth — exposing where each representation's accuracy decays, "where the patterns fall apart." *(Instrumentation note: this cross-validation has been done ad hoc against `hands.jsonl` + `memory-export.json`; folding it into the eval tooling as a standard analysis is pending.)*
2. **Tokens per decision vs. hand count.** From the prompts in `pi-session.jsonl`: per-decision input tokens and their growth slope. AKG should be ~flat; full-history linear; markdown-rewrite scaling with file size. This is where a *cost* win is demonstrated.

**Secondary — poker performance (diagnostic).** Chip delta, chips/BB per hand, treatment/control deltas, showdown vs non-showdown behavior. Treat single-session chip outcomes as diagnostics, not proof; they are high-variance and, in near-mirror matchups, uninformative without large samples. Prefer non-mirror matchups across the ladder, multiple seeds, and mirrored seat assignments to cancel positional effects.

**Process quality.** Malformed-action retries, fallback actions, decision-prompt counts, and AKG tool-use rates — durability and reliability signals, especially over long sessions.

**Memory quality.** Node/edge counts from `memory-export.json`, pattern availability and evidence counts, and whether queried evidence is relevant to the decision spot.

A design note that follows from the primary lens: strategies should run to **different hand counts on purpose**. Full-history failing early is itself a result ("broke at hand ~N"); there is no need for a common N.

## Current artifact model

A completed planned session writes under `research/experiments/<id>/sessions/<session-id>/`.

Authoritative session records (primary truth for game results):

- `manifest.json`
- `hands.jsonl`

Derived or agent-side records (additive analysis; regenerable without changing outcomes):

- `report.md`
- `eval.json`
- `agents/<name>/pi-session.jsonl`
- `agents/<name>/stderr.log`
- `agents/<name>/memory.akg`
- `agents/<name>/memory-export.json`

## Experiment arcs

**Run so far** — almost entirely `llm-akg-durable` vs `llm-stateless`, plus durability work:

- **`test-2b-retrieval-throttle`, `test-2c-once-per-hand`** — retrieval cost-tuning: cut redundant `akg_get_opponent` calls without hurting chips/hand.
- **`test-2d-showdown-pattern`** — added a richer pattern (`strong-showdown-caller`) to curb tail-risk showdown losses. **Lesson: enriching the memory model made play measurably worse, not better.** Shelved pending a revisit. This is the cautionary data point behind the rule that *AKG being able to store richer structure does not mean the LLM uses it well* — each addition must earn its keep empirically. It is also why no `confidence`/`status`/invalidation machinery is currently active in the durable agent.
- **`mirror-match-500`** — two durable agents, 500 hands. A durability and instrumentation sanity check: confirmed flawless `.akg` maintenance at scale, near-per-hand opponent querying with stat-cited reasoning, and two mutually consistent memory files from opposite seats. By design it does **not** measure a memory edge (identical memory both sides).

## Experiment roadmap

Three claims in increasing ambition. They are ordered, and each later phase is gated on the one before it.

### Phase 1 — Does `.akg` work functionally? ✅ Settled

`mirror-match-500` answered this unequivocally: flawless `.akg` maintenance across 500 hands (full node coverage, zero lost decisions, no degradation as the file grew to multiple MB) and an agent that queries and reasons over its model nearly every hand. (The "cross-validates to ground truth to the digit" property of that run came from the *code-computed* write path and was tautological; the now-model-maintained agent is expected to drift — see [`research/llm-akg-durable-rework.md`](research/llm-akg-durable-rework.md). The durability/engineering result stands regardless.) The durable store is real and reliable. Not in question.

### Phase 2 — Does the structure earn its keep? ⏳ Next

The ladder benchmark: the whole lineup against itself in **non-mirror** matchups at sub-500 hand counts, reported on the **fidelity-vs-cost frontier**, not chips. Strategies run to *different* hand counts on purpose — a rung breaking early (e.g. `llm-fullhistory` hitting the context ceiling) is a result, not a failure. The headline pairing is `llm-md-wiki` vs `llm-akg-durable`: can a knowledge graph faked in linked prose match a typed, queryable one?

Prerequisites:

- ~~Build `llm-md-single` and `llm-md-wiki` (see lineup)~~ — **done**; both built, model-maintained, and unit-tested. Contracts are specced in [`llm-md-single-spec.md`](llm-md-single-spec.md) and [`llm-md-wiki-spec.md`](llm-md-wiki-spec.md).
- Standardize the profile-fidelity cross-validation (stated read vs `hands.jsonl`) as a first-class eval analysis, rather than the ad-hoc script used to date — **still pending**.

The headline matchup is teed up as a checked-in pairwise definition at `research/experiments/phase2-wiki-vs-akg/` (seat-mirrored, non-mirror; agents built, definition runnable now), alongside the cost/fidelity brackets `research/experiments/phase2-fullhistory-vs-akg/` and `research/experiments/phase2-mdsingle-vs-akg/`. The full ladder is a set of such pairwise definitions — the experiment-definition contract is two-group, so each rung-vs-`llm-akg-durable` comparison is its own file.

The output of this phase is the **frontier baseline** that Phase 3 must beat.

> **🚧 Headline result — `phase2-wiki-vs-akg` (to fill in after tonight's run).**
>
> _Summarize the fidelity-vs-cost frontier for `llm-md-wiki` vs `llm-akg-durable` once the run completes. Pull from the generated report at `research/experiments/phase2-wiki-vs-akg/reports/`. Cover:_
> - _Profile fidelity vs. hand count — did the linked-prose wiki hold opponent-model accuracy, or did it drift, and where did it peel away from ground truth?_
> - _Tokens per decision vs. hand count — flat for AKG vs. growth for the wiki?_
> - _The one-line takeaway: can a knowledge graph faked in linked prose match a typed, queryable one — and at what cost?_

### Phase 3 — Can AKG evolve to react to a *shifting* opponent? 🔬 After Phase 2

Today's durable agent profiles an opponent's all-time *average*, so it structurally cannot notice an opponent changing gears. This phase asks whether AKG can — and splits cleanly into two halves:

- **Mechanism (AKG-native build):** windowed / recency-weighted profiling, plus `contradicts` / `superseded_by` edges so a stale read is *invalidated* rather than merely outweighed. All additive on the existing format; no rewrite.
- **Test (new instrument):** a **scripted, non-mirror opponent that changes strategy at a known hand**. Mirror self-play cannot provide this — with both sides drifting at once there is no ground-truth "the opponent changed *here*" to measure detection against.

**Gate — each evolution must beat the Phase 2 frontier baseline.** This is the discipline `test-2d` taught us: enriching the memory model made play *worse*. So a windowing/invalidation change is not accepted because the graph can now *represent* adaptation; it is accepted only if it moves the fidelity-vs-cost frontier in the right direction against the baseline. Buildable ≠ better; the frontier is the judge.

## Current research posture

Use the ladder to separate the questions cleanly:

1. Does any memory beat no memory? (`llm-stateless` floor)
2. Does raw full-history prompting buy enough to justify its token growth — and how far can it even get before the context ceiling? (`llm-fullhistory`)
3. Can a markdown knowledge base — flat or linked — hold a faithful opponent model as it scales, or does fidelity decay and rewrite cost climb? (`llm-md-single`, `llm-md-wiki`)
4. Does AKG's typed, queryable structure hold fidelity flat at flat per-decision cost where the prose representations cannot? (`llm-akg-durable`)

The strongest evidence comes from checked-in experiment definitions, reproducible seeds, generated reports, and artifact-level review — read on the fidelity-vs-cost frontier — not from one-off chip counts or procedural run notes.
