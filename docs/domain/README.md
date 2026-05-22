# Domain Docs

This directory is the canonical source for **Texas Hold'em domain semantics** used by this repository.

Use it for rules and terminology that agents should treat as ground truth rather than re-deriving from memory.

## Split from `docs/spec.md`

- **`docs/spec.md`** is the project's technical contract.
  - Architecture
  - Wire protocol
  - Session outputs
  - Build order
  - Scope and non-goals
  - Project-specific policy decisions

- **`docs/domain/`** is the poker-domain reference.
  - Hold'em terminology
  - Streets and flow of a hand
  - Blind/button behavior
  - Betting order
  - Hand rankings
  - Showdown concepts
  - Other stable game-rule facts

Rule of thumb:

- If it is true because of **this repository**, put it in `docs/spec.md`.
- If it is true because of **Texas Hold'em**, put it in `docs/domain/`.

If a project-specific behavior intentionally constrains or overrides generic poker rules, `docs/spec.md` takes precedence for that case.

## How to use these docs when implementing

Treat these docs as **normative** for poker behavior in this repository.

- Read [`texas-holdem.md`](texas-holdem.md) for base Hold'em rules.
- Read [`heads-up-nlhe.md`](heads-up-nlhe.md) for the heads-up no-limit interpretation used by this project.
- Use [`glossary.md`](glossary.md) as a quick terminology reference.

Do **not** fill gaps by guessing from memory when implementing rules code.

If implementation requires a poker rule that is not clearly stated in these docs or intentionally overridden in [`../spec.md`](../spec.md), treat that as a documentation gap and amend the docs rather than silently inventing behavior.

## Documents in this directory

- [`docs/domain/README.md`](README.md)
  - Index for domain docs and the spec/domain split.
- [`docs/domain/texas-holdem.md`](texas-holdem.md)
  - Canonical rules reference for generic Texas Hold'em.
  - Covers cards, streets, betting-round flow, legal action vocabulary, hand rankings, and showdown.
- [`docs/domain/heads-up-nlhe.md`](heads-up-nlhe.md)
  - Applied interpretation for heads-up no-limit Texas Hold'em.
  - Clarifies heads-up blind/button behavior, exact action order, and no-limit raise rules that are easy to implement incorrectly.
- [`docs/domain/glossary.md`](glossary.md)
  - Short-form glossary of common Hold'em terms used throughout the repo.
