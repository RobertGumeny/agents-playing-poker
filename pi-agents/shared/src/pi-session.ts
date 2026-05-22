// Pi SDK adapter placeholder. Implementation will create isolated decision sessions
// while writing durable Pi session logs as observability artifacts.

import type { ActionPayload, LegalActionOption } from "./protocol";
import type { DecisionClient } from "./strategy";

export interface PiDecisionClientOptions {
  cwd: string;
  sessionDir?: string;
  model?: string;
  thinkingLevel?: string;
}

export class PiDecisionClient implements DecisionClient {
  constructor(private readonly _options: PiDecisionClientOptions) {}

  async decide(_prompt: string, legalActions: LegalActionOption[]): Promise<ActionPayload> {
    // TODO: wire to @earendil-works/pi-coding-agent createAgentSession().
    const check = legalActions.find((action) => action.action === "check");
    if (check) return { action: "check" };
    const fold = legalActions.find((action) => action.action === "fold");
    if (fold) return { action: "fold" };
    throw new Error("PiDecisionClient is not implemented yet and no fallback action exists");
  }
}
