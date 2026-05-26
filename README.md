# Agent Poker

A research harness in which AI agents play no-limit Texas Hold'em against each other under identical rules and tools, differing only in their **memory strategy**.

The goal is to produce a measurable, inspectable demonstration that durable structured memory (via [AKG](https://github.com/RobertGumeny/akg-format)) gives an LLM agent a competitive advantage over no memory and over the naive "stuff history into the prompt" approach.

## Status

v0 is in progress. Build-order steps 1–4 are implemented: the rules engine, wire protocol, `poker-server`, and the scripted `random` / `heuristic` Go agents.

## Run the step-4 demo

This wrapper is now the primary supported way to run build-order step 4.

From the repo root, run the supported one-command scripted demo:

```bash
go run ./cmd/poker-demo
```

The wrapper builds the shipped Go binaries into a temporary directory, launches the default `random` versus `heuristic` match through `poker-server`, and prints the generated session bundle location plus the canonical artifacts to inspect next, for example:

```text
demo=random-vs-heuristic session_dir=/abs/path/to/repo/sessions/ses_2026-05-22T12-00-00Z
inspect_next: manifest=/abs/path/to/repo/sessions/ses_2026-05-22T12-00-00Z/manifest.json
inspect_next: hands=/abs/path/to/repo/sessions/ses_2026-05-22T12-00-00Z/hands.jsonl
inspect_next: agent_logs=/abs/path/to/repo/sessions/ses_2026-05-22T12-00-00Z/agents
```

Execution constraints:

- the wrapper requires a working Go toolchain because it compiles `poker-server`, `random-agent`, and `heuristic-agent` on demand
- `go` is used by default; override it with `-go-bin` if you need a different Go executable
- the wrapper keeps `poker-server` as the underlying primitive, so the session bundle and server behavior stay identical to a manual run

Useful supported overrides:

```bash
go run ./cmd/poker-demo \
  -session-id ses_demo_random_vs_heuristic \
  -sessions-dir sessions \
  -match-id mat_demo \
  -seed 17 \
  -hand-count 200
```

If you need lower-level debugging, custom seat wiring, or to run non-default agents directly, `poker-server` remains available as the escape hatch with explicit seat command wiring.

## Run convenient real sessions

For operator convenience, use the higher-level `poker-run` wrapper when you want real matches without remembering long `poker-server` invocations.

Examples:

```bash
go run ./cmd/poker-run -agent0 llm-fullhistory -agent1 heuristic -hands 25
```

```bash
go run ./cmd/poker-run -agent0 llm-fullhistory -agent1 heuristic -hands 50 -model anthropic:claude-sonnet-4-6
```

```bash
go run ./cmd/poker-run -agent0 llm-stateless -agent1 random -hands 100 -model anthropic:claude-sonnet-4-6
```

```bash
go run ./cmd/poker-run -agent0 llm-akg-recent -agent1 llm-stateless -hands 200 -seed 3 -model anthropic:claude-sonnet-4-6
```

```bash
go run ./cmd/poker-run \
  -agent0 heuristic \
  -agent1 random \
  -hands 200 \
  -seed 17 \
  -session-id ses_scripted_match \
  -sessions-dir sessions
```

Notes:

- supported aliases are `llm-stateless`, `llm-fullhistory`, `llm-akg-recent`, `heuristic`, and `random`
- `llm-akg-recent` is the current shallow AKG recency-memory baseline; reserve names such as `llm-akg-durable` for future durable AKG strategies
- `poker-run` resolves agent entrypoints to absolute paths before launch so agent subprocesses still work when `poker-server` sets each child `cmd.Dir` to its session directory
- for `llm-stateless`, `llm-fullhistory`, and `llm-akg-recent`, the wrapper clears `PI_POKER_FAKE_DECISIONS_JSON`, defaults `PI_POKER_THINKING_LEVEL=low`, and forwards `-model` / `-thinking-level` into Pi agent env
- Go agents are built into `.tmp/bin/` on demand for wrapper runs

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
