// Shared poker wire protocol types and JSONL helpers for Pi agents.
// This package intentionally mirrors docs/wire-protocol.md without importing Go internals.

export type MessageType =
  | "session_init"
  | "session_ready"
  | "hand_start"
  | "your_turn"
  | "action"
  | "hand_end"
  | "session_end"
  | "log";

export interface Envelope<TPayload = unknown> {
  v: 1;
  type: MessageType;
  id: string;
  in_reply_to?: string;
  payload: TPayload;
}

export interface LegalActionOption {
  action: "fold" | "check" | "call" | "bet" | "raise";
  amount?: number;
  min?: number;
  max?: number;
}

export interface ActionPayload {
  action: LegalActionOption["action"];
  amount?: number;
}

export function encodeEnvelope<TPayload>(envelope: Envelope<TPayload>): string {
  return `${JSON.stringify(envelope)}\n`;
}

export function decodeEnvelope(line: string): Envelope {
  return JSON.parse(line) as Envelope;
}
