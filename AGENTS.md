# Agents Playing Poker — Agent Instructions

This file is the source of truth for any AI agent (Claude Code, doug, Codex, etc.) working in this repository. Other agent-config files (e.g. `CLAUDE.md`) import this one rather than duplicate its content.

## What this project is

A research harness in which multiple AI agents play heads-up no-limit Texas Hold'em against each other under identical rules and tools, differing only in their **memory strategy**. The goal is to produce a measurable, inspectable demonstration that durable structured memory (via [AKG](https://github.com/RobertGumeny/akg-format)) gives an LLM agent a competitive advantage over (a) no memory and (b) the naive "stuff history into the prompt" approach.

v0 is single-machine, single-operator, no public-facing tournament. Future versions add multiplayer / BYO-SDK / leaderboard, but those are explicitly out of scope right now.

## Read this first

Before doing any implementation work, read these sources in order:

1. **[`docs/vision.md`](docs/vision.md)** — the project thesis, scope, and memory-strategy progression
2. **[`docs/research.md`](docs/research.md)** — the current experimental setup, strategy lineup, metrics, and run conventions
3. **[`docs/domain/README.md`](docs/domain/README.md)** — the domain-doc index and rules split

There is no single monolithic spec anymore. Repository-specific contracts now live in the focused docs for each subsystem:

- [`docs/wire-protocol.md`](docs/wire-protocol.md) — server/agent JSONL protocol
- [`docs/llm-akg-durable-spec.md`](docs/llm-akg-durable-spec.md) — durable AKG agent contract
- [`docs/eval-system.md`](docs/eval-system.md) — eval and experiment-definition system
- [`docs/session-artifacts.md`](docs/session-artifacts.md) — stable additive session-artifact schemas for `memory-export.json` and `eval.json`
- [`docs/experiment-definition.md`](docs/experiment-definition.md) — normative JSON contract for planned experiment session sets
- [`docs/kb/README.md`](docs/kb/README.md) — implementation knowledge-base index

`docs/domain/` is the canonical source for poker-domain semantics and terminology used by the implementation. If a topic-specific contract is ambiguous or missing, surface that and propose an amendment to the relevant doc in the same change. Do not silently make a judgment call that quietly drifts from the documented design.

### Project vs. domain docs

Use this split consistently:

- **Project docs**: repository-specific technical decisions — architecture, wire protocol, session outputs, strategy lineup, scope boundaries, and any implementation policy unique to this repo.
- **`docs/domain/`**: Texas Hold'em game rules and terminology — flop/turn/river, blinds, hole cards, betting order, hand rankings, showdown concepts, and similar poker-domain truth that agents should not reinvent.

Rule of thumb:

- If it is true because of this repository's design, it belongs in a focused project doc under `docs/`.
- If it is true because it is part of Texas Hold'em itself, it belongs in `docs/domain/`.

When the code needs actual Hold'em rules, agents should consult `docs/domain/` rather than improvising from memory. If a project-specific behavior intentionally constrains or overrides the generic domain rules, the focused project doc for that subsystem wins for that case.

## Adjacent code you'll need to know about

