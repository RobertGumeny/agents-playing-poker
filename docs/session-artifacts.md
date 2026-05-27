# Session Artifact Schemas

This document defines the stable JSON contracts for additive session artifacts used by offline tooling.

Normative scope in this file:
- `sessions/<id>/agents/<name>/memory-export.json`
- `sessions/<id>/eval.json`

For the primary live-match artifacts, see `manifest.json` and `hands.jsonl` in `internal/sessionlog` and the surrounding runtime docs.

## Authority and lifecycle

| Artifact | Writer | When it appears | Authority | Notes |
|---|---|---|---|---|
| `manifest.json` | server | during match teardown | primary session authority | match metadata, seats, totals, completion |
| `hands.jsonl` | server | streamed during play | primary hand-by-hand authority | one server-authoritative hand record per line |
| `agents/<name>/memory.akg` | memory-capable agent | during play | primary live memory store | durable agent-owned store |
| `agents/<name>/memory-export.json` | server | teardown, after agent shutdown | derived snapshot authority | JSON export of `memory.akg`; additive and non-fatal |
| `eval.json` | offline collector | post-session | derived summary authority | replaceable summary for downstream tooling; never overrides primary artifacts |

Two rules matter for downstream consumers:

1. `memory.akg` remains the authoritative durable memory store for replay or future re-export.
2. `eval.json` is a convenience summary. If it disagrees with `manifest.json`, `hands.jsonl`, `pi-session.jsonl`, or `memory-export.json`, the source artifacts win.

## `memory-export.json`

### Path

`sessions/<id>/agents/<name>/memory-export.json`

### Lifecycle

- Written by `internal/sessionlog.WriteMemoryExport` during session teardown.
- Generated only when `agents/<name>/memory.akg` exists and can be opened.
- Missing `memory.akg` is a no-op.
- Export failure is non-fatal: the session still completes and no partial `memory-export.json` should be trusted.
- The file is a read-only snapshot for analysis. It does not replace `memory.akg`.

### Top-level schema

```json
{
  "nodes": [],
  "edges": []
}
```

Both properties are always present.

- `nodes` — array of exported AKG nodes
- `edges` — array of exported outbound edges discovered while walking the graph

Consumers must not depend on array ordering.

### Node schema

Each entry in `nodes` has this shape:

```json
{
  "type": "string",
  "id": "string",
  "title": "string",
  "body": "string",
  "meta": {},
  "tags": []
}
```

Field semantics:

- `type` — AKG node type
- `id` — AKG node id
- `title` — AKG node title
- `body` — AKG node body; may be an empty string, but is emitted as a JSON string field
- `meta` — object copy of AKG node metadata; never `null`
- `tags` — array copy of AKG node tags; never `null`

### Edge schema

Each entry in `edges` has this shape:

```json
{
  "from": { "type": "string", "id": "string" },
  "relation": "string",
  "to": { "type": "string", "id": "string" },
  "meta": {}
}
```

Field semantics:

- `from` — source node reference
- `relation` — AKG edge relation
- `to` — destination node reference
- `meta` — object copy of AKG edge metadata; never `null`

### Stability rules

- The export mirrors AKG graph contents generically; it does not hard-code poker-specific node types.
- Downstream tools may inspect current `opponent`, `hand`, and `pattern` conventions, but the stable contract here is the generic graph shape above.
- Numeric metadata values round-trip through JSON numbers.
- Missing metadata and missing tags are normalized to `{}` and `[]` rather than `null`.

### Example

This example matches the checked-in export fixture shape covered by `internal/sessionlog/memory_export_test.go`:

```json
{
  "nodes": [
    {
      "type": "hand",
      "id": "hand-1",
      "title": "Hand 1",
      "body": "Hand summary.",
      "meta": {
        "hand_number": 1
      },
      "tags": ["hand", "showdown"]
    },
    {
      "type": "opponent",
      "id": "villain",
      "title": "villain",
      "body": "Villain profile.",
      "meta": {
        "hands_played": 3,
        "vpip": 2
      },
      "tags": ["opponent"]
    }
  ],
  "edges": [
    {
      "from": { "type": "opponent", "id": "villain" },
      "relation": "supported_by",
      "to": { "type": "hand", "id": "hand-1" },
      "meta": {
        "count": 1
      }
    }
  ]
}
```

