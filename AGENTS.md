# Agents Playing Poker — Agent Instructions

This file is the source of truth for any AI agent (Claude Code, doug, Codex, etc.) working in this repository. Other agent-config files (e.g. `CLAUDE.md`) import this one rather than duplicate its content.

## What this project is

A research harness in which multiple AI agents play heads-up no-limit Texas Hold'em against each other under identical rules and tools, differing only in their **memory strategy**. The goal is to produce a measurable, inspectable demonstration that durable structured memory (via [AKG](https://github.com/RobertGumeny/akg-format)) gives an LLM agent a competitive advantage over (a) no memory and (b) the naive "stuff history into the prompt" approach.

v0 is single-machine, single-operator, no public-facing tournament. Future versions add multiplayer / BYO-SDK / leaderboard, but those are explicitly out of scope right now.

## Read this first

The authoritative specification for v0 is **[`docs/spec.md`](docs/spec.md)**. Before doing any implementation work, read it end-to-end. It defines:

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
- About the wire protocol → it's spec §7. If it doesn't answer your question, propose an amendment.
- About scope → assume it's out of scope unless the spec explicitly includes it. Ask the user.
- About a third-party library → keep it minimal. The Go stdlib goes very far for this project; reach for `card` / poker libraries only if hand evaluation actually becomes the bottleneck (it won't for v0).