- **`~/source/akg`** — the AKG format spec, Go reference implementation, and conformance corpus. This project consumes the Go AKG SDK (lives in or near that repo) and, later, a TypeScript AKG SDK. When in doubt about how AKG behaves, read the spec at `~/source/akg/docs/spec/` and the reference implementation at `~/source/akg/internal/`.
- **`~/source/doug`** — a Go-based CLI orchestrator built on the Pi harness. Not a dependency of this project, but conceptually adjacent: doug also drives Pi child sessions and will eventually share the TS AKG SDK and Pi compaction extension we build here.
- **[pi.dev/docs/latest](https://pi.dev/docs/latest)** — Pi harness docs. Particularly relevant: RPC mode (stdin/stdout JSONL) and the `session_before_compact` extension hook.

## Working conventions

### Scope discipline

The single most important rule. Keep changes within the scope described by `docs/vision.md`, `docs/research.md`, and the focused subsystem docs. Do not:

- Build features not in the current docs because they "seem obvious."
- Add abstractions for hypothetical future requirements.
- Pre-build for 6-max, multiplayer, leaderboards, or any other future thing unless a current doc explicitly calls for it.

If something feels missing from the docs, that's a signal to surface a question or propose an amendment — not to invent.

### Go style

- Standard `gofmt`. No bikeshedding.
- Prefer the standard library. Add a dependency only when it materially saves work and the dependency is well-maintained.
- Table-driven tests for anything with non-trivial branching (rules engine especially).
- Keep packages small and single-purpose. Internal packages (`internal/...`) are not part of any public API.
- Error wrapping with `fmt.Errorf("...: %w", err)`. No `panic` outside of `main` / truly unrecoverable startup conditions.
- No comments explaining what code does — only why, and only when the why is non-obvious. Identifiers should carry the load.

### Determinism

The game server is deterministic given a seed. The rules engine, dealer, and match orchestrator must be pure functions of state + seed wherever feasible. LLM agents are not deterministic (temperature, model nondeterminism); that's fine and is part of the research framing.

### Tests

- Rules engine: extensive table-driven tests for hand evaluation, betting legality, pot accounting, blind rotation, showdown.
- Wire protocol: round-trip JSON tests for every message type, plus negative tests for malformed input.
- Match orchestrator: integration tests using `random` and `heuristic` agents (no LLMs) to verify a full session produces correct `hands.jsonl` and `manifest.json`.

LLM-agent behavior is not unit-tested (nondeterministic); validate it by running matches and inspecting the artifacts.

### Commits

- Conventional commit-ish prefixes (`feat:`, `fix:`, `chore:`, `docs:`, `test:`, `refactor:`) — match the AKG repo's style.
- One logical change per commit. A "wire up the rules engine" commit is fine; "wire up rules engine + rename a thing in match + fix a typo" is three commits.
- Never `git commit` without an explicit ask from the user.

## Repo layout

```
engine/        — all implementation code (Go + TypeScript)
  cmd/         — CLI binaries
  internal/    — Go packages
  pi-agents/   — TypeScript LLM agent implementations
  go.mod       — Go module root (run all go commands from engine/)
docs/          — project and domain documentation
research/      — experiment definitions, session artifacts, comparison reports
  experiments/ — one subdir per experiment slug; each contains the JSON manifest, sessions/, and reports/
  sessions/    — unclaimed session outputs not yet linked to an experiment (gitignored)
```

All `go run`, `go build`, and `go test` commands must be run from `engine/`.

## How to start

The original v0 build phasing is complete through the non-LLM demo and LLM baseline layers. For current experimental posture — what has been run, what the active thesis strategy is, and what's pending — read [`docs/research.md`](docs/research.md) and browse the checked-in experiment definitions under `research/experiments/`.

For new implementation work:

1. Read [`docs/kb/README.md`](docs/kb/README.md) and only the KB articles relevant to the task.
2. Read the focused contract for the subsystem you are touching, such as [`docs/wire-protocol.md`](docs/wire-protocol.md), [`docs/llm-akg-durable-spec.md`](docs/llm-akg-durable-spec.md), [`docs/eval-system.md`](docs/eval-system.md), [`docs/session-artifacts.md`](docs/session-artifacts.md), or [`docs/experiment-definition.md`](docs/experiment-definition.md).
3. Keep implementation and docs changes aligned in the same patch when behavior changes.

## When you're unsure

- About the format / AKG behavior → read `~/source/akg/docs/spec/`.
- About Texas Hold'em rules or terminology → read `docs/domain/` first.
- About the wire protocol → read `docs/wire-protocol.md`. If it doesn't answer your question, propose an amendment there.
- About scope or project behavior → assume it's out of scope unless the current docs explicitly include it. Ask the user.
- About a third-party library → keep it minimal. The Go stdlib goes very far for this project; reach for `card` / poker libraries only if hand evaluation actually becomes the bottleneck (it won't for v0).


<!-- DOUG-SPECIFIC-INSTRUCTIONS:START -->
<!-- Generated by doug init — project metadata below is managed automatically -->
DOUG_PROJECT_ID: agent-poker-32ae01
DOUG_PROJECT_NAME: Agent Poker

## Doug-Specific Instructions

<!-- Edit the rules below to reflect your repository's operating conventions -->
This section is managed by `doug init`. Keep repository-specific operating rules here, and keep task skills focused on their workflow.

### Progressive Disclosure

1. For doug-managed runs launched by `doug`, read `.doug/ACTIVE_TASK.md` for the active task brief.
2. Read `.doug/PRD.md` for product context and constraints when it is relevant to the task.
3. Read `docs/kb/README.md` for the knowledge base index.
4. Read only the KB articles relevant to the task at hand.

### Working Rules

- Only treat `.doug/ACTIVE_TASK.md` as the canonical task brief when the user request or launch prompt indicates a doug-managed run.
- In doug-managed runs, write your result directly into the `## Agent Result` block and summary sections at the bottom of `.doug/ACTIVE_TASK.md`.
- In doug-managed runs, `## Agent Result.outcome` must be exactly one of `SUCCESS`, `FAILURE`, `BUG`, or `EPIC_COMPLETE`.
- Do not depend on other internal doug control files. Only `.doug/ACTIVE_TASK.md` and `.doug/PRD.md` are part of the agent-facing contract.
- Use `.doug/ACTIVE_BUG.md` only for a blocking bug that interrupts the current runtime task and must hand context to a follow-up bugfix task.
- Write every bug report, including non-blocking and deferred findings, as a durable file under `.doug/logs/bugs/{epic}/` using `.doug/logs/BUG_REPORT_TEMPLATE.md`.
- If you find a bug that is outside the current task scope, report it in `.doug/logs/bugs/{epic}/` instead of fixing it opportunistically.
- Use `docs/kb/README.md` as the KB entrypoint instead of scanning the whole KB up front.
<!-- DOUG-SPECIFIC-INSTRUCTIONS:END -->
