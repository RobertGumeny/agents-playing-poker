# Knowledge Base

This directory captures durable implementation knowledge that is more operational than the top-level project docs.

Use it for:

- module-level architecture notes
- implementation constraints already reflected in code
- testing and verification guidance
- subsystem design rationale

Do not duplicate the normative sources:

- [`../vision.md`](../vision.md) and [`../research.md`](../research.md) for repository-level thesis and research framing
- [`../eval-system.md`](../eval-system.md) for the experiment-first operator workflow
- [`../experiment-definition.md`](../experiment-definition.md) for experiment JSON schema
- [`../session-artifacts.md`](../session-artifacts.md) for additive artifact schemas
- [`../wire-protocol.md`](../wire-protocol.md) for server/agent JSONL protocol
- [`../llm-akg-durable-spec.md`](../llm-akg-durable-spec.md) for the durable AKG agent contract
- [`../domain/README.md`](../domain/README.md) for Texas Hold'em rules and terminology

## Index

### Core engine and protocol

- [`rules-engine-foundation.md`](rules-engine-foundation.md)
  - Go rules engine determinism, hand progression, showdown resolution, and test coverage.
- [`wire-protocol-contract.md`](wire-protocol-contract.md)
  - Typed wire protocol models, envelope validation, reply correlation, and protocol tests.
- [`server-orchestration.md`](server-orchestration.md)
  - Server/match orchestration, child-process lifecycle, timeout handling, and session artifacts.

### Agents

- [`scripted-baseline-agents.md`](scripted-baseline-agents.md)
  - `random` and `heuristic` scripted sanity-check baselines.
- [`llm-stateless-pi-baseline.md`](llm-stateless-pi-baseline.md)
  - Stateless Pi-backed LLM baseline and shared TypeScript runtime boundary.
- [`llm-fullhistory-baseline.md`](llm-fullhistory-baseline.md)
  - Naive full-history prompt baseline and shared memory-policy seam.
- [`llm-akg-durable-active-retrieval.md`](llm-akg-durable-active-retrieval.md)
  - Durable AKG active-retrieval implementation details and verification surface.

### Experiments and evaluation

- [`experiment-planning-and-session-artifacts.md`](experiment-planning-and-session-artifacts.md)
  - Experiment-definition planning, memory export behavior, and artifact authority split.
- [`experiment-execution-and-coverage.md`](experiment-execution-and-coverage.md)
  - Root `poker experiment` coverage and execution semantics.
- [`offline-eval-collection.md`](offline-eval-collection.md)
  - Offline `eval.json` collection from session artifacts.
- [`experiment-comparison-and-operator-workflow.md`](experiment-comparison-and-operator-workflow.md)
  - Experiment report generation, metadata validation, and operator workflow.
- [`repeatable-benchmark-reporting.md`](repeatable-benchmark-reporting.md)
  - Benchmark report metrics, seat-bias checks, and interpretation guidance.