## `eval.json`

### Path

`sessions/<id>/eval.json`

### Lifecycle

- Written only by offline collection tooling after a session completes.
- Safe to delete and regenerate from source artifacts.
- Must be derived from checked-in session artifacts, not from operator memory or ad hoc prompt analysis.
- Must stay additive: absence of `eval.json` never means the session failed.

### Scope

`eval.json` is a session-level normalized summary. It intentionally keeps both seats in one file so later compare/report tooling can reason about a whole heads-up run without reopening the primary artifacts for common metrics.

### Top-level schema

```json
{
  "schema_version": 1,
  "session_id": "string",
  "match_id": "string",
  "source_artifacts": {},
  "session": {},
  "metrics": {},
  "seats": []
}
```

### Top-level fields

- `schema_version` — required integer schema marker; current value is `1`
- `session_id` — required session id from `manifest.json`
- `match_id` — required current match id from the first and only v0 manifest match entry
- `source_artifacts` — required provenance block listing the files used to build the summary
- `session` — required session metadata block
- `metrics` — required session-level derived metrics block
- `seats` — required per-seat summaries in manifest seat order

### `source_artifacts`

```json
{
  "manifest": "manifest.json",
  "hands": "hands.jsonl",
  "agents": {
    "llm-akg-durable": {
      "pi_session": "agents/llm-akg-durable/pi-session.jsonl",
      "memory_export": "agents/llm-akg-durable/memory-export.json"
    },
    "llm-stateless": {
      "pi_session": "agents/llm-stateless/pi-session.jsonl",
      "memory_export": null
    }
  }
}
```

Rules:

- `manifest` and `hands` are required relative paths.
- `agents` is keyed by agent directory name.
- `pi_session` is the relative path to `pi-session.jsonl` when present, otherwise `null`.
- `memory_export` is the relative path to `memory-export.json` when present, otherwise `null`.

### `session`

```json
{
  "seed": 1,
  "duration_s": 701,
  "hand_count": 25,
  "variant": "heads-up-nlhe",
  "info_realism": "showdown-only",
  "starting_stack": 200,
  "blinds": { "sb": 1, "bb": 2 },
  "completed": true,
  "server_version": "dev",
  "akg_spec_version": "v1-draft-2"
}
```

Derivation rules:

- `seed`, `hand_count`, `variant`, `info_realism`, `starting_stack`, `blinds`, `server_version`, and `akg_spec_version` come from `manifest.json`.
- `completed` comes from the first and only v0 manifest match entry.
- `duration_s` is `ended_at - started_at` in whole seconds.

### `metrics`

```json
{
  "preflop_only_hands": 17,
  "preflop_only_rate": 0.68,
  "showdown_hands": 3,
  "showdown_rate": 0.12,
  "biggest_swing_hand": {
    "hand_number": 1,
    "chips": 2
  },
  "fallback_action_count": 3
}
```

Derivation rules:

- `preflop_only_hands` — count of hands whose recorded actions never leave `street == "preflop"`
- `preflop_only_rate` — `preflop_only_hands / hand_count`
- `showdown_hands` — count of hands with `showdown_reached == true`
- `showdown_rate` — `showdown_hands / hand_count`
- `biggest_swing_hand` — hand with the largest winner net chip delta in `hands.jsonl`; this repo currently reports swing, not reconstructed gross pot size
- `fallback_action_count` — total count of `auto_fold`, `auto_check`, or actions carrying `forced_reason`

### `seats[]`

Each seat entry has this shape:

