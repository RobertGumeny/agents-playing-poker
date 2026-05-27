# Experiment Definition Contract

This document defines the checked-in **JSON** contract for planned experiment session sets.
It is the normative source for experiment-definition parsing and validation in this repo.

## Purpose

An experiment definition is a repo-owned plan for a two-group comparison:

- **`control`** — the baseline group
- **`treatment`** — the variant being tested

The file records:

- the experiment id and optional hypothesis text
- how each group's session ids are derived or enumerated
- the intended agent and optional opponent metadata for each group
- the per-session hand count expectation
- optional expected metric directions for later compare/report tooling

The format is JSON rather than YAML so parsing stays stdlib-only.

## Top-level schema

```json
{
  "id": "string",
  "hypothesis": "string, optional",
  "hands_per_session": 25,
  "control": { "...group...": true },
  "treatment": { "...group...": true },
  "expected_direction": {
    "chips_per_hand": "increase",
    "session_duration_s": "decrease"
  }
}
```

### Required fields

- **`id`** — stable experiment identifier
- **`hands_per_session`** — expected hand count for each planned session; must be `> 0`
- **`control`** — baseline group definition
- **`treatment`** — comparison group definition

### Optional fields

- **`hypothesis`** — free-form operator note about what the experiment is testing
- **`expected_direction`** — metric-to-direction map used by future compare tooling

Unknown JSON fields are invalid.

## Group schema

Each group must use **exactly one** session selection mode:

1. **session-base mode** — derive ids from a base name and count
2. **explicit-session mode** — list concrete session ids directly

Shared group fields:

```json
{
  "agent": "string",
  "opponent": "string, optional",
  "seeds": [1, 2, 3]
}
```

- **`agent`** — required intended agent identifier for the group
- **`opponent`** — optional intended opposing agent identifier
- **`seeds`** — optional planned seeds in session order

`opponent` remains optional at the file-format level because offline tooling can derive opponents from session artifacts later. However, `poker-eval run` can only launch a missing planned session when that session's group includes `opponent` metadata.

### Session-base mode

```json
{
  "session_base": "akg-durable-throttle-test",
  "sessions_count": 5,
  "agent": "llm-akg-durable@exp-0.1.3-throttle",
  "opponent": "llm-stateless",
  "seeds": [1, 2, 3, 4, 5]
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

Example for `session_base = "akg-durable-throttle-test"` and `sessions_count = 3`:

```json
[
  "akg-durable-throttle-test-1",
  "akg-durable-throttle-test-2",
  "akg-durable-throttle-test-3"
]
```

#### Deterministic seed derivation

Seed derivation is positional and deterministic:

- if **`seeds`** is provided, use it as-is in order
- if **`seeds`** is omitted or empty, default seeds are `1..sessions_count`

So a 3-session group with no explicit `seeds` plans seeds `[1, 2, 3]`.

### Explicit-session mode

```json
{
  "sessions": [
    "fullhistory-vs-stateless-a",
    "fullhistory-vs-stateless-b"
  ],
  "agent": "llm-fullhistory",
  "seeds": [1, 1]
}
```

Rules:

- **`sessions`** is required and must be non-empty
- every session id in **`sessions`** must be non-empty
- duplicate session ids are invalid
- **`session_base`** and **`sessions_count`** must be omitted
- if **`seeds`** is present, its length must equal `len(sessions)`

#### Deterministic session order and seed derivation

In explicit-session mode, the listed array order is authoritative.

- planned session ids are exactly the entries in **`sessions`**
- if **`seeds`** is provided, use it positionally
- if **`seeds`** is omitted or empty, default seeds are `1..len(sessions)`

This default is only a planning fallback. For already-run historical sessions, later tooling may verify actual seeds from `manifest.json`.

## Group labeling semantics

The stable group labels are the top-level property names:

- **`control`**
- **`treatment`**

These labels are part of the contract and are used when expanding planned sessions, validating experiment coverage, and evaluating `expected_direction` outcomes.

The JSON file does not carry a separate per-group `label` field.

## Expected-direction semantics

`expected_direction` is an optional map from metric name to one of:

- **`"increase"`** — treatment is expected to be higher than control
- **`"decrease"`** — treatment is expected to be lower than control

Semantics are always **treatment relative to control**.

Examples:

- `"chips_per_hand": "increase"` means the treatment should outperform the control on chips per hand
- `"session_duration_s": "decrease"` means the treatment should finish faster than the control

If a metric is absent from `expected_direction`, later reporting tooling should treat it as informational only and skip direction pass/fail marking.

## Example: session-base experiment

```json
{
  "id": "test-2b-retrieval-throttle",
  "hypothesis": "Throttling memory retrieval to once per hand should cut tool use and session time.",
  "hands_per_session": 25,
  "control": {
    "session_base": "akg-durable-vs-stateless-test",
    "sessions_count": 5,
    "agent": "llm-akg-durable/0.1.0",
    "opponent": "llm-stateless"
  },
  "treatment": {
    "session_base": "akg-durable-throttle-test",
    "sessions_count": 5,
    "agent": "llm-akg-durable@exp-0.1.3-throttle",
    "opponent": "llm-stateless"
  },
  "expected_direction": {
    "chips_per_hand": "increase",
    "akg_get_opponent_per_session": "decrease",
    "session_duration_s": "decrease",
    "preflop_only_rate": "decrease"
  }
}
```

This expands deterministically to:

- control sessions `akg-durable-vs-stateless-test-{1..5}` with default seeds `1..5`
- treatment sessions `akg-durable-throttle-test-{1..5}` with default seeds `1..5`

## Example: explicit-session experiment

```json
{
  "id": "historical-fullhistory-vs-durable",
  "hands_per_session": 200,
  "control": {
    "sessions": [
      "fullhistory-vs-stateless-a",
      "fullhistory-vs-stateless-b"
    ],
    "agent": "llm-fullhistory",
    "seeds": [1, 1]
  },
  "treatment": {
    "sessions": [
      "akg-durable-vs-fullhistory-a",
      "akg-durable-vs-fullhistory-b"
    ],
    "agent": "llm-akg-durable"
  },
  "expected_direction": {
    "chips_per_hand": "increase"
  }
}
```

This preserves the explicit session ordering exactly as written.

## Validation summary

A definition is invalid when any of the following are true:

- required top-level fields are missing
- `hands_per_session <= 0`
- a group omits `agent`
- a group mixes session-base and explicit-session fields
- a group provides neither session mode
- `sessions_count <= 0` in session-base mode
- `seeds` length does not match planned session count
- an explicit session id is empty or duplicated
- an `expected_direction` value is not `increase` or `decrease`
- unknown JSON fields are present
