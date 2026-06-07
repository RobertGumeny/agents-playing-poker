# Experiment Definition Contract

This document defines the checked-in **JSON** contract for planned experiment session sets.
It is the normative source for experiment-definition parsing and validation in this repo.

## Purpose

An experiment definition is a repo-owned plan for a set of sessions comparing two strategies on the fidelity-vs-cost frontier. It records:

- the experiment id and optional hypothesis text
- how each group's session ids are derived or enumerated
- the model used for Pi-backed agents unless overridden at runtime
- the strategy assigned to each seat in each group
- the per-session hand count (driven by the opponent strategy's expected context ceiling)

The format is JSON rather than YAML so parsing stays stdlib-only.

## Top-level schema

```json
{
  "id": "string",
  "hypothesis": "string, optional",
  "model": "anthropic:claude-sonnet-4-6",
  "hands_per_session": 60,
  "decision_deadline_seconds": 180,
  "groups": [ { "...group...": true } ]
}
```

### Required fields

- **`id`** — stable experiment identifier
- **`model`** — model identifier used for Pi-backed LLM agents unless overridden by the runner
- **`hands_per_session`** — expected hand count for every session in this experiment; must be `> 0`. Set this to the opponent strategy's expected context ceiling — the hand count where that strategy is likely to hit its breaking point (cost runaway, fidelity collapse, or context limit). AKG can run further; the ceiling is determined by the weaker strategy.
- **`groups`** — one or more session groups; minimum 1 element

### Optional fields

- **`hypothesis`** — free-form operator note about what the experiment is testing
- **`decision_deadline_seconds`** — per-decision wall-clock deadline for the agents; must be `>= 0`. Omitted or `0` falls back to the runner's `30s` default. The `poker experiment run`/`go` `-decision-deadline` flag (a Go duration, e.g. `180s`) overrides this per run.

Unknown JSON fields are invalid.

## Group schema

Each group must use **exactly one** session selection mode:

1. **session-base mode** — derive ids from a base name and count
2. **explicit-session mode** — list concrete session ids directly

Shared group fields:

```json
{
  "seat0": "string",
  "seat1": "string, optional",
  "seeds": [1, 2, 3]
}
```

- **`seat0`** — required strategy identifier placed in Seat 0 for this group
- **`seat1`** — optional strategy identifier placed in Seat 1. Required for `poker experiment run`/`go` to launch a missing session; may be omitted for offline tooling that derives opponents from existing session artifacts.
- **`seeds`** — optional planned seeds in session order

### Session-base mode

```json
{
  "session_base": "akg-vs-mdsingle",
  "sessions_count": 2,
  "seat0": "llm-akg-durable",
  "seat1": "llm-md-single",
  "seeds": [1, 2]
}
```

Rules:

- **`session_base`** is required
- **`sessions_count`** is required and must be `> 0`
- **`sessions`** must be omitted
- if **`seeds`** is present, its length must equal `sessions_count`

#### Deterministic session id derivation

Derived ids are:

- `<session_base>-1`
- `<session_base>-2`
- ...
- `<session_base>-<sessions_count>`

Example for `session_base = "akg-vs-mdsingle"` and `sessions_count = 2`:

```json
["akg-vs-mdsingle-1", "akg-vs-mdsingle-2"]
```

#### Deterministic seed derivation

- if **`seeds`** is provided, use it as-is in order
- if **`seeds`** is omitted or empty, default seeds are `1..sessions_count`

### Explicit-session mode

```json
{
  "sessions": [
    "akg-vs-mdsingle-seed1",
    "akg-vs-mdsingle-seed2"
  ],
  "seat0": "llm-akg-durable",
  "seat1": "llm-md-single",
  "seeds": [1, 2]
}
```

Rules:

- **`sessions`** is required and must be non-empty
- every session id in **`sessions`** must be non-empty
- duplicate session ids are invalid
- **`session_base`** and **`sessions_count`** must be omitted
- if **`seeds`** is present, its length must equal `len(sessions)`

## Group labels

Groups are referenced internally by positional label: `group-0`, `group-1`, etc., derived from the array index. There is no separate per-group label field in the JSON.

## Seat mirroring

To cancel positional and blind-rotation effects, add a second group with seats swapped. The two groups then contribute mirrored observations for each strategy across both seat positions:

```json
{
  "groups": [
    {
      "session_base": "akg-vs-mdsingle",
      "sessions_count": 2,
      "seat0": "llm-akg-durable",
      "seat1": "llm-md-single",
      "seeds": [1, 2]
    },
    {
      "session_base": "mdsingle-vs-akg",
      "sessions_count": 2,
      "seat0": "llm-md-single",
      "seat1": "llm-akg-durable",
      "seeds": [1, 2]
    }
  ]
}
```

## Example: session-base experiment

```json
{
  "id": "phase2-mdsingle-vs-akg",
  "hypothesis": "AKG vs md-single on the fidelity-vs-cost frontier.",
  "model": "anthropic:claude-sonnet-4-6",
  "hands_per_session": 60,
  "decision_deadline_seconds": 180,
  "groups": [
    {
      "session_base": "akg-vs-mdsingle",
      "sessions_count": 2,
      "seat0": "llm-akg-durable",
      "seat1": "llm-md-single",
      "seeds": [1, 2]
    },
    {
      "session_base": "mdsingle-vs-akg",
      "sessions_count": 2,
      "seat0": "llm-md-single",
      "seat1": "llm-akg-durable",
      "seeds": [1, 2]
    }
  ]
}
```

This expands to sessions `akg-vs-mdsingle-{1,2}` and `mdsingle-vs-akg-{1,2}` with seeds `[1, 2]` each.

## Example: explicit-session experiment

```json
{
  "id": "historical-benchmark",
  "model": "anthropic:claude-sonnet-4-6",
  "hands_per_session": 60,
  "groups": [
    {
      "sessions": ["akg-vs-mdsingle-seed1", "akg-vs-mdsingle-seed2"],
      "seat0": "llm-akg-durable",
      "seat1": "llm-md-single",
      "seeds": [1, 2]
    }
  ]
}
```

## Validation summary

A definition is invalid when any of the following are true:

- required top-level fields are missing
- `model` is empty
- `hands_per_session <= 0`
- `decision_deadline_seconds < 0`
- `groups` is empty
- a group omits `seat0`
- a group mixes session-base and explicit-session fields
- a group provides neither session mode
- `sessions_count <= 0` in session-base mode
- `seeds` length does not match planned session count
- an explicit session id is empty or duplicated
- unknown JSON fields are present
