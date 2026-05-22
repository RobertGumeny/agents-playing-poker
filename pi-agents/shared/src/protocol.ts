// Shared poker wire protocol types and JSONL helpers for Pi agents.
// This package intentionally mirrors docs/wire-protocol.md without importing Go internals.

export const PROTOCOL_VERSION = 1 as const;

export const MESSAGE_TYPES = [
  "session_init",
  "session_ready",
  "hand_start",
  "your_turn",
  "action",
  "hand_end",
  "session_end",
  "log",
] as const;

export type MessageType = (typeof MESSAGE_TYPES)[number];
export type Street = "preflop" | "flop" | "turn" | "river";
export type PlayerAction = "post_blind" | "fold" | "check" | "call" | "bet" | "raise" | "auto_check" | "auto_fold";
export type DecisionAction = "fold" | "check" | "call" | "bet" | "raise";

export interface Envelope<TType extends MessageType = MessageType, TPayload = unknown> {
  v: typeof PROTOCOL_VERSION;
  type: TType;
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
  memory_dir?: string;
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
  action: PlayerAction;
  amount?: number;
  street: Street;
  forced_reason?: string;
}

export interface LegalActionOption {
  action: DecisionAction;
  amount?: number;
  min?: number;
  max?: number;
}

export interface YourTurnPayload {
  hand_number: number;
  street: Street;
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

export interface SessionEndPayload {
  [key: string]: never;
}

export interface SessionReadyPayload {
  version: string;
}

export interface ActionPayload {
  action: DecisionAction;
  amount?: number;
}

export interface LogPayload {
  level: string;
  message: string;
  fields?: Record<string, unknown>;
}

export type SessionInitMessage = Envelope<"session_init", SessionInitPayload>;
export type SessionReadyMessage = Envelope<"session_ready", SessionReadyPayload>;
export type HandStartMessage = Envelope<"hand_start", HandStartPayload>;
export type YourTurnMessage = Envelope<"your_turn", YourTurnPayload>;
export type ActionMessage = Envelope<"action", ActionPayload>;
export type HandEndMessage = Envelope<"hand_end", HandEndPayload>;
export type SessionEndMessage = Envelope<"session_end", SessionEndPayload>;
export type LogMessage = Envelope<"log", LogPayload>;

export type ServerToAgentMessage = SessionInitMessage | HandStartMessage | YourTurnMessage | HandEndMessage | SessionEndMessage;
export type AgentToServerMessage = SessionReadyMessage | ActionMessage | LogMessage;
export type ProtocolMessage = ServerToAgentMessage | AgentToServerMessage;

export function encodeEnvelope<TMessage extends ProtocolMessage>(envelope: TMessage): string {
  validateEnvelope(envelope);
  return `${JSON.stringify(envelope)}\n`;
}

export function decodeEnvelope(line: string): ProtocolMessage {
  const normalizedLine = normalizeSingleJSONLine(line);

  let parsed: unknown;
  try {
    parsed = JSON.parse(normalizedLine);
  } catch (error) {
    throw new Error(`decode envelope: ${error instanceof Error ? error.message : String(error)}`);
  }

  validateEnvelope(parsed);
  return parsed;
}

function normalizeSingleJSONLine(line: string): string {
  if (line.length === 0) {
    throw new Error("decode envelope: empty line");
  }

  let normalized = line;
  if (normalized.endsWith("\n")) {
    normalized = normalized.slice(0, -1);
  }
  if (normalized.endsWith("\r")) {
    normalized = normalized.slice(0, -1);
  }

  if (normalized.length === 0) {
    throw new Error("decode envelope: empty line");
  }
  if (normalized.includes("\n") || normalized.includes("\r")) {
    throw new Error("decode envelope: expected exactly one JSON object per line");
  }

  return normalized;
}

function validateEnvelope(value: unknown): asserts value is ProtocolMessage {
  const envelope = expectRecord(value, "envelope");

  if (envelope.v !== PROTOCOL_VERSION) {
    throw new Error(`validate envelope: unsupported protocol version ${String(envelope.v)}`);
  }

  if (!isMessageType(envelope.type)) {
    throw new Error(`validate envelope: unsupported message type ${JSON.stringify(envelope.type)}`);
  }

  if (!isNonEmptyString(envelope.id)) {
    throw new Error("validate envelope: missing id");
  }

  if (!Object.hasOwn(envelope, "payload")) {
    throw new Error("validate envelope: missing payload");
  }

  if (Object.hasOwn(envelope, "in_reply_to") && envelope.in_reply_to !== undefined && typeof envelope.in_reply_to !== "string") {
    throw new Error("validate envelope: in_reply_to must be a string");
  }

  if (requiresReplyCorrelation(envelope.type) && !isNonEmptyString(envelope.in_reply_to)) {
    throw new Error(`validate envelope: message type ${JSON.stringify(envelope.type)} requires in_reply_to`);
  }

  validatePayload(envelope.type, envelope.payload);
}

function validatePayload(type: MessageType, payload: unknown): void {
  switch (type) {
    case "session_init":
      validateSessionInitPayload(payload);
      return;
    case "session_ready":
      validateSessionReadyPayload(payload);
      return;
    case "hand_start":
      validateHandStartPayload(payload);
      return;
    case "your_turn":
      validateYourTurnPayload(payload);
      return;
    case "action":
      validateActionPayload(payload);
      return;
    case "hand_end":
      validateHandEndPayload(payload);
      return;
    case "session_end":
      validateSessionEndPayload(payload);
      return;
    case "log":
      validateLogPayload(payload);
      return;
  }
}

function validateSessionInitPayload(value: unknown): asserts value is SessionInitPayload {
  const payload = expectRecord(value, "session_init payload");
  expectString(payload.session_id, "session_init payload.session_id");
  expectString(payload.agent_name, "session_init payload.agent_name");
  validateMatchConfig(payload.match);
  validateSeatDescriptors(payload.seats, "session_init payload.seats");
  expectNumber(payload.your_seat, "session_init payload.your_seat");
  if (payload.memory_dir !== undefined) {
    expectString(payload.memory_dir, "session_init payload.memory_dir");
  }
}

function validateMatchConfig(value: unknown): asserts value is MatchConfig {
  const match = expectRecord(value, "session_init payload.match");
  expectString(match.match_id, "session_init payload.match.match_id");
  expectNumber(match.seed, "session_init payload.match.seed");
  expectNumber(match.hand_count, "session_init payload.match.hand_count");
  expectString(match.variant, "session_init payload.match.variant");
  expectString(match.info_realism, "session_init payload.match.info_realism");
  expectNumber(match.starting_stack, "session_init payload.match.starting_stack");
  const blinds = expectRecord(match.blinds, "session_init payload.match.blinds");
  expectNumber(blinds.sb, "session_init payload.match.blinds.sb");
  expectNumber(blinds.bb, "session_init payload.match.blinds.bb");
  expectNumber(match.decision_deadline_ms, "session_init payload.match.decision_deadline_ms");
}

function validateHandStartPayload(value: unknown): asserts value is HandStartPayload {
  const payload = expectRecord(value, "hand_start payload");
  expectNumber(payload.hand_number, "hand_start payload.hand_number");
  expectNumber(payload.dealer_seat, "hand_start payload.dealer_seat");
  validateChipMap(payload.stacks, "hand_start payload.stacks");
  validateBlindAmounts(payload.blinds_posted, "hand_start payload.blinds_posted");
  validateStringArray(payload.your_hole_cards, "hand_start payload.your_hole_cards");
}

function validateYourTurnPayload(value: unknown): asserts value is YourTurnPayload {
  const payload = expectRecord(value, "your_turn payload");
  expectNumber(payload.hand_number, "your_turn payload.hand_number");
  expectStreet(payload.street, "your_turn payload.street");
  validateStringArray(payload.board, "your_turn payload.board");
  expectNumber(payload.pot, "your_turn payload.pot");
  expectNumber(payload.to_call, "your_turn payload.to_call");
  validateChipMap(payload.stacks, "your_turn payload.stacks");
  validateSeatDescriptors(payload.seats, "your_turn payload.seats");
  validateActionHistory(payload.action_history, "your_turn payload.action_history");
  validateLegalActions(payload.legal_actions, "your_turn payload.legal_actions");
}

function validateHandEndPayload(value: unknown): asserts value is HandEndPayload {
  const payload = expectRecord(value, "hand_end payload");
  expectNumber(payload.hand_number, "hand_end payload.hand_number");
  validateStringArray(payload.board, "hand_end payload.board");
  if (payload.showdown !== undefined) {
    validateShowdown(payload.showdown, "hand_end payload.showdown");
  }
  validateHandResults(payload.result, "hand_end payload.result");
}

function validateSessionEndPayload(value: unknown): asserts value is SessionEndPayload {
  expectRecord(value, "session_end payload");
}

function validateSessionReadyPayload(value: unknown): asserts value is SessionReadyPayload {
  const payload = expectRecord(value, "session_ready payload");
  expectString(payload.version, "session_ready payload.version");
}

function validateActionPayload(value: unknown): asserts value is ActionPayload {
  const payload = expectRecord(value, "action payload");
  expectDecisionAction(payload.action, "action payload.action");
  if (payload.amount !== undefined) {
    expectNumber(payload.amount, "action payload.amount");
  }
}

function validateLogPayload(value: unknown): asserts value is LogPayload {
  const payload = expectRecord(value, "log payload");
  expectString(payload.level, "log payload.level");
  expectString(payload.message, "log payload.message");
  if (payload.fields !== undefined) {
    expectRecord(payload.fields, "log payload.fields");
  }
}

function validateSeatDescriptors(value: unknown, path: string): asserts value is SeatDescriptor[] {
  const seats = expectArray(value, path);
  for (const [index, seat] of seats.entries()) {
    const item = expectRecord(seat, `${path}[${index}]`);
    expectNumber(item.seat, `${path}[${index}].seat`);
    expectString(item.name, `${path}[${index}].name`);
  }
}

function validateBlindAmounts(value: unknown, path: string): asserts value is BlindAmount[] {
  const blinds = expectArray(value, path);
  for (const [index, blind] of blinds.entries()) {
    const item = expectRecord(blind, `${path}[${index}]`);
    expectNumber(item.seat, `${path}[${index}].seat`);
    expectNumber(item.amount, `${path}[${index}].amount`);
  }
}

function validateActionHistory(value: unknown, path: string): asserts value is ActionHistoryEntry[] {
  const history = expectArray(value, path);
  for (const [index, entry] of history.entries()) {
    const item = expectRecord(entry, `${path}[${index}]`);
    expectNumber(item.seat, `${path}[${index}].seat`);
    expectPlayerAction(item.action, `${path}[${index}].action`);
    if (item.amount !== undefined) {
      expectNumber(item.amount, `${path}[${index}].amount`);
    }
    expectStreet(item.street, `${path}[${index}].street`);
    if (item.forced_reason !== undefined) {
      expectString(item.forced_reason, `${path}[${index}].forced_reason`);
    }
  }
}

function validateLegalActions(value: unknown, path: string): asserts value is LegalActionOption[] {
  const actions = expectArray(value, path);
  for (const [index, action] of actions.entries()) {
    const item = expectRecord(action, `${path}[${index}]`);
    expectDecisionAction(item.action, `${path}[${index}].action`);
    if (item.amount !== undefined) {
      expectNumber(item.amount, `${path}[${index}].amount`);
    }
    if (item.min !== undefined) {
      expectNumber(item.min, `${path}[${index}].min`);
    }
    if (item.max !== undefined) {
      expectNumber(item.max, `${path}[${index}].max`);
    }
  }
}

function validateShowdown(value: unknown, path: string): asserts value is Record<string, ShowdownEntry> {
  const showdown = expectRecord(value, path);
  for (const [seat, entry] of Object.entries(showdown)) {
    const item = expectRecord(entry, `${path}.${seat}`);
    validateStringArray(item.hole_cards, `${path}.${seat}.hole_cards`);
    expectString(item.rank, `${path}.${seat}.rank`);
  }
}

function validateHandResults(value: unknown, path: string): asserts value is HandEndPayload["result"] {
  const results = expectArray(value, path);
  for (const [index, result] of results.entries()) {
    const item = expectRecord(result, `${path}[${index}]`);
    expectNumber(item.seat, `${path}[${index}].seat`);
    expectNumber(item.chips_delta, `${path}[${index}].chips_delta`);
  }
}

function validateChipMap(value: unknown, path: string): asserts value is Record<string, number> {
  const chips = expectRecord(value, path);
  for (const [seat, amount] of Object.entries(chips)) {
    expectNumber(amount, `${path}.${seat}`);
  }
}

function validateStringArray(value: unknown, path: string): asserts value is string[] {
  const items = expectArray(value, path);
  for (const [index, item] of items.entries()) {
    expectString(item, `${path}[${index}]`);
  }
}

function expectRecord(value: unknown, path: string): Record<string, unknown> {
  if (!isRecord(value)) {
    throw new Error(`decode ${path}: expected object`);
  }
  return value;
}

function expectArray(value: unknown, path: string): unknown[] {
  if (!Array.isArray(value)) {
    throw new Error(`decode ${path}: expected array`);
  }
  return value;
}

function expectString(value: unknown, path: string): string {
  if (typeof value !== "string") {
    throw new Error(`decode ${path}: expected string`);
  }
  return value;
}

function expectNumber(value: unknown, path: string): number {
  if (typeof value !== "number" || Number.isNaN(value)) {
    throw new Error(`decode ${path}: expected number`);
  }
  return value;
}

function expectStreet(value: unknown, path: string): Street {
  if (value !== "preflop" && value !== "flop" && value !== "turn" && value !== "river") {
    throw new Error(`decode ${path}: expected street`);
  }
  return value;
}

function expectDecisionAction(value: unknown, path: string): DecisionAction {
  if (value !== "fold" && value !== "check" && value !== "call" && value !== "bet" && value !== "raise") {
    throw new Error(`decode ${path}: expected decision action`);
  }
  return value;
}

function expectPlayerAction(value: unknown, path: string): PlayerAction {
  if (
    value !== "post_blind" &&
    value !== "fold" &&
    value !== "check" &&
    value !== "call" &&
    value !== "bet" &&
    value !== "raise" &&
    value !== "auto_check" &&
    value !== "auto_fold"
  ) {
    throw new Error(`decode ${path}: expected player action`);
  }
  return value;
}

function requiresReplyCorrelation(type: MessageType): boolean {
  return type === "session_ready" || type === "action";
}

function isMessageType(value: unknown): value is MessageType {
  return typeof value === "string" && MESSAGE_TYPES.includes(value as MessageType);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.length > 0;
}
