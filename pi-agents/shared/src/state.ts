// Current session/hand state tracking shared by Pi poker agents.

import type { HandStartPayload, SessionInitPayload } from "./protocol.js";

export interface SessionState {
  sessionId: string;
  matchId: string;
  agentName: string;
  yourSeat: number;
  seats: Array<{ seat: number; name: string }>;
  memoryDir?: string;
}

export interface HandState {
  handNumber: number;
  dealerSeat: number;
  stacks: Record<string, number>;
  blindsPosted: Array<{ seat: number; amount: number }>;
  yourHoleCards: string[];
}

export interface AgentState {
  session?: SessionState;
  hand?: HandState;
}

export function createAgentState(): AgentState {
  return {};
}

export function applySessionInit(state: AgentState, payload: SessionInitPayload): void {
  state.session = {
    sessionId: payload.session_id,
    matchId: payload.match.match_id,
    agentName: payload.agent_name,
    yourSeat: payload.your_seat,
    seats: payload.seats,
    memoryDir: payload.memory_dir,
  };
}

export function applyHandStart(state: AgentState, payload: HandStartPayload): void {
  state.hand = {
    handNumber: payload.hand_number,
    dealerSeat: payload.dealer_seat,
    stacks: payload.stacks,
    blindsPosted: payload.blinds_posted,
    yourHoleCards: payload.your_hole_cards,
  };
}

export function resetHandState(state: AgentState): void {
  delete state.hand;
}
