# Agent Poker

A research harness in which AI agents play no-limit Texas Hold'em against each other under identical rules and tools, differing only in their **memory strategy**.

The goal is to produce a measurable, inspectable demonstration that durable structured memory (via [AKG](https://github.com/RobertGumeny/akg-format)) gives an LLM agent a competitive advantage over no memory and over the naive "stuff history into the prompt" approach.

## Status

v0 is in progress. Build-order steps 1–4 are implemented: the rules engine, wire protocol, `poker-server`, and the scripted `random` / `heuristic` Go agents.

## Run the step-4 demo

From the repo root, run the supported one-command scripted demo:

```bash
go run ./cmd/poker-demo
```

The wrapper launches the default `random` versus `heuristic` match through `poker-server` and prints the generated session bundle path, for example:

```text
demo=random-vs-heuristic session_dir=/abs/path/to/repo/sessions/ses_2026-05-22T12-00-00Z
inspect: jq . /abs/path/to/repo/sessions/ses_2026-05-22T12-00-00Z/manifest.json
```

Useful supported overrides:

```bash
go run ./cmd/poker-demo \
  -session-id ses_demo_random_vs_heuristic \
  -sessions-dir sessions \
  -match-id mat_demo \
  -seed 17 \
  -hand-count 200
```

If you need the low-level primitive directly, `poker-server` still supports explicit seat command wiring.

## Inspect the session bundle

The demo writes artifacts under `sessions/<id>/`:

- `manifest.json` — match metadata, seat versions, totals, and completion status
- `hands.jsonl` — one server-authoritative hand record per line
- `agents/<name>/stdout.log` and `agents/<name>/stderr.log` — per-agent process logs

Useful inspection commands:

```bash
jq . sessions/ses_demo_random_vs_heuristic/manifest.json
head -n 3 sessions/ses_demo_random_vs_heuristic/hands.jsonl
ls sessions/ses_demo_random_vs_heuristic/agents
```

Timeout behavior is server-enforced. If an agent exceeds `-decision-deadline`, the server records `action: "auto_fold"` with `forced_reason: "decision_timeout"` in `hands.jsonl` and still exits cleanly.

## Read

- **[`docs/spec.md`](docs/spec.md)** — the v0 specification (architecture, wire protocol, strategy lineup, output format, build phasing).
- **[`AGENTS.md`](AGENTS.md)** — instructions for AI agents working in this repo.
- **[`docs/wire-protocol.md`](docs/wire-protocol.md)** — the JSONL agent/server contract.
