# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Added the core heads-up no-limit Hold'em engine, including deterministic dealing, betting-round progression, legal-action generation, blind rotation, pot accounting, hand evaluation, and showdown resolution.
- Added a Go wire-protocol implementation with typed envelopes, validated JSONL helpers, and broad malformed-payload coverage.
- Added `poker-server` match orchestration, including child-process lifecycle management, stdio coordination, deterministic session artifacts, and per-agent logs such as `manifest.json`, `hands.jsonl`, and stdio captures.
- Added baseline non-LLM agents: a protocol-compliant random agent and a scripted heuristic agent, along with support for short blind all-in edge cases so full matches complete cleanly.
- Added `poker-demo` as the main one-command local demo flow, with stronger orchestration coverage and clearer guidance for running and inspecting the default random-vs-heuristic session.
- Added a focused TypeScript workspace for Pi agents, covering shared runtime code, state transitions, prompt building, strict action parsing, legal-action validation, fallback handling, and resilient JSONL runner behavior.
- Added a real SDK-backed `llm-stateless` agent as a buildable external command, plus protocol and artifact compatibility coverage.
- Added `llm-fullhistory`, which resets its Pi session each hand and injects compact prior-hand summaries through a shared runtime split between memory policy and decision engine responsibilities.
- Added deterministic `hand_end.action_history` and `hand_end.showdown_reached` fields so memory strategies can build stable prior-hand summaries.
- Renamed the original AKG-backed Pi agent to `llm-akg-recent`, and added coverage for its `poker-run` alias behavior.
- Added benchmark and reporting tooling, including normalized session loading, mirror validation, aggregate metrics, Markdown benchmark reviews, and a `poker-report` CLI that can target explicit session directories.
- Added the first `poker-eval` workflow pieces: experiment initialization and listing, deterministic run planning and session derivation, dry-run config output, run delegation to `poker-run`, result collection, treatment-vs-control comparison reporting, status and coverage readouts, shared offline eval loaders, and deterministic `eval.json` generation.
- Added non-fatal teardown exports for agent memory, plus the experiment-definition JSON contract and validation coverage.
- Added broad test coverage across the rules engine, wire protocol, orchestration layer, demo flow, benchmark reporting, and LLM agent seams.

### Changed
- Reworked the documentation to better explain the end-to-end experiment workflow, command-line surface area, and the implemented reporting/evaluation flow.
- Clarified the stable session-artifact contracts for `memory-export.json`, `eval.json`, and persisted timeout `auto_fold` entries.
- Updated the specs, wire-protocol docs, knowledge base, and operator-facing docs to reflect `llm-fullhistory`, the shared Pi runtime seam, and the `llm-akg-recent` naming cleanup.
- Tightened the implementer documentation for the stdio JSONL wire protocol.
