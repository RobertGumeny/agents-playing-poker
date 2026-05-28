# Vision: Agents That Remember and Reason

## The core insight

The biggest unlock for agent capability is not better prompts or bigger context windows. It is giving agents access to structured memory they can actually reason about.

Retrieval is necessary but not sufficient. An agent that can fetch a few relevant text chunks is doing glorified search. An agent that can traverse a knowledge structure — follow edges, identify patterns, reason about relationships between past events — is doing something qualitatively different. That is what this project is designed to demonstrate.

## Why poker

Poker is a controlled, adversarial, multi-session environment with a clean ground truth: chips. It produces a measurable outcome for every decision, every hand, every session. It requires modeling an opponent over time, updating beliefs as new evidence arrives, and acting under uncertainty.

That is exactly the domain where structured memory should matter most. An agent that remembers "villain 3-bet light three times from the button last session" should outperform one that does not. Whether it actually does, and by how much, is a question this project can answer precisely.

## Memory-strategy framing

Agent memory in this project is the variable under test. The core strategy lineup is:

- **`llm-stateless`** — current hand only; the low-token no-memory baseline.
- **`llm-fullhistory`** — raw or lightly formatted prior-hand history injected into the prompt; the naive high-token memory baseline.
- **`llm-akg-recent`** — shallow bounded AKG memory using recent hands plus a growing opponent profile; useful as a wiring and token-efficiency control.
- **`llm-akg-durable`** — durable structured AKG opponent memory with graph-backed opponent/profile/pattern evidence and decision-time retrieval tools; this is where the core thesis lives.

The thesis does not require AKG to win every short match. Poker outcomes are noisy: a few all-in pots, seat/order effects, blind rotation, LLM nondeterminism, invalid-action fallbacks, and ordinary variance can dominate short-run chip results. The target result is that durable AKG memory can match or beat full-history prompting over planned experiment sets while using materially less context growth and producing memory artifacts that can be inspected, queried, and improved.

A recent-hand AKG snapshot should therefore be labeled honestly. It can show that bounded recency changes behavior at lower token cost than full-history prompting, but it is not by itself proof that durable structured memory gives an agent a strategic edge.

## Experimental framing

The experiment is not only "which agent wins the most chips." The stronger claim is two-dimensional:

1. **Poker performance:** net chips, chips per hand / BB per hand, matchup results, and confidence intervals where enough samples exist.
2. **Context efficiency:** prompt/completion tokens per hand, tokens per decision, token-growth slope, cost per hand, and chips or BB won per token budget.

This matters because a raw full-history agent may win some chips simply by stuffing more context into the prompt. That is a useful baseline, but not the thesis. The AKG thesis is that structured memory should preserve or improve decision quality while keeping context bounded and inspectable.

The current operator workflow is experiment-first: define the comparison as JSON under `experiments/`, run it with `poker experiment go <experiment-id>`, and interpret the generated session artifacts plus `reports/<experiment-id>.md`.

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
