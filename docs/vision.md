# Vision: Agents That Remember and Reason

## The core insight

The biggest unlock for agent capability is not better prompts or bigger context windows. It is giving agents access to structured memory they can actually reason about.

Retrieval is necessary but not sufficient. An agent that can fetch a few relevant text chunks is doing glorified search. An agent that can traverse a knowledge structure — follow edges, identify patterns, reason about relationships between past events — is doing something qualitatively different. That is what this project is designed to demonstrate.

## Why poker

Poker is a controlled, adversarial, multi-session environment with a clean ground truth: chips. It produces a measurable outcome for every decision, every hand, every session. It requires modeling an opponent over time, updating beliefs as new evidence arrives, and acting under uncertainty.

That is exactly the domain where structured memory should matter most. An agent that remembers "villain 3-bet light three times from the button last session" should outperform one that does not. Whether it actually does, and by how much, is a question this project can answer precisely.

## Memory-strategy framing

Agent memory in this project is the variable under test. The current strategy lineup is:

- **`llm-stateless`** — current hand only; the low-token no-memory baseline.
- **`llm-fullhistory`** — raw or lightly formatted prior-hand history injected into the prompt; the naive high-token memory baseline.
- **`llm-akg-recent`** — shallow bounded AKG memory using recent hands plus a growing opponent profile; useful as a wiring and token-efficiency control.
- **`llm-akg-durable`** — durable structured AKG opponent memory with graph-backed opponent/profile/pattern evidence and decision-time retrieval tools; this is where the core thesis lives.

Planned future strategies that would sharpen the comparison:

- **`llm-md-single`** — a single growing markdown file that the agent rewrites or appends after each hand. The "obvious first thing a developer would build." Honest representation of the simplest possible durable memory.
- **`llm-md-wiki`** — multiple markdown files organized by topic (`opponent-profile.md`, `patterns.md`, `hand-log.md`), read selectively. Same class as `llm-akg-durable` (selective durable memory, decision-time retrieval) but flat files instead of a typed graph. The most direct apples-to-apples test of whether AKG's structure does work that ad-hoc file organization cannot.

The thesis does not require AKG to win every short match. Poker outcomes are noisy: a few all-in pots, seat/order effects, blind rotation, LLM nondeterminism, invalid-action fallbacks, and ordinary variance can dominate short-run chip results. The target result is that durable AKG memory can match or beat full-history prompting over planned experiment sets while using materially less context growth and producing memory artifacts that can be inspected, queried, and improved.

A recent-hand AKG snapshot should therefore be labeled honestly. It can show that bounded recency changes behavior at lower token cost than full-history prompting, but it is not by itself proof that durable structured memory gives an agent a strategic edge.

## Experimental framing

The experiment is not only "which agent wins the most chips." The stronger claim is two-dimensional:

1. **Poker performance:** net chips, chips per hand / BB per hand, matchup results, and confidence intervals where enough samples exist.
2. **Context efficiency:** prompt/completion tokens per hand, tokens per decision, token-growth slope, cost per hand, and chips or BB won per token budget.

This matters because a raw full-history agent may win some chips simply by stuffing more context into the prompt. That is a useful baseline, but not the thesis. The AKG thesis is that structured memory should preserve or improve decision quality while keeping context bounded and inspectable.

The current operator workflow is experiment-first: define the comparison as JSON under `experiments/`, run it with `poker experiment go <experiment-id>`, and interpret the generated session artifacts plus `reports/<experiment-id>.md`.

## Mirror match: two memory agents against each other

A natural future experiment is to seat two `llm-akg-durable` agents against each other for 500+ hands. Both agents build opponent models simultaneously — each modeling a villain who is also adapting in response to being modeled.

This creates dynamics that a stateless opponent cannot produce:

- **Adaptation arms race.** If one agent builds a `folds-to-cbet` pattern and starts barreling more aggressively, the other agent's fold rate should drop — which should eventually invalidate the pattern. Does the AKG store clean it up? Does the exploiting agent notice and re-adjust?
- **Convergence or divergence.** Do two agents with identical memory architectures converge toward equilibrium over 500 hands, or does whichever agent builds accurate patterns *faster* in the early window pull ahead and hold that edge?
- **Dual artifact analysis.** Each agent produces its own `memory.akg` at the end — two models of the same match from opposite seats. Comparing them tests whether the memory captures reality accurately, not just selectively.

The infrastructure is already in place: each seat runs as an independent Pi subprocess with its own `memory_dir`. A mirror match is likely just `poker-run -agent0 llm-akg-durable -agent1 llm-akg-durable` with a small extension to the experiment definition format to express symmetric agent configurations. The first step is a 25-hand sanity run to confirm both memory files diverge correctly.

## Why AKG and not markdown files

Markdown works. It is readable, writable, and easy to produce. It is also fragile: it grows noisy, lacks queryable structure, resists deterministic retrieval, and degrades as a reasoning substrate as it scales.

AKG is a compact graph format designed to be stored anywhere, queried deterministically, and extended cleanly. It is not a document format that happens to contain facts — it is a fact store that happens to be readable. That distinction matters when the agent's task is to reason over a knowledge structure rather than summarize a document.

The choice is not about elegance. It is about building a substrate that can scale from "inject a few opponent stats" to "traverse a multi-session knowledge graph and reason about patterns" without a rewrite at each step.

## Multiplayer and concurrent agents

The current design is heads-up: one agent against one opponent in each session. AKG-backed agents write to the server-provided `memory_dir`, so simultaneous sessions use isolated memory files and do not share or conflict with one another.

Future multiplayer work would need explicit product and protocol design. It is not part of the current experiment-first v0 scope.

## The broader claim

Structured retrievable memory that agents can reason about — not just recall — is a general unlock for agent capability. The gap between a stateless LLM and a capable agent is not primarily a model capability gap. It is a memory architecture gap.

Poker is the proof of concept. The thesis generalizes.
