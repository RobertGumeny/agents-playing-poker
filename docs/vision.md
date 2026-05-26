# Vision: Agents That Remember and Reason

## The core insight

The biggest unlock for agent capability is not better prompts or bigger context windows. It is giving agents access to structured memory they can actually reason about.

Retrieval is necessary but not sufficient. An agent that can fetch a few relevant text chunks is doing glorified search. An agent that can traverse a knowledge structure — follow edges, identify patterns, reason about relationships between past events — is doing something qualitatively different. That is what this project is designed to demonstrate.

## Why poker

Poker is a controlled, adversarial, multi-session environment with a clean ground truth: chips. It produces a measurable outcome for every decision, every hand, every session. It requires modeling an opponent over time, updating beliefs as new evidence arrives, and acting under uncertainty.

That is exactly the domain where structured memory should matter most. An agent that remembers "villain 3-bet light three times from the button last session" should outperform one that does not. Whether it actually does, and by how much, is a question this project can answer precisely.

## The AKG progression

Agent memory in this project is not a fixed feature — it is a design space with a natural layered progression.

**v0 (current, `llm-akg-recent`):** Programmatic writes, LLM reads. The current CLI agent `llm-akg-recent` writes a growing opponent profile and recent hand summaries into an AKG graph, then injects those bounded facts at decision time. This is a shallow memory baseline and cost-control phase, not the final AKG thesis agent.

**Near term:** Durable, situation-aware retrieval. The next strategy should store and retrieve opponent tendencies by street, position, action type, bet sizing, fold/call/raise response, and showdown evidence. Queries like "find hands where villain 3-bet and I held a drawing hand" or "how has villain responded to turn pressure after calling flop?" let the agent pull targeted historical evidence rather than recency-weighted summaries.

**Medium term:** Agent-authored nodes. The LLM writes its own synthesized observations back into the graph: "villain appears to slow-play sets on wet boards," "villain's river bet sizing is polarized." A theory of mind, persisted across sessions, authored by the agent itself.

**Long term:** Edges and patterns. Hands become instances of named patterns. Patterns accumulate win-rate statistics. The agent reasons over a knowledge structure — "this situation matches pattern X, which has been profitable 70% of the time against this opponent type" — rather than scanning a list of past events.

## Experimental framing

The experiment is not only "which agent wins the most chips." Short poker sessions are noisy: a few all-in pots, seat/order effects, blind rotation, LLM nondeterminism, invalid-action fallbacks, and ordinary variance can dominate any memory-strategy signal. Early runs should therefore be treated as exploratory unless they are paired, mirrored, and aggregated across enough hands.

The stronger claim is two-dimensional:

1. **Poker performance:** net chips, BB/100, and matchup results.
2. **Context efficiency:** prompt/completion tokens per hand, tokens per decision, token-growth slope, and chips or BB won per token budget.

This matters because a raw full-history agent may win some chips simply by stuffing more context into the prompt. That is a useful baseline, but not the thesis. The AKG thesis is that structured memory should preserve or improve decision quality while keeping context bounded and inspectable.

A healthy strategy lineup should distinguish these conditions clearly:

- **`llm-stateless`:** current hand only; the low-token baseline.
- **`llm-fullhistory`:** raw or lightly formatted history stuffed into the prompt; the naive high-token memory baseline.
- **`llm-akg-recent` / `llm-akg-recent5`:** a shallow recency-memory smoke test; useful for wiring and token-efficiency comparison, but not sufficient proof of durable structured memory.
- **`llm-akg-durable`:** cumulative structured opponent model with evidence counts, tendencies, bet-sizing patterns, showdown evidence, and retrieved beliefs across all prior hands/sessions; this is where the core thesis lives.

A recent-hand AKG snapshot is therefore not a failed experiment, but it must be labeled honestly. It proves, at most, that a bounded recency layer can change behavior at a lower token cost than full-history prompting. It does not by itself prove that durable structured memory gives an agent a strategic edge. Its value is as a phase-0 control and implementation smoke test that motivates the next step: cumulative, inspectable opponent memory.

The desired progression is:

1. Establish the stateless baseline.
2. Run shallow bounded-memory AKG (`llm-akg-recent`) against stateless across more seeds as a low-cost recency and token-efficiency control.
3. Use `llm-fullhistory` sparingly as an expensive calibration baseline while the AKG strategy is still shallow.
4. Replace the shallow snapshot with durable, situation-aware AKG opponent modeling.
5. Pit the durable AKG strategy against `llm-fullhistory` as the final-boss benchmark.
6. Compare not only chip totals, but BB/100, tokens per hand, prompt growth over time, and BB per token.

The target result is not "AKG wins every short match," and the current recency agent should not carry the full thesis burden. The target result is that durable AKG memory can match or beat full-history prompting with materially less context growth and with a memory artifact that can be inspected, queried, and improved.

## Why AKG and not markdown files

Markdown works. It is readable, writable, and easy to produce. It is also fragile: it grows noisy, lacks queryable structure, resists deterministic retrieval, and degrades as a reasoning substrate as it scales.

AKG is a compact binary graph format designed to be stored anywhere, queried deterministically, and extended cleanly. It is not a document format that happens to contain facts — it is a fact store that happens to be readable. That distinction matters when the agent's task is to reason over a knowledge structure rather than summarize a document.

The choice is not about elegance. It is about building a substrate that can scale from "inject a few opponent stats" to "traverse a multi-session knowledge graph and reason about patterns" without a rewrite at each step.

## Multiplayer and concurrent agents

The current v0 design assumes heads-up: one AKG agent, one opponent, one singleton "villain" node. But the architecture already supports concurrent sessions safely — each session writes to its own isolated `memory_dir`, so multiple AKG agents running simultaneously never share or conflict on the same memory file.

The more interesting extension is multi-player (6-max and beyond). Each agent at the table would maintain its own AKG graph, scoped to its own perspective. Opponent profile nodes would be keyed by seat rather than a singleton, letting each agent independently build a model of every other player at the table. No shared state, no coordination required — each agent's knowledge is its own.

This is another place where the binary-file-on-disk design pays off. Spinning up six isolated AKG agents in the same match is the same operation as spinning up one, repeated six times. The memory architecture does not need to change.

## The broader claim

Structured retrievable memory that agents can reason about — not just recall — is a general unlock for agent capability. The gap between a stateless LLM and a capable agent is not primarily a model capability gap. It is a memory architecture gap.

Poker is the proof of concept. The thesis generalizes.
