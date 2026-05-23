# Knowledge Base

This directory captures durable implementation knowledge that is more operational than the top-level spec.

Use it for:
- module-level architecture notes
- implementation constraints that are already reflected in code
- testing and verification guidance
- epic summaries that future tasks should understand before extending a subsystem

Do not duplicate the normative sources:
- Use [`../spec.md`](../spec.md) for repository-level product and architecture decisions.
- Use [`../domain/README.md`](../domain/README.md) for Texas Hold'em rules and terminology.

## Index

- [`rules-engine-foundation.md`](rules-engine-foundation.md)
  - What EPIC-1 delivered in the Go rules engine.
  - Covers deck/dealer determinism, hand-state progression, showdown resolution, and current test coverage.
- [`wire-protocol-contract.md`](wire-protocol-contract.md)
  - What EPIC-2 delivered in the Go wire protocol contract.
  - Covers typed message models, envelope validation, reply correlation, and protocol test coverage.
- [`server-orchestration.md`](server-orchestration.md)
  - What EPIC-3 delivered in the Go server and match orchestrator.
  - Covers the epic delivery slices, process lifecycle, timeout/incomplete-match handling, session artifacts, and deterministic replay coverage.
- [`scripted-baseline-agents.md`](scripted-baseline-agents.md)
  - What EPIC-4 delivered in the scripted non-LLM agent layer and step-4 demo path.
  - Covers `random` and `heuristic` agent behavior, baseline constraints, demo verification, and the timeout-tested local run surface.
- [`one-command-scripted-demo-flow.md`](one-command-scripted-demo-flow.md)
  - What EPIC-5 delivered in the operator-facing wrapper around the scripted step-4 demo.
  - Covers `cmd/poker-demo`, its narrow override surface, artifact inspection output, and the layering boundary with `poker-server`.
- [`llm-stateless-pi-baseline.md`](llm-stateless-pi-baseline.md)
  - What EPIC-7 delivered in the first runnable Pi-backed LLM baseline.
  - Covers the shared TypeScript runtime, per-decision Pi session isolation, external-process command shape, canonical `pi-session.jsonl` artifacts, and test seams that avoid live-model requirements.
- [`llm-fullhistory-baseline.md`](llm-fullhistory-baseline.md)
  - What EPIC-8 delivered in the prompt-history Pi baseline.
  - Covers the fresh-session-per-hand boundary, compact prior-hand summary format, `hand_end` protocol support, and the shared runtime seam between memory policy and decision engine.
