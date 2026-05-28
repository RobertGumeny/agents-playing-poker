# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- feat: add poker-eval init and ls experiment ergonomics
- feat: add poker-eval compare for experiment treatment/control reporting
- feat: add poker-eval collect and deterministic eval.json generation
- feat: add shared offline eval artifact loaders
- feat: add experiment status and coverage readouts to poker-eval
- feat: finish poker-eval run wrapper with deterministic planning, dry-run config output, and poker-run delegation
- feat: add poker-eval run planning and deterministic session derivation
- feat: add non-fatal session teardown memory-export artifacts
- docs: define JSON experiment-definition contract and add validation coverage
- test: complete benchmark reporting fixture and golden coverage
- feat: add poker-report CLI for explicit session directories
- feat: add benchmark review Markdown renderer
- feat: add benchmark aggregation metrics and mirror validation
- feat: add reporting session loader and normalized model
- test: cover llm-akg-recent poker-run alias resolution
- feat: rename llm-akg Pi agent to llm-akg-recent
- feat: add llm-fullhistory and split the shared Pi runtime into memory-policy and decision-engine seams
- feat: add llm-fullhistory as a buildable Pi agent with compact prompt-history memory and fresh Pi session reset per hand
- feat: split the shared Pi runtime seam into strategy-owned memory policy and strategy-owned decision engine responsibilities
- feat: add final `hand_end.action_history` plus `hand_end.showdown_reached` so memory strategies can build deterministic prior-hand summaries
- test: verify llm-fullhistory formatter stability, per-hand Pi session reset behavior, subprocess wiring, and seam compatibility with llm-stateless
- test: verify llm-stateless external-process wiring and canonical pi-session artifact compatibility
- test: add stateless fake-client protocol integration coverage
- feat: make llm-stateless a buildable external agent command and keep session_init metadata in the shared TS runtime
- feat: replace the llm-stateless Pi decision placeholder with a real SDK-backed per-decision client
- Hardened shared Pi-agent prompt construction, strict action parsing, legal-action validation, and safe fallback coverage.
- feat: implement shared Pi agent state transitions and resilient JSONL runner loop
- feat: complete TypeScript wire protocol payload types and validated JSONL helpers
- feat: add a narrow npm workspace for pi-agents shared runtime and tests
- test: add focused poker-demo orchestration coverage and promote the wrapper as the primary step-4 run path
- feat: make poker-demo session guidance clearer and fix short-blind all-in demo instability
- feat: add `poker-demo` as the supported one-command scripted demo wrapper around `poker-server`, with docs and integration coverage for the default random-vs-heuristic flow
- Added CLI-level step-4 demo coverage for poker-server, including random-vs-heuristic session bundle verification and timeout auto-fold behavior, and documented how to run and inspect the non-LLM demo.
- feat: add scripted heuristic Go agent baseline and support short blind all-in match completion
- Implemented the protocol-compliant Go random agent baseline and verified it completes full server-driven matches using only server-advertised legal actions.
- test: add integration coverage for deterministic session artifacts and incomplete matches
- Verified EPIC-3 server orchestration is integrated with the rules engine and persists the required session bundle artifacts.
- feat: add poker-server process lifecycle, stdio orchestration, and session artifact writing
- feat: add `poker-server` match orchestration, child-process lifecycle management, and session artifact writing
- feat: add session log helpers for `manifest.json`, `hands.jsonl`, and per-agent stdio logs
- test: add orchestration coverage for happy-path sessions, timeout auto-folds, and deterministic `hands.jsonl` replay
- test: add wire protocol payload-malformation coverage across all message types
- feat: add Go wire protocol envelope and payload types
- test: add table-driven rules-engine coverage for blinds, legal actions, pot accounting, hand evaluation, and showdown
- feat: add core NLHE domain model and deterministic Hold'em dealer primitives
- feat: add heads-up betting-round progression, legal-action generation, blind rotation, and pot accounting

### Changed
- docs: define stable session-artifact schemas for memory-export.json and eval.json
- docs: clarify llm-akg-recent operator references
- docs: update the spec, wire protocol, KB, and operator docs for the implemented llm-fullhistory baseline and shared Pi runtime seam
- docs: document persisted timeout `auto_fold` entries in session artifacts
- docs: add concise implementer contract for the stdio JSONL wire protocol

### Fixed

### Removed
