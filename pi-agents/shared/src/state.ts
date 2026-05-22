// Current session/hand state tracking shared by Pi poker agents.

import type { HandStartPayload, MatchConfig, SessionInitPayload, YourTurnPayload } from "./protocol.js";

export interface SessionState {
  sessionId: string;
  matchId: string;
  match: MatchConfig;
  agentName: string;
  yourSeat: number;
  seats: Array<{ seat: number; name: string }>;
  memoryDir?: string;
}

export interface TurnState {
  street: string;
  board: string[];
  pot: number;
  toCall: number;
  stacks: Record<string, number>;
  seats: Array<{ seat: number; name: string }>;
  actionHistory: YourTurnPayload["action_history"];
  legalActions: YourTurnPayload["legal_actions"];
}

export interface HandState {
  handNumber: number;
  dealerSeat: number;
  stacks: Record<string, number>;
  blindsPosted: Array<{ seat: number; amount: number }>;
  yourHoleCards: string[];
  currentTurn?: TurnState;
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
    match: {
      ...payload.match,
      blinds: { ...payload.match.blinds },
    },
    agentName: payload.agent_name,
    yourSeat: payload.your_seat,
    seats: payload.seats.map((seat) => ({ ...seat })),
    memoryDir: payload.memory_dir,
  };
  resetHandState(state);
}

export function applyHandStart(state: AgentState, payload: HandStartPayload): void {
  state.hand = {
    handNumber: payload.hand_number,
    dealerSeat: payload.dealer_seat,
    stacks: { ...payload.stacks },
    blindsPosted: payload.blinds_posted.map((blind) => ({ ...blind })),
    yourHoleCards: [...payload.your_hole_cards],
  };
}

export function applyYourTurn(state: AgentState, payload: YourTurnPayload): void {
  if (!state.hand || state.hand.handNumber !== payload.hand_number) {
    state.hand = {
      handNumber: payload.hand_number,
      dealerSeat: -1,
      stacks: { ...payload.stacks },
      blindsPosted: [],
      yourHoleCards: [],
    };
  }

  state.hand.currentTurn = {
    street: payload.street,
    board: [...payload.board],
    pot: payload.pot,
    toCall: payload.to_call,
    stacks: { ...payload.stacks },
    seats: payload.seats.map((seat) => ({ ...seat })),
    actionHistory: payload.action_history.map((entry) => ({ ...entry })),
    legalActions: payload.legal_actions.map((action) => ({ ...action })),
  };
  state.hand.stacks = { ...payload.stacks };
}

export function resetHandState(state: AgentState): void {
  delete state.hand;
}

export function resetSessionState(state: AgentState): void {
  resetHandState(state);
  delete state.session;
}
