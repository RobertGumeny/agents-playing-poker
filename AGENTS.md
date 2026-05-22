# Agents Playing Poker — Agent Instructions

This file is the source of truth for any AI agent (Claude Code, doug, Codex, etc.) working in this repository. Other agent-config files (e.g. `CLAUDE.md`) import this one rather than duplicate its content.

## What this project is

A research harness in which multiple AI agents play heads-up no-limit Texas Hold'em against each other under identical rules and tools, differing only in their **memory strategy**. The goal is to produce a measurable, inspectable demonstration that durable structured memory (via [AKG](https://github.com/RobertGumeny/akg-format)) gives an LLM agent a competitive advantage over (a) no memory and (b) the naive "stuff history into the prompt" approach.

v0 is single-machine, single-operator, no public-facing tournament. Future versions add multiplayer / BYO-SDK / leaderboard, but those are explicitly out of scope right now.

## Read this first

Before doing any implementation work, read these two sources in order:

1. **[`docs/spec.md`](docs/spec.md)** — the authoritative v0 technical specification
2. **[`docs/domain/README.md`](docs/domain/README.md)** — the domain-doc index and rules split

`docs/spec.md` is the technical contract for this repository. `docs/domain/` is the canonical source for poker-domain semantics and terminology used by the implementation.

The authoritative specification for v0 is **[`docs/spec.md`](docs/spec.md)**. It defines:

- The thesis the project must prove
- System architecture (Go game server, Pi/TS LLM agents, Go scripted agents)
- The complete wire protocol between server and agents
- Game model (heads-up NLHE with cash-game auto-rebuy)
- Strategy lineup for v0
- SDK helper surface (the small one)
- The AKG-aware Pi compaction extension
- Session output format
- Build phasing (§17) — **start here for sequencing**

The spec is a contract. If you find ambiguity or omissions, surface them and propose a spec amendment in the same change — do not silently make a judgment call that quietly drifts from the spec.

### Spec vs. domain docs

Use this split consistently:

- **`docs/spec.md`**: project-specific technical decisions — architecture, wire protocol, session outputs, build sequencing, scope boundaries, and any implementation policy unique to this repo.
- **`docs/domain/`**: Texas Hold'em game rules and terminology — flop/turn/river, blinds, hole cards, betting order, hand rankings, showdown concepts, and similar poker-domain truth that agents should not reinvent.

Rule of thumb:

- If it is true because of this repository's design, it belongs in `docs/spec.md`.
- If it is true because it is part of Texas Hold'em itself, it belongs in `docs/domain/`.

When the code needs actual Hold'em rules, agents should consult `docs/domain/` rather than improvising from memory. If a project-specific behavior intentionally constrains or overrides the generic domain rules, `docs/spec.md` wins for that case.

## Adjacent code you'll need to know about

- **`~/source/akg`** — the AKG format spec, Go reference implementation, and conformance corpus. This project consumes the Go AKG SDK (lives in or near that repo) and, later, a TypeScript AKG SDK. When in doubt about how AKG behaves, read the spec at `~/source/akg/docs/spec/` and the reference implementation at `~/source/akg/internal/`.
- **`~/source/doug`** — a Go-based CLI orchestrator built on the Pi harness. Not a dependency of this project, but conceptually adjacent: doug also drives Pi child sessions and will eventually share the TS AKG SDK and Pi compaction extension we build here.
- **[pi.dev/docs/latest](https://pi.dev/docs/latest)** — Pi harness docs. Particularly relevant: RPC mode (stdin/stdout JSONL) and the `session_before_compact` extension hook.

## Working conventions

### Scope discipline

The single most important rule. The spec lists what's in v0 and what's explicitly out of scope (§15). Do not:

- Build features not in the spec because they "seem obvious."
- Add abstractions for hypothetical future requirements.
- Pre-build for 6-max, multiplayer, leaderboards, or any other v1+ thing. The spec calls out the small list of extension points the v0 design should preserve; preserving those is the entire forward-compatibility budget.

If something feels missing from the spec, that's a signal to surface a question or propose an amendment — not to invent.

### Go style

- Standard `gofmt`. No bikeshedding.
- Prefer the standard library. Add a dependency only when it materially saves work and the dependency is well-maintained.
- Table-driven tests for anything with non-trivial branching (rules engine especially).
- Keep packages small and single-purpose. Internal packages (`internal/...`) are not part of any public API.
- Error wrapping with `fmt.Errorf("...: %w", err)`. No `panic` outside of `main` / truly unrecoverable startup conditions.
- No comments explaining what code does — only why, and only when the why is non-obvious. Identifiers should carry the load.

### Determinism

The game server is deterministic given a seed. The rules engine, dealer, and match orchestrator must be pure functions of state + seed wherever feasible. LLM agents are not deterministic (temperature, model nondeterminism); that's fine and is called out in the spec.

### Tests

- Rules engine: extensive table-driven tests for hand evaluation, betting legality, pot accounting, blind rotation, showdown.
- Wire protocol: round-trip JSON tests for every message type, plus negative tests for malformed input.
- Match orchestrator: integration tests using `random` and `heuristic` agents (no LLMs) to verify a full session produces correct `hands.jsonl` and `manifest.json`.

LLM-agent behavior is not unit-tested (nondeterministic); validate it by running matches and inspecting the artifacts.

### Commits

- Conventional commit-ish prefixes (`feat:`, `fix:`, `chore:`, `docs:`, `test:`, `refactor:`) — match the AKG repo's style.
- One logical change per commit. A "wire up the rules engine" commit is fine; "wire up rules engine + rename a thing in match + fix a typo" is three commits.
- Never `git commit` without an explicit ask from the user.

## How to start

Follow the build phasing in [`docs/spec.md`](docs/spec.md) §17. Specifically:

1. **Rules engine** (pure Go, heavily unit-tested)
2. **Wire protocol** (Go types + JSON schemas + a short docs page)
3. **Game server + match orchestrator**
4. **`random` and `heuristic` agents**

Steps 1–4 are end-to-end demoable with no LLMs and no AKG — at the end you should be able to run `poker-server` and watch two trivial Go agents play a 200-hand match against each other, producing a valid `sessions/<id>/` output bundle. Stop and check in with the user before proceeding to steps 5+ (Go AKG SDK, TS AKG SDK, Pi agents).

## When you're unsure

- About the format / AKG behavior → read `~/source/akg/docs/spec/`.
- About Texas Hold'em rules or terminology → read `docs/domain/` first.
- About the wire protocol → it's spec §7. If it doesn't answer your question, propose an amendment.
- About scope or project behavior → assume it's out of scope unless the spec explicitly includes it. Ask the user.
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
