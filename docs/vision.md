# Vision: Agents That Remember and Reason

## The core insight

The biggest unlock for agent capability is not better prompts or bigger context windows. It is giving agents access to structured memory they can actually reason about.

Retrieval is necessary but not sufficient. An agent that can fetch a few relevant text chunks is doing glorified search. An agent that can traverse a knowledge structure — follow edges, identify patterns, reason about relationships between past events — is doing something qualitatively different. That is what this project is designed to demonstrate.

## Why poker

Poker is a controlled, adversarial, multi-session environment with a clean ground truth: chips. It produces a measurable outcome for every decision, every hand, every session. It requires modeling an opponent over time, updating beliefs as new evidence arrives, and acting under uncertainty.

It also gives us something rarer: a **verifiable memory**. Because the engine logs every hole card and action, we can take whatever an agent *claims* to remember about its opponent and check it against what actually happened. That turns "did the memory work" from a vibe into a measurement.

## The ladder of memory representations

Memory is the variable under test. The right way to see the lineup is as a ladder of increasing structure, where each rung is expected to **fail in a different, predictable way**. Naming the failure mode up front is what tells us *what to measure* — not just who won.

| Strategy | Structure | Predicted failure mode |
|---|---|---|
| `llm-stateless` | none — current hand only | No opponent model at all. The floor. |
| `llm-fullhistory` | raw transcript in context | **Context ceiling + O(N) cost.** Tokens grow every hand; it cannot reach long sessions before it blows the window or the budget. The "stuff it all in" baseline on the cost axis. |
| `llm-md-single` | one freeform markdown file the agent rewrites | **Maintenance fidelity + rewrite cost.** No addressability: to update, the agent re-reads and re-writes the whole file. Facts get silently dropped, contradicted, or bloated; rewrites get expensive and lossy as the file grows. |
| `llm-md-wiki` | linked markdown pages (`[[wiki links]]`) — a knowledge graph faked in prose | **Query/aggregation + link integrity.** Better than one file: addressable, traversable. But it is still text. It cannot *compute* "folds to c-bet 39/77"; it has to re-read and re-tally in-context, and links rot at scale. This is the true head-to-head rival to AKG — same structural ambition, no typed or queryable substrate. |
| `llm-akg-durable` | typed, queryable graph with computed aggregates | The thesis strategy. A decision-time query returns a *precomputed* opponent profile, so per-decision cost stays roughly flat regardless of session length. |

Two supporting strategies sit off the ladder: `llm-akg-recent` is a shallow bounded-AKG wiring/efficiency control (honestly a weak baseline, not proof of anything), and `random`/`heuristic` are non-LLM scripted agents for validating protocol, rules, and artifacts without model calls.

The point of the ladder is that `llm-md-wiki` vs `llm-akg-durable` is the experiment that actually matters. If a linked pile of markdown can do everything AKG does, AKG is overhead. The thesis is that it cannot — that somewhere up the ladder, *holding an accurate, cheap-to-query model of a 200-hand opponent* stops being something you can fake in prose.

## The shape of the claim: a fidelity-vs-cost frontier

The experiment is **not** primarily "which agent wins the most chips." Chip outcomes are noisy — a few all-in pots, seat and blind-rotation effects, LLM nondeterminism, and ordinary variance dominate short runs, and in near-mirror matchups they need enormous sample sizes to mean anything. Chips are a diagnostic, not the thesis.

The thesis lives on two lower-variance curves, both measurable from artifacts the harness already produces:

1. **Profile fidelity vs. hand count.** Take what the agent *states* it knows about the opponent and cross-validate it against engine ground truth (`hands.jsonl`). For `llm-akg-durable` this already matches to the digit at 500 hands. Run the same check up the ladder and watch the error grow. **Where each curve peels away from zero is, literally, "where the patterns fall apart."**
2. **Tokens per decision vs. hand count.** `pi-session.jsonl` carries every prompt, so this is directly extractable. AKG should be roughly flat; full-history climbs linearly; markdown-rewrite climbs with file size.

The result we are hunting is a frontier: **AKG holds fidelity flat while keeping per-decision cost flat, and the unstructured strategies are forced to trade one for the other.** If that is true, the fidelity-vs-cost picture shows it far more convincingly — and more honestly — than a chip count ever could. A win can be a *performance* win, a *cost* win, or both; the frontier captures all three.

A corollary on experiment design: the strategies should run to **different hand counts on purpose**. Full-history failing early is a finding, not a problem — "it broke at hand ~N" is exactly the kind of result we want. There is no need for a common N; we want each representation's breaking point.

## Why AKG and not markdown files

Markdown works. It is readable, writable, and easy to produce. It is also fragile: it grows noisy, lacks queryable structure, resists deterministic retrieval, and degrades as a reasoning substrate as it scales. The `llm-md-*` rungs exist precisely to measure *how* fragile, and at what scale.

AKG is a compact graph format designed to be stored anywhere, queried deterministically, and extended cleanly. It is not a document format that happens to contain facts — it is a fact store that happens to be readable.

Crucially, AKG has **headroom the current strategy barely uses**. Today's `llm-akg-durable` keeps an aggregate, all-time opponent profile — which is why it cannot window observations or notice an opponent changing gears. Those are limits of the *strategy*, not the *format*: AKG's node types and edge relations are an open vocabulary, `meta` is unconstrained, and a temporal index is built in. Richer profiling — windowed/recency-weighted reads, board-texture or sizing "spot" nodes, `contradicts`/`superseded_by` edges for invalidation — is all additive on top of the existing architecture, no rewrite at each step. The constraint going forward is not "can AKG represent this." It is "does feeding the LLM this richer structure make it play better, at acceptable cost" — a question only experiments answer, and one we have already seen cut the wrong way once (see `research.md`).

## Mirror match: a settled durability result

We seated two `llm-akg-durable` agents against each other for 500 hands (`mirror-match-500/mirror-sanity-1`). It was a **durability and instrumentation sanity check**, and it delivered:

- The agent maintained a multi-megabyte `.akg` file across all 500 hands with **full node coverage, no gaps, zero lost decisions, and no degradation as the file grew** — the durable strategy scales to long sessions.
- Both agents queried their opponent model nearly every hand and cited concrete tendencies in their reasoning, and the two memory files told a **mutually consistent story** of the same match from opposite seats, cross-validating to the digit on unambiguous stats.

What it explicitly did **not** show is a memory *edge*: in a mirror, both sides have identical memory by construction, so the chip result (one seat +4857) is variance plus emergent style divergence, not evidence that memory helps. Measuring lift requires a non-mirror matchup against the lower rungs of the ladder. And testing whether memory can detect *adaptation* requires a scripted opponent that changes gears at a known hand — mirror self-play, with both sides drifting at once, cannot attribute that cleanly.

## Multiplayer and concurrent agents

The current design is heads-up: one agent against one opponent per session. AKG-backed agents write to the server-provided `memory_dir`, so simultaneous sessions use isolated memory files and do not conflict. Future multiplayer work would need explicit product and protocol design and is not part of the current experiment-first v0 scope.

## The broader claim

Structured retrievable memory that agents can reason about — not just recall — is a general unlock for agent capability. The gap between a stateless LLM and a capable agent is not primarily a model capability gap. It is a memory architecture gap.

Poker is the proof of concept. The thesis generalizes.