```json
{
  "seat": 0,
  "name": "llm-akg-durable",
  "version": "llm-akg-durable@exp-0.1.2-prompt",
  "chips_delta": 0,
  "pi_session_present": true,
  "decision_prompt_count": 113,
  "tool_calls": {
    "akg_get_opponent": 76,
    "akg_list_patterns": 27,
    "akg_get_pattern": 4
  },
  "tool_calls_per_hand": {
    "akg_get_opponent": 3.04,
    "akg_list_patterns": 1.08,
    "akg_get_pattern": 0.16
  },
  "memory_export": null
}
```

Field semantics:

- `seat` — manifest seat number
- `name` — manifest seat strategy name
- `version` — manifest seat version string, or empty string when absent
- `chips_delta` — seat result from the manifest match result map
- `pi_session_present` — whether `agents/<name>/pi-session.jsonl` existed
- `decision_prompt_count` — count of decision prompts observed in that seat's `pi-session.jsonl`; `0` when no Pi session log exists
- `tool_calls` — map of tool name to count, derived from `pi-session.jsonl` assistant messages where `content[].type == "toolCall"`
- `tool_calls_per_hand` — `tool_calls[name] / hand_count`, rounded only by normal JSON number formatting
- `memory_export` — `null` when no export exists, otherwise a lightweight graph summary object

### `memory_export` summary object

When a seat has `memory-export.json`, `seats[].memory_export` has this shape:

```json
{
  "node_count": 2,
  "edge_count": 1,
  "nodes_by_type": {
    "hand": 1,
    "opponent": 1
  },
  "edges_by_relation": {
    "supported_by": 1
  }
}
```

This summary is intentionally generic. Downstream tools that need full graph details should reopen the raw `memory-export.json` path named in `source_artifacts`.

### Example

This example is derived from the checked-in session fixture `sessions/akg-durable-prompt-test-1/` and matches the current artifact shapes in the repo:

```json
{
  "schema_version": 1,
  "session_id": "akg-durable-prompt-test-1",
  "match_id": "mat_001",
  "source_artifacts": {
    "manifest": "manifest.json",
    "hands": "hands.jsonl",
    "agents": {
      "llm-akg-durable": {
        "pi_session": "agents/llm-akg-durable/pi-session.jsonl",
        "memory_export": null
      },
      "llm-stateless": {
        "pi_session": "agents/llm-stateless/pi-session.jsonl",
        "memory_export": null
      }
    }
  },
  "session": {
    "seed": 1,
    "duration_s": 701,
    "hand_count": 25,
    "variant": "heads-up-nlhe",
    "info_realism": "showdown-only",
    "starting_stack": 200,
    "blinds": { "sb": 1, "bb": 2 },
    "completed": true,
    "server_version": "dev",
    "akg_spec_version": "v1-draft-2"
  },
  "metrics": {
    "preflop_only_hands": 17,
    "preflop_only_rate": 0.68,
    "showdown_hands": 3,
    "showdown_rate": 0.12,
    "biggest_swing_hand": {
      "hand_number": 1,
      "chips": 2
    },
    "fallback_action_count": 3
  },
  "seats": [
    {
      "seat": 0,
      "name": "llm-akg-durable",
      "version": "llm-akg-durable@exp-0.1.2-prompt",
      "chips_delta": 0,
      "pi_session_present": true,
      "decision_prompt_count": 113,
      "tool_calls": {
        "akg_get_opponent": 76,
        "akg_list_patterns": 27,
        "akg_get_pattern": 4
      },
      "tool_calls_per_hand": {
        "akg_get_opponent": 3.04,
        "akg_list_patterns": 1.08,
        "akg_get_pattern": 0.16
      },
      "memory_export": null
    },
    {
      "seat": 1,
      "name": "llm-stateless",
      "version": "llm-stateless/0.1.0",
      "chips_delta": 0,
      "pi_session_present": true,
      "decision_prompt_count": 85,
      "tool_calls": {},
      "tool_calls_per_hand": {},
      "memory_export": null
    }
  ]
}
```
