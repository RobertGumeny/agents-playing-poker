import type { HandStartPayload, MatchConfig, SessionInitPayload, YourTurnPayload } from "./protocol.js";
export interface SessionState {
    sessionId: string;
    matchId: string;
    match: MatchConfig;
    agentName: string;
    yourSeat: number;
    seats: Array<{
        seat: number;
        name: string;
    }>;
    memoryDir?: string;
}
export interface TurnState {
    street: string;
    board: string[];
    pot: number;
    toCall: number;
    stacks: Record<string, number>;
    seats: Array<{
        seat: number;
        name: string;
    }>;
    actionHistory: YourTurnPayload["action_history"];
    legalActions: YourTurnPayload["legal_actions"];
}
export interface HandState {
    handNumber: number;
    dealerSeat: number;
    stacks: Record<string, number>;
    blindsPosted: Array<{
        seat: number;
        amount: number;
    }>;
    yourHoleCards: string[];
    currentTurn?: TurnState;
}
export interface AgentState {
    session?: SessionState;
    hand?: HandState;
}
export declare function createAgentState(): AgentState;
export declare function applySessionInit(state: AgentState, payload: SessionInitPayload): void;
export declare function applyHandStart(state: AgentState, payload: HandStartPayload): void;
export declare function applyYourTurn(state: AgentState, payload: YourTurnPayload): void;
export declare function resetHandState(state: AgentState): void;
export declare function resetSessionState(state: AgentState): void;
