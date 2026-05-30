import type { ActionHistoryEntry, ActionPayload, LegalActionOption, ShowdownEntry } from "./protocol.js";
import type { AgentState } from "./state.js";
export interface DecisionContext {
    state: AgentState;
    handNumber: number;
    street: string;
    board: string[];
    pot: number;
    toCall: number;
    stacks: Record<string, number>;
    actionHistory: ActionHistoryEntry[];
    legalActions: LegalActionOption[];
}
export interface HandStartContext {
    state: AgentState;
    handNumber: number;
    dealerSeat: number;
    stacks: Record<string, number>;
    blindsPosted: Array<{
        seat: number;
        amount: number;
    }>;
    yourHoleCards: string[];
}
export interface CompletedHandContext {
    state: AgentState;
    handNumber: number;
    dealerSeat: number;
    heroSeat: number;
    seats: Array<{
        seat: number;
        name: string;
    }>;
    heroHoleCards: string[];
    board: string[];
    actionHistory: ActionHistoryEntry[];
    showdownReached: boolean;
    showdown?: Record<string, ShowdownEntry>;
    result: Array<{
        seat: number;
        chips_delta: number;
    }>;
}
export interface PromptAugmentation {
    sections: string[];
}
export interface MemoryPolicy {
    beforeDecision(context: DecisionContext): Promise<PromptAugmentation>;
    afterHandEnd(context: CompletedHandContext): Promise<void>;
}
export interface DecisionRequest {
    context: DecisionContext;
    prompt: string;
    legalActions: LegalActionOption[];
}
export interface DecisionEngine {
    decide(request: DecisionRequest): Promise<ActionPayload>;
    onHandStart?(context: HandStartContext): Promise<void> | void;
    onHandEnd?(context: CompletedHandContext): Promise<void> | void;
    onSessionEnd?(): Promise<void> | void;
}
