# Agent Poker

A research harness in which AI agents play no-limit Texas Hold'em against each other under identical rules and tools, differing only in their **memory strategy**.

The goal is to produce a measurable, inspectable demonstration that durable structured memory (via [AKG](https://github.com/RobertGumeny/akg-format)) gives an LLM agent a competitive advantage over no memory and over the naive "stuff history into the prompt" approach.

## Status

v0, pre-implementation. The specification is complete; the code is not yet written.

## Read

- **[`docs/spec.md`](docs/spec.md)** — the v0 specification (architecture, wire protocol, strategy lineup, output format, build phasing).
- **[`AGENTS.md`](AGENTS.md)** — instructions for AI agents working in this repo.
