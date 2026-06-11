# Session Artifact Schemas

This document is the complete map of files a session emits, plus the stable JSON contracts for the additive artifacts that offline tooling reads.

Normative schema scope in this file (the contracts downstream tooling depends on):
- `sessions/<id>/agents/<name>/session-decisions.jsonl`
- `sessions/<id>/agents/<name>/memory-export.json`
- `sessions/<id>/eval.json`

For the primary live-match artifacts, see `manifest.json` and `hands.jsonl` in `internal/sessionlog` and the surrounding runtime docs.

## Complete artifact map

Every artifact lives on three axes: **owner** (server / agent / derived), **scope** (per-hand / session / experiment), and, for agent traces, **phase** (decide / learn). The filename scheme encodes them.

| Artifact | Owner | Scope | Writer | Consumers |
|---|---|---|---|---|
| `manifest.json` | server | session | server (match teardown) | eval collector, every report |
| `hands.jsonl` | server | per-hand | server (streamed during play) | eval collector, fidelity ground truth, replay |
| `session-report.md` | derived | session | server (standalone runs only) | human; suppressed in experiment mode |
| `eval.json` | derived | session | offline collector | compare/report, cheap-regen cache |
| `agents/<name>/memory.akg` | agent | session | memory-capable agent (during play) | live memory store; re-export source |
| `agents/<name>/session-decisions.jsonl` | agent (decide) | session | TS `shared/pi-session.ts` | eval collector (tokens, tool calls, engagement), replay overlay |
| `agents/<name>/session-updates.jsonl` | agent (learn) | session | TS `shared/update.ts` | eval collector (update token cost) |
| `agents/<name>/memory-export.json` | derived | session | server (teardown, after agent shutdown) | fidelity, index-size checks, human inspection |
| `agents/<name>/stderr.log` / `stdout.log` | runtime | session | agent process | retry-metric extraction, debugging |

**Renamed traces (dual-read).** `session-decisions.jsonl` and `session-updates.jsonl` were formerly `pi-session.jsonl` and `update-session.jsonl`. New sessions write the current names; the Go reader accepts either, so archived sessions (incl. the committed phase-2 brackets) still load and reproduce. `source_artifacts` in `eval.json` records whichever name was found.

**Transient scratch (not artifacts).** `session-decisions-export-NNNN.jsonl` / `update-session-export-NNNN.jsonl` are written, appended into the canonical log, then removed; a survivor is crash debris and is gitignored. `diagnostics.jsonl` (a best-effort `graph_rot` log) was dropped — nothing in the toolchain read it.

## Authority and lifecycle

Two rules matter for downstream consumers:

1. `memory.akg` remains the authoritative durable memory store for replay or future re-export.
2. `eval.json` is a convenience summary. If it disagrees with `manifest.json`, `hands.jsonl`, `session-decisions.jsonl`, or `memory-export.json`, the source artifacts win.

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

## `session-decisions.jsonl`

### Path

`sessions/<id>/agents/<name>/session-decisions.jsonl` (was `pi-session.jsonl`; the reader still accepts the legacy name).

### Lifecycle

- Written by `persistSessionLog` in TS `shared/pi-session.ts` once per persisted Pi sub-session — once per decision in `decision` scope, once per hand in `hand` scope (the retrieval agents `llm-akg-durable` and `llm-md-wiki` use `hand` scope, so one persisted group holds every street decision of a hand).
- The sibling learn-phase trace `session-updates.jsonl` (was `update-session.jsonl`) has the same line grammar; it records the post-hand memory-update session rather than decisions.
- Append-only JSONL; safe to stream line-by-line (multi-MB). The `poker analyze hand`/`traces` CLI and the Go eval collector both read it this way.

### Line grammar

Each line is one JSON object. Two marker types delimit the stream; the rest are Pi transcript events.

```json
{"type":"hand_boundary","hand_number":3}
{"type":"session","version":3}
{"type":"message","message":{"role":"user","content":[{"type":"text","text":"Hand: 3\nStreet: turn ..."}]}}
{"type":"message","message":{"role":"assistant","content":[{"type":"thinking","text":"..."},{"type":"toolCall","name":"akg_get_node"}],"usage":{"input":700,"cacheRead":100,"totalTokens":820,"cost":{"total":0.004}}}}
{"type":"message","message":{"role":"toolResult","content":[{"type":"text","text":"node body ..."}]}}
{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"{\"action\":\"call\"}"}]}}
```

Field semantics:

- `hand_boundary` — first-class hand attribution. Emitted immediately before each persisted transcript, carrying the integer `hand_number`. Tools key off this field rather than grepping `"Hand: N"` prose. Legacy traces without the marker fall back to prose parsing.
- `session` — Pi sub-session delimiter (one per `exportToJsonl`). Token attribution flushes one row per `session`/`hand_boundary` group.
- `message` with `role: "user"` — one decision prompt. In `hand` scope a group has several, one per street decision.
- `message` with `role: "assistant"` — model turn. `content[].type == "toolCall"` (with `name`) is a memory read; the terminal assistant `text` carries the action JSON. `usage` carries token + cost accounting.
- `message` with `role: "toolResult"` — a tool's returned content. It is its own role, **not** a `user` message, so per-decision segmentation (split on `user` messages) is unambiguous.

A decision is one `user` message plus the assistant/`toolResult` turns up to the next `user` message. Because the action is the terminal assistant text, any read in a decision necessarily precedes it — this is what makes the deterministic memory-engagement metric (`seats[].memory_engagement` below) and the zero-reads tripwire computable without prose parsing.

