// Current session/hand state tracking shared by Pi poker agents.

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
