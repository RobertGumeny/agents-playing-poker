// Memory strategy seam. The outer runner stays shared; implementations decide
// what, if any, prior-session information is exposed to the model.

import type { ActionPayload, LegalActionOption } from "./protocol";
import type { AgentState } from "./state";

export interface DecisionContext {
  state: AgentState;
  handNumber: number;
  street: string;
  board: string[];
  pot: number;
  toCall: number;
  stacks: Record<string, number>;
  actionHistory: unknown[];
  legalActions: LegalActionOption[];
}

export interface HandEndContext {
  state: AgentState;
  handNumber: number;
  board: string[];
  showdown?: unknown;
  result: Array<{ seat: number; chips_delta: number }>;
}

export interface PromptAugmentation {
  sections: string[];
}

export interface MemoryStrategy {
  name: string;
  version: string;
  beforeDecision(context: DecisionContext): Promise<PromptAugmentation>;
  afterHandEnd(context: HandEndContext): Promise<void>;
}

export interface DecisionClient {
  decide(prompt: string, legalActions: LegalActionOption[]): Promise<ActionPayload>;
}
