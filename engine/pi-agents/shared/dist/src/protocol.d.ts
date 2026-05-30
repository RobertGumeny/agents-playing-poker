export declare const PROTOCOL_VERSION: 1;
export declare const MESSAGE_TYPES: readonly ["session_init", "session_ready", "hand_start", "your_turn", "action", "hand_end", "session_end", "log"];
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
    action_history: ActionHistoryEntry[];
    showdown_reached: boolean;
    showdown?: Record<string, ShowdownEntry>;
    result: Array<{
        seat: number;
        chips_delta: number;
    }>;
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
export declare function encodeEnvelope<TMessage extends ProtocolMessage>(envelope: TMessage): string;
export declare function decodeEnvelope(line: string): ProtocolMessage;
