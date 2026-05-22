# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
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
- docs: document persisted timeout `auto_fold` entries in session artifacts
- docs: add concise implementer contract for the stdio JSONL wire protocol

### Fixed

### Removed
