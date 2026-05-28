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
go run ./cmd/poker-run -agent0 llm-akg-durable -agent1 llm-akg-recent -hands 200 -seed 3 -model anthropic:claude-sonnet-4-6
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

- supported aliases are `llm-stateless`, `llm-fullhistory`, `llm-akg-recent`, `llm-akg-durable`, `heuristic`, and `random`
- `llm-akg-recent` is the shallow AKG recency-memory baseline; `llm-akg-durable` is the active durable AKG retrieval agent
- `poker-run` resolves agent entrypoints to absolute paths before launch so agent subprocesses still work when `poker-server` sets each child `cmd.Dir` to its session directory
- for `llm-stateless`, `llm-fullhistory`, `llm-akg-recent`, and `llm-akg-durable`, the wrapper clears `PI_POKER_FAKE_DECISIONS_JSON`, defaults `PI_POKER_THINKING_LEVEL=low`, and forwards `-model` / `-thinking-level` into Pi agent env
- Go agents are built into `.tmp/bin/` on demand for wrapper runs

## Inspect the session bundle

The demo writes artifacts under `sessions/<id>/`:

- `manifest.json` — match metadata, seat versions, totals, and completion status
- `hands.jsonl` — one server-authoritative hand record per line
- `agents/<name>/stdout.log` and `agents/<name>/stderr.log` — per-agent process logs
- `agents/<name>/memory-export.json` — optional server-generated JSON snapshot for memory-capable agents when `memory.akg` is present; see [`docs/session-artifacts.md`](docs/session-artifacts.md)

Useful inspection commands:

```bash
jq . sessions/ses_demo_random_vs_heuristic/manifest.json
head -n 3 sessions/ses_demo_random_vs_heuristic/hands.jsonl
ls sessions/ses_demo_random_vs_heuristic/agents
```

## Run a planned experiment

Use `poker-eval` for the full experiment loop and `poker-run` as the single-session primitive underneath it.

- `poker-run` runs one session and writes `sessions/<id>/...`
- `poker-eval` plans many sessions from a checked-in experiment definition, inspects coverage, launches missing work through `poker-run`, collects `eval.json`, and compares control vs treatment

### 1. Create the experiment definition

Bootstrap a valid JSON experiment file:

```bash
go run ./cmd/poker-eval init \
  -out experiments/my-first-benchmark.json \
  -hypothesis "Bounded memory should beat stateless play at similar cost." \
  -control-agent llm-stateless \
  -control-opponent heuristic \
  -treatment-agent llm-akg-recent \
  -sessions-count 5
```

This writes `experiments/my-first-benchmark.json`, which must satisfy [`docs/experiment-definition.md`](docs/experiment-definition.md).

### 2. Discover experiment files and inspect coverage

```bash
go run ./cmd/poker-eval ls
go run ./cmd/poker-eval ls -experiments-dir experiments -sessions-dir sessions
```

`ls` scans experiment JSON files and reports planned, present, missing, and incomplete session counts.

For one experiment's detailed coverage:

```bash
go run ./cmd/poker-eval status -experiment experiments/test-2b-retrieval-throttle.json
```

Coverage is derived from primary session artifacts only:

- `present` — complete `manifest.json` plus matching `hands.jsonl`
- `missing` — no session directory yet
- `incomplete` — session directory exists but primary artifacts do not match the planned session

### 3. Preview the deterministic execution plan

```bash
go run ./cmd/poker-eval run -experiment experiments/test-2b-retrieval-throttle.json -dry-run
```

Dry-run prints the exact control/treatment session ids, seeds, agents, opponents, and target `sessions/<id>` directories without launching anything.

### 4. Run the planned sessions

```bash
go run ./cmd/poker-eval run \
  -experiment experiments/test-2b-retrieval-throttle.json \
  -model anthropic:claude-sonnet-4-6
```

Current execution semantics:

- `poker-eval run` expands the checked-in experiment definition deterministically
- `present` sessions are skipped
- `missing` and `incomplete` sessions are launched again
- each launched session is delegated to `poker-run`; `poker-eval` does not duplicate match-running logic
- `-model` and `-thinking-level` are runtime overrides forwarded to Pi-backed agents through `poker-run`
- any session that needs execution must have `opponent` metadata in the experiment definition

Each launched `poker-run` session writes the normal session bundle under `sessions/<id>/`:

- `manifest.json` — session and match summary
- `hands.jsonl` — server-authoritative hand log
- `report.md` — single-session markdown report generated after a completed run
- `agents/<name>/stdout.log` and `agents/<name>/stderr.log` — per-agent process logs
- `agents/<name>/pi-session.jsonl` — Pi transcript when that seat is Pi-backed
- `agents/<name>/memory.akg` — durable live AKG store for memory-capable agents
- `agents/<name>/memory-export.json` — optional teardown export when `memory.akg` exists; see [`docs/session-artifacts.md`](docs/session-artifacts.md)

If you need one-off ad hoc sessions instead of a planned experiment, call `poker-run` directly.

### 5. Collect offline summaries

After the planned sessions complete, collect normalized per-session summaries:

```bash
go run ./cmd/poker-eval collect \
  sessions/akg-durable-vs-stateless-test-{1..5} \
  sessions/akg-durable-retrieval-test-{1..5}
```

`collect` reads the session artifacts offline and writes `sessions/<id>/eval.json` for each named session. The stable summary contract lives in [`docs/session-artifacts.md`](docs/session-artifacts.md).

### 6. Compare treatment vs control

```bash
go run ./cmd/poker-eval compare -experiment experiments/test-2b-retrieval-throttle.json
```

`compare`:

- loads the checked-in experiment definition and planned session list
- requires collected `eval.json` for every planned session
- prints a markdown comparison report to stdout
- summarizes session, gameplay, and observed tool-use metrics
- evaluates any `expected_direction` checks from the experiment definition
- warns when observed group metadata is mixed, such as inconsistent derived opponents inside a group without explicit `opponent` metadata

If you want a file, redirect stdout:

```bash
go run ./cmd/poker-eval compare -experiment experiments/test-2b-retrieval-throttle.json \
  > reports/test-2b-retrieval-throttle.md
```

## Generate a benchmark report

Use `poker-report` to create a repeatable Markdown review from explicit session directories. The report is derived from each session's `manifest.json` and `hands.jsonl` artifacts, not from existing per-session `report.md` files.

```bash
go run ./cmd/poker-report \
  -sessions sessions/akg-recent-vs-stateless-seed1-a,sessions/akg-recent-vs-stateless-seed1-b,sessions/akg-recent-vs-stateless-seed2-a,sessions/akg-recent-vs-stateless-seed2-b \
  -label akg-recent-vs-stateless \
  -out reports/akg-recent-vs-stateless.md
```

Historical session artifacts may still contain the old shallow-memory strategy label `llm-akg`. Current reporting canonicalizes that label to `llm-akg-recent` and includes a warning note when normalization occurred; do not rewrite old session bundles just to rename historical artifacts.

Timeout behavior is server-enforced. If an agent exceeds `-decision-deadline`, the server records `action: "auto_fold"` with `forced_reason: "decision_timeout"` in `hands.jsonl` and still exits cleanly.

## Read

- **[`docs/vision.md`](docs/vision.md)** — project vision and thesis framing.
- **[`AGENTS.md`](AGENTS.md)** — instructions for AI agents working in this repo.
- **[`docs/wire-protocol.md`](docs/wire-protocol.md)** — the JSONL agent/server contract.
- **[`docs/session-artifacts.md`](docs/session-artifacts.md)** — stable additive session-artifact schemas for `memory-export.json` and `eval.json`.
