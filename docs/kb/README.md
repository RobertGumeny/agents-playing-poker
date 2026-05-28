# Knowledge Base

This directory captures durable implementation knowledge that is more operational than the top-level project docs.

Use it for:
- module-level architecture notes
- implementation constraints that are already reflected in code
- testing and verification guidance
- why a subsystem is designed the way it is

Do not duplicate the normative sources:
- Use [`../vision.md`](../vision.md) and [`../research.md`](../research.md) for repository-level product and research framing.
- Use focused subsystem docs such as [`../wire-protocol.md`](../wire-protocol.md) and [`../llm-akg-durable-spec.md`](../llm-akg-durable-spec.md) for implementation contracts.
- Use [`../domain/README.md`](../domain/README.md) for Texas Hold'em rules and terminology.

## Index

- [`rules-engine-foundation.md`](rules-engine-foundation.md)
  - The Go rules engine: deck/dealer determinism, hand-state progression, showdown resolution, and test coverage.
- [`wire-protocol-contract.md`](wire-protocol-contract.md)
  - The Go wire protocol: typed message models, envelope validation, reply correlation, and protocol test coverage.
- [`server-orchestration.md`](server-orchestration.md)
  - The Go server and match orchestrator: process lifecycle, timeout/incomplete-match handling, session artifacts, and deterministic replay coverage.
- [`scripted-baseline-agents.md`](scripted-baseline-agents.md)
  - The scripted non-LLM agent layer: `random` and `heuristic` agent behavior, baseline constraints, demo verification, and the timeout-tested local run surface.
- [`one-command-scripted-demo-flow.md`](one-command-scripted-demo-flow.md)
  - `cmd/poker-demo`: its narrow override surface, artifact inspection output, and the layering boundary with `poker-server`.
- [`llm-stateless-pi-baseline.md`](llm-stateless-pi-baseline.md)
  - The stateless LLM baseline: shared TypeScript runtime, per-decision Pi session isolation, external-process command shape, canonical `pi-session.jsonl` artifacts, and test seams that avoid live-model requirements.
- [`llm-fullhistory-baseline.md`](llm-fullhistory-baseline.md)
  - The full-history LLM baseline: fresh-session-per-hand boundary, compact prior-hand summary format, `hand_end` protocol support, and the shared runtime seam between memory policy and decision engine.
- [`repeatable-benchmark-reporting.md`](repeatable-benchmark-reporting.md)
  - `cmd/poker-report` and `internal/reporting`: required metrics, seat-bias checks, showdown/non-showdown splits, cost reporting gaps, and the comparison ladder from `llm-akg-recent` to durable AKG and `llm-fullhistory`.
- [`experiment-planning-and-session-artifacts.md`](experiment-planning-and-session-artifacts.md)
  - Experiment-definition parsing and additive analysis artifacts: strict JSON experiment contracts, deterministic planned-session expansion, non-fatal `memory-export.json` teardown behavior, and the authority split between primary and derived session files.
- [`experiment-execution-and-coverage.md`](experiment-execution-and-coverage.md)
  - `poker-eval run` and `poker-eval status`: delegated `poker-run` execution, present/missing/incomplete session checks, and rerun semantics for incomplete sessions.
- [`offline-eval-collection.md`](offline-eval-collection.md)
  - `poker-eval collect`: shared offline artifact loading, Pi tool-call parsing, retry metrics from `stderr.log`, generic memory-export summaries, and current metric derivation boundaries.
- [`experiment-comparison-and-operator-workflow.md`](experiment-comparison-and-operator-workflow.md)
  - `poker-eval init`, `poker-eval ls`, `poker-eval compare`: the `poker-eval`/`poker-run` responsibility split, report semantics, and comparison-time warnings and validation.
