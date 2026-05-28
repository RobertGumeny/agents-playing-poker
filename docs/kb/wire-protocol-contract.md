# Wire Protocol Contract

## Scope

The current protocol surface lives in:
- `internal/wire`
- [`../wire-protocol.md`](../wire-protocol.md)

It now provides:
- typed Go representations for every v0 protocol message
- envelope validation for protocol version, message type, message ID, required payload presence, and required reply correlation
- typed payload decoding from raw JSON envelopes
- round-trip JSON coverage for every message type
- malformed-envelope and malformed-payload negative tests across the full protocol surface
- a concise implementer contract in `docs/wire-protocol.md`

## Normative sources

The protocol implementation is intentionally anchored to repository docs instead of drifting into ad hoc JSON shapes:
- [`../wire-protocol.md`](../wire-protocol.md) for the authoritative implementer-facing contract
- [`../research.md`](../research.md) for the current match and strategy framing
- [`../domain/texas-holdem.md`](../domain/texas-holdem.md)
- [`../domain/heads-up-nlhe.md`](../domain/heads-up-nlhe.md)

The domain docs remain the source of poker semantics. The protocol docs define message shape and lifecycle only.

## `internal/wire`

`internal/wire` is the code-level contract between the future server/orchestrator and both Go and Pi agents.

### Envelope model

`Envelope` is the validation boundary for one JSONL message. Current enforced invariants:
- `v` must equal `ProtocolVersion` (`1`)
- `type` must be one of the supported v0 message types
- `id` must be present
- `payload` must always be present, including `{}` for empty payloads like `session_end`
- `in_reply_to` is required for reply messages that answer a specific server request

`DecodeEnvelope` handles unmarshal + validation, and `DecodePayload` handles typed payload decoding from the validated envelope.

### Typed messages and payloads

The package defines typed payloads and `Message[T]` aliases for all v0 protocol messages:
- server → agent: `session_init`, `hand_start`, `your_turn`, `hand_end`, `session_end`
- agent → server: `session_ready`, `action`, `log`

Important current shape decisions:
- cards are protocol strings like `As` and `Td`
- stack and showdown maps use seat numbers as JSON object keys
- `your_turn.seats` is always present even in heads-up play
- `session_ready` and `action` are the only message types that currently require reply correlation
- `log` is optional and not correlated to a specific request

### Reply-correlation policy

The current implementation treats reply correlation as a protocol requirement, not an application convention.

Required today:
- `session_ready.in_reply_to` must reference the originating `session_init`
- `action.in_reply_to` must reference the originating `your_turn`

This keeps the wire contract explicit before the server loop exists.

## Contract clarifications

A few implementation-significant ambiguities were resolved when the contract was first implemented:
- `your_turn.seats` is explicitly part of the payload shape
- `session_ready` is explicitly defined as the reply to `session_init`
- `session_end` carries an explicit empty payload object
- the optional structured `log` message is part of the documented contract

Future wire changes should amend `docs/wire-protocol.md` first instead of silently changing `internal/wire`.

## Test coverage

`internal/wire/types_test.go` covers:
- JSON round-trip for every message type
- malformed JSON rejection
- unsupported protocol version rejection
- unsupported message type rejection
- missing `id` rejection
- missing `payload` rejection
- missing required `in_reply_to` rejection
- malformed payload decoding rejection for every message type

## Current boundaries

This layer intentionally stops at the contract layer.

Not implemented here:
- stdio read/write loops
- server-side message dispatch
- timeout enforcement in a live match loop
- JSON Schema artifacts
- agent behavior beyond matching the documented message shapes

Those belong to later subsystem work and should be grounded in the focused docs for that layer.

## Integration contract

`internal/wire` plus `docs/wire-protocol.md` is the canonical v0 integration surface for:
- message names and payload fields
- reply-correlation behavior
- empty-payload handling
- envelope validation expectations
- serialization failure cases that must remain predictable

That separation matters because later match orchestration should be able to reject malformed agent traffic at the protocol boundary before any game-state logic runs.
