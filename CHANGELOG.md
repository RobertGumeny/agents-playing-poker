# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
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