> **Deferred:** a first-class `decision_index` field (per-decision addressing within a hand-scope group). Per-decision attribution is already recoverable by segmenting on `user` messages, so this is held until the Track B replay overlay needs a stable per-decision anchor.

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
      "pi_session": "agents/llm-akg-durable/session-decisions.jsonl",
      "memory_export": "agents/llm-akg-durable/memory-export.json",
      "stderr": "agents/llm-akg-durable/stderr.log"
    },
    "llm-stateless": {
      "pi_session": "agents/llm-stateless/session-decisions.jsonl",
      "memory_export": null,
      "stderr": null
    }
  }
}
```

Rules:

- `manifest` and `hands` are required relative paths.
- `agents` is keyed by agent directory name.
- `pi_session` is the relative path to the decision trace when present, otherwise `null`. The key name is historical; the path points to `session-decisions.jsonl` (or the legacy `pi-session.jsonl` for archived sessions).
- `memory_export` is the relative path to `memory-export.json` when present, otherwise `null`.
- `stderr` is the relative path to `stderr.log` when present, otherwise `null`.

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
    "akg_get_node": 76,
    "akg_list_nodes": 27,
    "akg_get_nodes": 4
  },
  "tool_calls_per_hand": {
    "akg_get_node": 3.04,
    "akg_list_nodes": 1.08,
    "akg_get_nodes": 0.16
  },
  "retry_metrics": {
    "attempt_failures": 2,
    "malformed_action_retries": 2,
    "exhausted_count": 1,
    "max_attempts_observed": 2
  },
  "memory_export": null,
  "memory_engagement": {
    "decisions": 385,
    "decisions_with_read": 48,
    "total_reads": 48,
    "reads_by_tool": { "akg_get_node": 41, "akg_get_nodes": 7 }
  }
}
```

Field semantics:

- `seat` — manifest seat number
- `name` — manifest seat strategy name
- `version` — manifest seat version string, or empty string when absent
- `chips_delta` — seat result from the manifest match result map
- `pi_session_present` — whether the seat's decision trace (`session-decisions.jsonl`, or legacy `pi-session.jsonl`) existed
- `decision_prompt_count` — count of decision prompts observed in that seat's decision trace; `0` when no decision trace exists
- `tool_calls` — map of tool name to count, derived from the decision trace's assistant messages where `content[].type == "toolCall"`
- `tool_calls_per_hand` — `tool_calls[name] / hand_count`, rounded only by normal JSON number formatting
- `retry_metrics` — per-seat summary derived from `stderr.log`; all counts are `0` when no retry log exists
- `memory_export` — `null` when no export exists, otherwise a lightweight graph summary object
- `memory_engagement` — present only when a decision trace exists; the per-decision retrieval-read footprint (see below). Omitted otherwise.

### `memory_engagement` object

```json
{
  "decisions": 385,
  "decisions_with_read": 48,
  "total_reads": 48,
  "reads_by_tool": { "akg_get_node": 41, "akg_get_nodes": 7 }
}
```

Derivation rules (from `session-decisions.jsonl`, deterministic, no LLM judging):

- `decisions` — count of decision prompts (`user` messages).
- `decisions_with_read` — decisions that recorded ≥1 memory read before acting. Coverage is `decisions_with_read / decisions`.
- `total_reads` — total memory reads across all decisions. Every tool call in a decision trace is a read, since write tools are registered only in the post-hand update session; the metric needs no per-agent read-tool table.
- `reads_by_tool` — read count keyed by tool name; omitted when zero.

A retrieval agent (`llm-akg-durable`, `llm-md-wiki`) with `total_reads == 0` over decisions is the hard-fail **tripwire** condition — memory was write-only. Non-retrieval strategies inject memory into the prompt and read nothing by design, so the tripwire is scoped to retrieval agents. The compare/report path raises a `TRIPWIRE FAIL` warning, scoped per session so one clean session cannot mask a broken one.

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

This example is illustrative and reflects the current artifact shapes in the repo. (Tool-call names shown are the durable agent's current open-vocabulary read tools; older archived sessions may carry the pre-rework `akg_get_opponent` / `akg_list_patterns` names.)

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
        "pi_session": "agents/llm-akg-durable/session-decisions.jsonl",
        "memory_export": null,
        "stderr": "agents/llm-akg-durable/stderr.log"
      },
      "llm-stateless": {
        "pi_session": "agents/llm-stateless/session-decisions.jsonl",
        "memory_export": null,
        "stderr": "agents/llm-stateless/stderr.log"
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
        "akg_get_node": 76,
        "akg_list_nodes": 27,
        "akg_get_nodes": 4
      },
      "tool_calls_per_hand": {
        "akg_get_node": 3.04,
        "akg_list_nodes": 1.08,
        "akg_get_nodes": 0.16
      },
      "retry_metrics": {
        "attempt_failures": 1,
        "malformed_action_retries": 1,
        "exhausted_count": 0,
        "max_attempts_observed": 2
      },
      "memory_export": null,
      "memory_engagement": {
        "decisions": 113,
        "decisions_with_read": 90,
        "total_reads": 107,
        "reads_by_tool": { "akg_get_node": 76, "akg_list_nodes": 27, "akg_get_nodes": 4 }
      }
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
      "retry_metrics": {
        "attempt_failures": 0,
        "malformed_action_retries": 0,
        "exhausted_count": 0,
        "max_attempts_observed": 0
      },
      "memory_export": null,
      "memory_engagement": {
        "decisions": 85,
        "decisions_with_read": 0,
        "total_reads": 0
      }
    }
  ]
}
```
