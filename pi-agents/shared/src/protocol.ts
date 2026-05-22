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

export interface SeatDescriptor {
  seat: number;
  name: string;
}

export interface BlindAmount {
  seat: number;
  amount: number;
}

export interface MatchConfig {
  match_id: string;
  seed: number;
  hand_count: number;
  variant: string;
  info_realism: string;
  starting_stack: number;
  blinds: {
    sb: number;
    bb: number;
  };
  decision_deadline_ms: number;
}

export interface SessionInitPayload {
  session_id: string;
  agent_name: string;
  match: MatchConfig;
  seats: SeatDescriptor[];
  your_seat: number;
  memory_dir: string;
}

export interface HandStartPayload {
  hand_number: number;
  dealer_seat: number;
  stacks: Record<string, number>;
  blinds_posted: BlindAmount[];
  your_hole_cards: string[];
}

export interface ActionHistoryEntry {
  seat: number;
  action: string;
  amount?: number;
  street: string;
  forced_reason?: string;
}

export interface LegalActionOption {
  action: "fold" | "check" | "call" | "bet" | "raise";
  amount?: number;
  min?: number;
  max?: number;
}

export interface YourTurnPayload {
  hand_number: number;
  street: string;
  board: string[];
  pot: number;
  to_call: number;
  stacks: Record<string, number>;
  seats: SeatDescriptor[];
  action_history: ActionHistoryEntry[];
  legal_actions: LegalActionOption[];
}

export interface ShowdownEntry {
  hole_cards: string[];
  rank: string;
}

export interface HandEndPayload {
  hand_number: number;
  board: string[];
  showdown?: Record<string, ShowdownEntry>;
  result: Array<{ seat: number; chips_delta: number }>;
}

export interface SessionReadyPayload {
  version: string;
}

export interface ActionPayload {
  action: LegalActionOption["action"];
  amount?: number;
}

export interface LogPayload {
  level: string;
  message: string;
  fields?: Record<string, unknown>;
}

export function encodeEnvelope<TPayload>(envelope: Envelope<TPayload>): string {
  return `${JSON.stringify(envelope)}\n`;
}

export function decodeEnvelope(line: string): Envelope {
  return JSON.parse(line.trim()) as Envelope;
}
