# Vision: Agents That Remember and Reason

## The core insight

The biggest unlock for agent capability is not better prompts or bigger context windows. It is giving agents access to structured memory they can actually reason about.

Retrieval is necessary but not sufficient. An agent that can fetch a few relevant text chunks is doing glorified search. An agent that can traverse a knowledge structure — follow edges, identify patterns, reason about relationships between past events — is doing something qualitatively different. That is what this project is designed to demonstrate.

## Why poker

Poker is a controlled, adversarial, multi-session environment with a clean ground truth: chips. It produces a measurable outcome for every decision, every hand, every session. It requires modeling an opponent over time, updating beliefs as new evidence arrives, and acting under uncertainty.

That is exactly the domain where structured memory should matter most. An agent that remembers "villain 3-bet light three times from the button last session" should outperform one that does not. Whether it actually does, and by how much, is a question this project can answer precisely.

## The AKG progression

Agent memory in this project is not a fixed feature — it is a design space with a natural layered progression.

**v0 (current):** Programmatic writes, LLM reads. The server writes opponent profiling data and recent hand summaries into an AKG graph. At decision time, the relevant nodes are injected into the agent's context. The LLM reads structured facts; it does not author them.

**Near term:** Richer retrieval. Semantic queries over the graph — "find hands where villain 3-bet and I held a drawing hand" — let the agent pull targeted historical evidence rather than recency-weighted summaries.

**Medium term:** Agent-authored nodes. The LLM writes its own synthesized observations back into the graph: "villain appears to slow-play sets on wet boards," "villain's river bet sizing is polarized." A theory of mind, persisted across sessions, authored by the agent itself.

**Long term:** Edges and patterns. Hands become instances of named patterns. Patterns accumulate win-rate statistics. The agent reasons over a knowledge structure — "this situation matches pattern X, which has been profitable 70% of the time against this opponent type" — rather than scanning a list of past events.

## Why AKG and not markdown files

Markdown works. It is readable, writable, and easy to produce. It is also fragile: it grows noisy, lacks queryable structure, resists deterministic retrieval, and degrades as a reasoning substrate as it scales.

AKG is a compact binary graph format designed to be stored anywhere, queried deterministically, and extended cleanly. It is not a document format that happens to contain facts — it is a fact store that happens to be readable. That distinction matters when the agent's task is to reason over a knowledge structure rather than summarize a document.

The choice is not about elegance. It is about building a substrate that can scale from "inject a few opponent stats" to "traverse a multi-session knowledge graph and reason about patterns" without a rewrite at each step.

## The broader claim

Structured retrievable memory that agents can reason about — not just recall — is a general unlock for agent capability. The gap between a stateless LLM and a capable agent is not primarily a model capability gap. It is a memory architecture gap.

Poker is the proof of concept. The thesis generalizes.
