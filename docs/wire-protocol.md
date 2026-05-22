# Wire protocol

This document is the implementer-facing contract for the v0 server↔agent stdio protocol described in [`spec.md`](spec.md).

For poker rules and terminology, do not infer behavior from this page alone. Use:
- [`docs/domain/texas-holdem.md`](domain/texas-holdem.md)
- [`docs/domain/heads-up-nlhe.md`](domain/heads-up-nlhe.md)

The server is authoritative for legal actions, pot/state accounting, and outcomes.

## Transport

- JSON Lines over stdio: exactly one JSON object per line.
- Server writes protocol messages to the agent's stdin.
- Agent writes protocol messages to stdout.
- Agent stderr is out-of-band debug output captured by the server.

## Envelope

Every protocol message uses this envelope:

```json
{ "v": 1, "type": "<message_type>", "id": "<message_id>", "payload": { ... } }
```

Optional reply correlation field:

```json
{ "in_reply_to": "<originating_message_id>" }
```

Envelope rules:
- `v` must be `1`.
- `type` must be a supported message type.
- `id` must be present.
- `payload` must always be present, including `{}` for empty payloads such as `session_end`.
- `in_reply_to` is required on reply messages that answer a specific server request.
- Treat incoming `id` values as opaque strings; when replying, copy the triggering server message's `id` into `in_reply_to`.

## Message flow

Normal lifecycle for one agent process:

1. Server sends `session_init`.
2. Agent replies with `session_ready`.
3. For each hand, server sends `hand_start`, then zero or more `your_turn` messages, then `hand_end`.
4. Server sends `session_end` once at match end.

Only `your_turn` expects an action decision. `hand_start` and `hand_end` are notifications.

## Server → agent messages

### `session_init`
Sent once at startup.

```json
{
  "v": 1,
  "type": "session_init",
  "id": "msg-1",
  "payload": {
    "session_id": "ses_2026-05-21_001",
    "agent_name": "llm-akg",
    "match": {
      "match_id": "mat_001",
      "seed": 12345,
      "hand_count": 200,
      "variant": "heads-up-nlhe",
      "info_realism": "showdown-only",
      "starting_stack": 200,
      "blinds": {"sb": 1, "bb": 2},
      "decision_deadline_ms": 30000
    },
    "seats": [
      {"seat": 0, "name": "llm-akg"},
      {"seat": 1, "name": "llm-fullhistory"}
    ],
    "your_seat": 0,
    "memory_dir": "/abs/path/to/agent/dir"
  }
}
```

Notes:
- `variant` is a project-level identifier, not a full rules description.
- `memory_dir` is the agent-owned durable working directory for match-local state.

### `hand_start`
Sent once per hand after blinds are posted.

```json
{
  "v": 1,
  "type": "hand_start",
  "id": "msg-2",
  "payload": {
    "hand_number": 47,
    "dealer_seat": 1,
    "stacks": {"0": 200, "1": 200},
    "blinds_posted": [{"seat": 0, "amount": 1}, {"seat": 1, "amount": 2}],
    "your_hole_cards": ["As", "Kh"]
  }
}
```

### `your_turn`
Sent whenever the agent must act.

```json
{
  "v": 1,
  "type": "your_turn",
  "id": "msg-3",
  "payload": {
    "hand_number": 47,
    "street": "flop",
    "board": ["Td", "9h", "2c"],
    "pot": 6,
    "to_call": 2,
    "stacks": {"0": 197, "1": 197},
    "seats": [
      {"seat": 0, "name": "llm-akg"},
      {"seat": 1, "name": "llm-fullhistory"}
    ],
    "action_history": [
      {"seat": 1, "action": "call", "amount": 1, "street": "preflop"},
      {"seat": 0, "action": "check", "street": "preflop"},
      {"seat": 0, "action": "bet", "amount": 2, "street": "flop"}
    ],
    "legal_actions": [
      {"action": "fold"},
      {"action": "call", "amount": 2},
      {"action": "raise", "min": 4, "max": 197}
    ]
  }
}
```

Notes:
- `seats` is always included; the protocol does not hard-code heads-up-only assumptions into the seat list shape.
- `action_history` is the server's record of completed actions in this hand.
- `legal_actions` is authoritative. Agents must choose from this list rather than recomputing legality locally.
- For `call`, `amount` is the exact amount to send back.
- For `raise`, `min` and `max` are the inclusive total wager bounds to send back as `amount`.

### `hand_end`
Sent once per hand after resolution.

```json
{
  "v": 1,
  "type": "hand_end",
  "id": "msg-4",
  "payload": {
    "hand_number": 47,
    "board": ["Td", "9h", "2c", "5s", "Kc"],
    "showdown": {
      "0": {"hole_cards": ["As", "Kh"], "rank": "two pair, kings and tens"},
      "1": {"hole_cards": ["9s", "9d"], "rank": "three of a kind, nines"}
    },
    "result": [{"seat": 1, "chips_delta": 14}, {"seat": 0, "chips_delta": -14}]
  }
}
```

Notes:
- In `showdown-only` information mode, non-revealed opponent cards are omitted unless shown at showdown.
- If no showdown occurs, `showdown` contains only revealed winner cards.
- In `perfect-info`, both players' hole cards are always included.

### `session_end`
Final message before shutdown.

```json
{
  "v": 1,
  "type": "session_end",
  "id": "msg-5",
  "payload": {}
}
```

Agents should use this to flush durable state and exit cleanly.

## Agent → server messages

### `session_ready`
Required reply to `session_init`.

```json
{
  "v": 1,
  "type": "session_ready",
  "id": "msg-6",
  "in_reply_to": "msg-1",
  "payload": {"version": "heuristic/0.1.0"}
}
```

`version` is persisted for session metadata.

### `action`
Required reply to each `your_turn`.

```json
{
  "v": 1,
  "type": "action",
  "id": "msg-7",
  "in_reply_to": "msg-3",
  "payload": {"action": "call", "amount": 2}
}
```

Action reply rules:
- `in_reply_to` must reference the triggering `your_turn` message.
- `action` must be one of the currently advertised `legal_actions`.
- For `fold` and `check`, omit `amount`.
- For `call`, send the exact `amount` provided by the server.
- For `bet` or `raise`, send an `amount` within the server-provided inclusive bounds.

In v0, implementers should primarily expect `fold`, `check`, `call`, and `raise` in `legal_actions`; the server remains the source of truth for exact availability.

### `log` (optional)
Optional structured log entry for capture in session artifacts.

```json
{
  "v": 1,
  "type": "log",
  "id": "msg-8",
  "payload": {
    "level": "info",
    "message": "raised turn blocker candidate",
    "fields": {"hand_number": 47, "street": "turn"}
  }
}
```

`log` does not reply to a specific server message.

## Timeouts and misbehavior

- The server advertises the decision deadline in `session_init.match.decision_deadline_ms`.
- If an `action` reply does not arrive before the deadline, the server records `auto_fold`, persists it in `hands.jsonl` with `forced_reason: "decision_timeout"`, and continues.
- If the agent process exits or crashes mid-match, the server aborts the match, marks it incomplete, and still persists partial artifacts.
- The server never trusts agent-reported game state. It computes legal actions, betting state, showdown results, and chip deltas itself.

## Implementer guidance

- Parse each line independently as one complete JSON message.
- Preserve unknown future fields if you proxy or log messages, but only rely on the fields defined here for v0.
- Use the domain docs for poker semantics and the server payloads for current legal decisions.
- Do not attempt to negotiate protocol version or alternate message shapes in v0.
