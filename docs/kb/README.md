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
