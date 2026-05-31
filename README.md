# Agents Playing Poker

![AI Agents Playing Poker in a Data Center](static/AgentsPlayingPoker.png)

Agents Playing Poker is a research harness, inspired by the classic painting "Dogs Playing Poker", for testing agentic memory strategies and patterns. The project runs deterministic heads-up no-limit Texas Hold'em matches between agents, coordinated by a Go game server. The cards, blinds, and rules are fixed by seed; the variable under test is the agent strategy, especially its memory strategy.

## Setup

Install the `poker` CLI once from the `engine/` directory:

```bash
cd engine && make install
```

This builds and places `poker` in `~/go/bin/` (which should already be on your PATH). All further commands use `poker` directly from the repo root.

## Quick start: no API key needed

```bash
poker demo
```

Plays a no-LLM random-vs-heuristic match and prints the session artifact paths. No API key or Node required.

## Quick start: run an experiment

Experiment definitions live in [`research/experiments/`](research/experiments/). Each file is a JSON plan for a control-vs-treatment comparison. Run from the repo root:

```bash
poker experiment go test-2b-retrieval-throttle
```

`poker experiment go` does the full operator loop:

1. Loads `research/experiments/<id>/<id>.json`.
2. Checks which planned sessions are already present.
3. Runs missing or incomplete sessions.
4. Collects per-session `eval.json` summaries.
5. Writes a comparison report to `research/experiments/<id>/reports/<id>.md`.

Use `--model` to override the model for Pi-backed LLM agents:

```bash
poker experiment go test-2b-retrieval-throttle --model anthropic:claude-sonnet-4-6
```

## Draft a new experiment

```bash
poker experiment new my-memory-test
```

Scaffolds `research/experiments/my-memory-test/my-memory-test.json` with sensible defaults. Edit the hypothesis, agent keys, and `expected_direction`, then run `poker experiment go my-memory-test`.

The normative schema is documented in [`docs/experiment-definition.md`](docs/experiment-definition.md).

## Experiment commands

| Command | Purpose |
| --- | --- |
| `poker experiment ls` | List all experiments and their session coverage. |
| `poker experiment status <id>` | Show planned, present, missing, and incomplete sessions. |
| `poker experiment new <id>` | Scaffold a new experiment definition for editing. |
| `poker experiment run <id>` | Run only the missing/incomplete sessions. |
| `poker experiment analyze <id>` | Collect `eval.json` files and write the comparison report. |
| `poker experiment go <id>` | Run + analyze in one shot. |

## Add a new memory strategy

```bash
poker strategy new llm-md-single
```

Scaffolds a typed TypeScript `MemoryPolicy` stub under `engine/pi-agents/llm-md-single/` and registers it in the strategy registry. Follow the printed next-steps to implement, build, and run it.

| Command | Purpose |
| --- | --- |
| `poker strategy ls` | List known strategies and whether they are built. |
| `poker strategy new <key>` | Scaffold a new TypeScript memory strategy. |

## Ad-hoc matches

```bash
poker match run --agent0 heuristic --agent1 random --hands 200 --seed 17
poker match run --agent0 llm-akg-durable --agent1 llm-stateless --hands 25 --model anthropic:claude-sonnet-4-6
```

Use these when you need a single non-repeatable match outside of an experiment plan.

## What is being compared?

Current agent strategies:

- `llm-stateless` — sees only the current hand.
- `llm-fullhistory` — injects prior hand history into the prompt. This grows with each hand.
- `llm-akg-recent` — shallow bounded AKG memory using recent hands and an opponent profile.
- `llm-akg-durable` — durable structured AKG opponent memory.
- `heuristic` and `random` — scripted non-LLM baselines.

The research claim is not just "which agent wins more chips." It is whether structured memory can improve or preserve poker performance while keeping context growth bounded and inspectable.

For the full framing, read [`docs/vision.md`](docs/vision.md) and [`docs/research.md`](docs/research.md).

## Outputs

Each session writes a bundle under `research/experiments/<id>/sessions/<session-id>/`:

- `manifest.json` — match metadata, agent names, chip totals, and completion status.
- `hands.jsonl` — server-authoritative hand log.
- `eval.json` — normalized per-session summary used by experiment reports.
- `report.md` — single-session report.
- `agents/<name>/stdout.log` and `stderr.log` — agent process logs.
- `agents/<name>/pi-session.jsonl` — Pi transcript for Pi-backed agents.
- `agents/<name>/memory.akg` — durable AKG memory file for memory-capable agents.
- `agents/<name>/memory-export.json` — optional JSON export of memory artifacts.

Artifact contracts are documented in [`docs/session-artifacts.md`](docs/session-artifacts.md).

## Important docs

- [`docs/vision.md`](docs/vision.md) — project thesis and memory-strategy framing.
- [`docs/research.md`](docs/research.md) — current research setup and interpretation guidance.
- [`docs/experiment-definition.md`](docs/experiment-definition.md) — experiment JSON contract.
- [`docs/session-artifacts.md`](docs/session-artifacts.md) — session artifact schemas.
- [`docs/wire-protocol.md`](docs/wire-protocol.md) — server/agent JSONL protocol.
- [`docs/domain/README.md`](docs/domain/README.md) — poker rules and terminology index.
- [`AGENTS.md`](AGENTS.md) — instructions for AI agents working in this repository.
