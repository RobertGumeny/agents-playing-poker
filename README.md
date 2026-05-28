# Agent Poker

Agent Poker is a research harness for testing whether LLM poker agents get better when they have durable, structured memory.

The project runs deterministic heads-up no-limit Texas Hold'em matches between agents. The cards, blinds, and rules are fixed by seed; the variable under test is the agent strategy, especially its memory strategy.

The main workflow is now experiment-first:

1. Write an experiment JSON file under `experiments/`.
2. Run it with `poker experiment go <experiment-id>`.
3. Inspect the generated session artifacts and Markdown report.

## What is being compared?

Current agent strategies include:

- `llm-stateless` — sees only the current hand.
- `llm-fullhistory` — injects prior hand history into the prompt.
- `llm-akg-recent` — shallow bounded AKG memory using recent hands and an opponent profile.
- `llm-akg-durable` — durable structured AKG opponent memory.
- `heuristic` and `random` — scripted non-LLM baselines.

The research claim is not just “which agent wins more chips.” It is whether structured memory can improve or preserve poker performance while keeping context growth bounded and inspectable.

For the full framing, read [`docs/vision.md`](docs/vision.md) and [`docs/research.md`](docs/research.md).

## Quick start: run an experiment

Experiment definitions live in [`experiments/`](experiments/). Each file is a JSON plan for a control-vs-treatment comparison.

Example:

```bash
go run ./cmd/poker experiment go test-2b-retrieval-throttle
```

If you have built the root `poker` binary, the same command is:

```bash
./poker experiment go test-2b-retrieval-throttle
```

`poker experiment go` does the full operator loop:

1. Loads `experiments/<experiment-id>.json`.
2. Checks which planned sessions are already present under `sessions/`.
3. Runs missing or incomplete sessions.
4. Collects per-session `eval.json` summaries.
5. Writes a comparison report under `reports/<experiment-id>.md`.

Use `-model` to override the model for Pi-backed LLM agents:

```bash
go run ./cmd/poker experiment go test-2b-retrieval-throttle \
  -model anthropic:claude-sonnet-4-6
```

If the experiment file already has a `model` field, that model is used unless overridden on the command line.

## Experiment file shape

A minimal experiment compares a `control` group against a `treatment` group:

```json
{
  "id": "my-memory-test",
  "hypothesis": "Durable AKG memory should improve chip efficiency against stateless play.",
  "model": "anthropic:claude-sonnet-4-6",
  "hands_per_session": 25,
  "control": {
    "session_base": "stateless-control",
    "sessions_count": 5,
    "agent": "llm-stateless",
    "opponent": "heuristic"
  },
  "treatment": {
    "session_base": "akg-durable-treatment",
    "sessions_count": 5,
    "agent": "llm-akg-durable",
    "opponent": "heuristic"
  },
  "expected_direction": {
    "chips_per_hand": "increase",
    "tokens_per_hand": "decrease"
  }
}
```

Save it as:

```text
experiments/my-memory-test.json
```

Then run:

```bash
go run ./cmd/poker experiment go my-memory-test
```

The normative schema is documented in [`docs/experiment-definition.md`](docs/experiment-definition.md).

## Experiment commands

The root CLI is organized around `poker experiment`:

| Command | Purpose |
| --- | --- |
| `poker experiment status <id>` | Show planned, present, missing, and incomplete sessions. |
| `poker experiment run <id>` | Run only the missing/incomplete sessions. |
| `poker experiment analyze <id>` | Collect `eval.json` files and write the comparison report. |
| `poker experiment go <id>` | Run missing work, collect summaries, and write the report. |

When using `go run`, prefix commands with `go run ./cmd/poker`, for example:

```bash
go run ./cmd/poker experiment status test-2b-retrieval-throttle
```

## Outputs

Each session writes a bundle under `sessions/<session-id>/`:

- `manifest.json` — match metadata, agent names, chip totals, and completion status.
- `hands.jsonl` — server-authoritative hand log.
- `eval.json` — normalized per-session summary used by experiment reports.
- `report.md` — single-session report.
- `agents/<name>/stdout.log` and `stderr.log` — agent process logs.
- `agents/<name>/pi-session.jsonl` — Pi transcript for Pi-backed agents.
- `agents/<name>/memory.akg` — durable AKG memory file for memory-capable agents.
- `agents/<name>/memory-export.json` — optional JSON export of memory artifacts.

Experiment comparison reports are written to:

```text
reports/<experiment-id>.md
```

Artifact contracts are documented in [`docs/session-artifacts.md`](docs/session-artifacts.md).

## Smoke tests and lower-level commands

The experiment workflow is the normal path. These commands are still useful when debugging.

Run a no-LLM scripted smoke test:

```bash
go run ./cmd/poker-demo
```

Run one ad hoc match directly:

```bash
go run ./cmd/poker-run -agent0 heuristic -agent1 random -hands 200 -seed 17
```

Run one LLM match directly:

```bash
go run ./cmd/poker-run \
  -agent0 llm-akg-durable \
  -agent1 llm-stateless \
  -hands 25 \
  -model anthropic:claude-sonnet-4-6
```

Use these direct commands only when you do not need a planned, repeatable experiment.

## Important docs

- [`docs/vision.md`](docs/vision.md) — project thesis and memory-strategy framing.
- [`docs/research.md`](docs/research.md) — current research setup and interpretation guidance.
- [`docs/experiment-definition.md`](docs/experiment-definition.md) — experiment JSON contract.
- [`docs/session-artifacts.md`](docs/session-artifacts.md) — session artifact schemas.
- [`docs/wire-protocol.md`](docs/wire-protocol.md) — server/agent JSONL protocol.
- [`docs/domain/README.md`](docs/domain/README.md) — poker rules and terminology index.
- [`AGENTS.md`](AGENTS.md) — instructions for AI agents working in this repository.
