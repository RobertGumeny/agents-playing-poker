# Agents Playing Poker — Research State

**Authors:** Robert Gumeny  
**Repo:** [github.com/RobertGumeny/agent-poker](https://github.com/RobertGumeny/agent-poker)  
**Status:** Active research

## Thesis

Can structured, retrievable memory make an LLM agent measurably better at poker?

The specific claim:

> Given the same model, same game rules, same tools, and the same deal sequence, an LLM agent backed by durable structured [AKG](https://github.com/RobertGumeny/akg) memory should match or outperform an agent that dumps the full hand history into its context, while using materially fewer tokens — and both memory strategies should outperform an agent with no memory at all.

Poker is a good thesis vehicle because it demands two things that memory directly addresses: **opponent modeling** and **pattern recognition**. It also produces objective outcome data: chips, hands, decisions, costs, and artifacts.

## Current workflow

Research runs are experiment-first.

1. Create or edit a JSON experiment definition under [`experiments/`](../experiments/).
2. Run it with:

   ```bash
   go run ./cmd/poker experiment go <experiment-id>
   ```

3. Inspect:
   - `reports/<experiment-id>.md` for the treatment/control comparison
   - `sessions/<session-id>/manifest.json` and `hands.jsonl` for primary session truth
   - `sessions/<session-id>/eval.json` for derived per-session metrics
   - agent artifacts such as `pi-session.jsonl`, `memory.akg`, and `memory-export.json`

The root experiment command exposes the same artifact chain as smaller steps:

| Command | Purpose |
| --- | --- |
| `poker experiment status <id>` | Show planned, present, missing, and incomplete sessions. |
| `poker experiment run <id>` | Run only missing or incomplete planned sessions. |
| `poker experiment analyze <id>` | Collect missing `eval.json` files and write `reports/<id>.md`. |
| `poker experiment go <id>` | Run missing work and then analyze it. |

The experiment JSON contract is defined in [`experiment-definition.md`](experiment-definition.md). Session artifact schemas are defined in [`session-artifacts.md`](session-artifacts.md).

## Game setup

The current research harness runs heads-up no-limit Texas Hold'em.

- **Players:** 2
- **Starting stack:** 200bb by default in match runners unless overridden
- **Blinds:** 1/2 by default
- **Information realism:** showdown-only; opponent hole cards are visible only when the hand reaches showdown
- **Deal sequence:** deterministic by seed
- **Rules authority:** the server/rules engine, not the agents

The same experiment definition pins hand count, seeds, strategies, and model. This makes a treatment/control comparison reproducible from the checked-in plan.

## Strategy lineup

All LLM agents in a benchmark should use the same model and runtime settings unless those settings are the explicit variable under test. The strategy difference should be memory policy.

### `llm-stateless`

No strategic memory. Each decision sees only the current hand state: hole cards, board, pot, stacks, current-hand action history, and legal actions.

This is the low-token floor. Any memory strategy should beat it over enough hands if opponent modeling matters.

### `llm-fullhistory`

Naive memory baseline. Before each decision, prior completed hands are serialized into prompt context. This gives the model broad access to past events, but the prompt grows with the match and mixes relevant and irrelevant history.

This is the high-token benchmark that durable structured memory is ultimately meant to match or beat.

### `llm-akg-recent`

Shallow bounded AKG memory. The agent writes an opponent profile and recent hand summaries into `memory.akg`, then injects a bounded memory summary at decision time.

This is useful as a smoke test for durable memory wiring and token efficiency, but it should not be treated as the final proof of the structured-memory thesis.

### `llm-akg-durable`

Durable structured AKG memory. The agent stores completed hands, an opponent profile, pattern nodes, and evidence edges. During decisions, the model can query AKG through read-only tools before returning a legal poker action.

This is the primary thesis strategy: memory should be cumulative, bounded at prompt time, inspectable, and relevant to the current decision.

### `random` and `heuristic`

Scripted non-LLM sanity-check baselines. They are useful for validating protocol, rules, artifacts, and local execution without model calls. They are not the primary research comparison for memory strategy claims.

## Metrics and interpretation

Short poker runs are noisy. Treat single-session chip outcomes as diagnostics, not proof. Prefer planned treatment/control experiment sets with multiple seeds and, where useful, mirrored seat assignments.

Primary result dimensions:

- **Poker performance**
  - chip delta
  - chips per hand / BB per hand
  - per-session and aggregate treatment/control deltas
  - showdown and non-showdown behavior where available
- **Context and cost efficiency**
  - tokens per hand
  - tokens per decision
  - prompt-growth slope
  - cost per hand
  - chips or BB per token/cost budget
- **Reliability and process quality**
  - malformed action retries
  - fallback actions
  - decision prompt counts
  - tool-use rates for AKG agents
- **Memory quality**
  - memory node/edge counts from `memory-export.json`
  - pattern availability and evidence counts
  - whether queried evidence is relevant to decision spots

The main interpretive question is not whether AKG wins one short match. It is whether durable structured memory improves or preserves performance while bounding prompt growth and producing an inspectable reasoning substrate.

## Current artifact model

A completed planned session writes under `sessions/<session-id>/`.

Authoritative session records:

- `manifest.json`
- `hands.jsonl`

Derived or agent-side records:

- `report.md`
- `eval.json`
- `agents/<name>/pi-session.jsonl`
- `agents/<name>/stderr.log`
- `agents/<name>/memory.akg`
- `agents/<name>/memory-export.json`

`manifest.json` and `hands.jsonl` are the primary truth for game results. `eval.json`, `report.md`, and `memory-export.json` are additive analysis artifacts and can be regenerated or extended without changing match outcomes.

## Current research posture

Use `llm-stateless`, `llm-fullhistory`, `llm-akg-recent`, and `llm-akg-durable` to separate three questions:

1. Does memory help over no memory?
2. Does raw full-history prompting help enough to justify its token growth?
3. Can durable structured AKG memory match or beat raw history while staying bounded and inspectable?

The strongest evidence will come from checked-in experiment definitions, reproducible seeds, generated reports, and artifact-level review — not from one-off procedural run notes.
