# One-Command Scripted Demo Flow

EPIC-5 improved the operator UX for build-order step 4 by adding a supported wrapper around `poker-server` for the default scripted demo.

## Epic delivery summary

The archived EPIC-5 task log shows the work landed in three slices:
- `EPIC-5-001`: added `cmd/poker-demo` as the supported top-level entrypoint for the default `random` versus `heuristic` match
- `EPIC-5-002`: made successful runs print the canonical session bundle paths to inspect next
- `EPIC-5-003`: added focused wrapper-level coverage and promoted the wrapper as the primary step-4 operator path

## Scope delivered

The current one-command demo surface lives primarily in:
- `cmd/poker-demo`
- `cmd/poker-demo/main_test.go`
- `README.md`
- [`scripted-baseline-agents.md`](scripted-baseline-agents.md)

It now provides:
- a single supported command for the default non-LLM demo: `go run ./cmd/poker-demo`
- a small explicit override surface for session metadata and match knobs
- success output that prints the absolute session directory plus direct paths to `manifest.json`, `hands.jsonl`, and `agents/`
- preservation of `poker-server` as the underlying primitive and debugging escape hatch

## Current CLI behavior

`cmd/poker-demo` resolves the repo root, creates a temporary bin directory, builds these shipped binaries into it, then launches the server with explicit seat wiring:
- `./cmd/poker-server`
- `./cmd/random-agent`
- `./cmd/heuristic-agent`

The wrapper currently forwards these match settings into `poker-server`:
- `session-id`
- `sessions-dir`
- `match-id`
- `seed`
- `hand-count`
- `starting-stack`
- `small-blind`
- `big-blind`
- `decision-deadline`
- `go-bin`

The seat mapping is intentionally fixed in the wrapper:
- seat 0: `random`
- seat 1: `heuristic`

That fixed wiring is part of the v0 UX goal: remove setup friction for the baseline checkpoint without expanding `poker-demo` into a generic tournament launcher.

## Relationship to the lower-level server

EPIC-5 did not change the low-level server contract.

Important layering to preserve:
- `poker-demo` is only a convenience wrapper
- `poker-server` remains the canonical runtime primitive
- session artifacts still live under `sessions/<id>/`
- timeout handling, hand progression, and manifest writing still come from the existing server and match packages

If future work needs custom seat commands, non-default agents, or lower-level debugging, it should drop to `poker-server` rather than stretching `poker-demo` beyond its narrow purpose.

## Durable operational constraints

The archived session logs surfaced a few constraints worth keeping in docs instead of only in code:
- the wrapper requires a working Go toolchain at runtime because it shells out to `go build`
- the current implementation builds fresh temporary binaries before each run instead of assuming prebuilt executables
- printing absolute artifact paths is intentional so operators can immediately inspect or copy/paste them
- wrapper-level coverage should stay focused on orchestration helpers and UX output, while full match confidence continues to come from `poker-server`, `internal/match`, and the scripted agent tests

One failed EPIC-5 attempt also exposed a real gameplay edge case: a short small-blind all-in hand could leave the demo unstable until the rules engine advanced directly to showdown when no further action was possible. That fix belongs to the rules and scripted-baseline layers, but it is part of why the wrapper can now be treated as the reliable default operator path.

## Normative sources

This wrapper layer should stay aligned with:
- [`../spec.md`](../spec.md) for build sequencing and v0 scope boundaries
- [`../README.md`](../README.md) for the current operator-facing run path
- [`server-orchestration.md`](server-orchestration.md) for the underlying server lifecycle and artifact contract
- [`scripted-baseline-agents.md`](scripted-baseline-agents.md) for the baseline agent behavior and step-4 demo context

## Current boundaries

Still out of scope for `poker-demo`:
- arbitrary agent selection
- multiplayer or tournament orchestration
- LLM or Pi-specific flows
- alternative artifact formats
- long-lived installable packaging beyond the current repo-local Go run path

## Why this matters for later work

Later agent and evaluation work can assume the repository now has two distinct operator surfaces:
- a low-level server CLI for explicit wiring and debugging
- a high-confidence one-command baseline demo for the default scripted checkpoint

That split keeps the step-4 demo easy to run without weakening the underlying server contract that later LLM-based work must continue to use.
