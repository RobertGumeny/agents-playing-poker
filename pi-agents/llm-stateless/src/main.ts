#!/usr/bin/env node

import { appendFile, mkdir } from "node:fs/promises";
import path from "node:path";

import {
  PiDecisionClient,
  parsePiThinkingLevel,
  runPokerAgent,
  type ActionPayload,
  type DecisionClient,
  type LegalActionOption,
  type MemoryStrategy,
} from "@agent-poker/pi-agent-shared";

let serverMemoryDir: string | undefined;

const strategy: MemoryStrategy = {
  name: "llm-stateless",
  version: "llm-stateless/0.1.0",
  async beforeDecision(context) {
    serverMemoryDir = context.state.session?.memoryDir;
    return { sections: [] };
  },
  async afterHandEnd() {
    // Deliberately forget prior hands. Durable Pi logs are observability artifacts,
    // not strategic memory exposed to future prompts.
  },
};

function createDecisionClient(): DecisionClient {
  const explicitSessionDir = process.env.PI_POKER_PI_SESSION_DIR;
  const sessionDirProvider = () => explicitSessionDir ?? serverMemoryDir;
  const fakeDecisions = parseFakeDecisions(process.env.PI_POKER_FAKE_DECISIONS_JSON);
  if (fakeDecisions) {
    return new ScriptedDecisionClient(fakeDecisions, sessionDirProvider);
  }

  return new PiDecisionClient({
    cwd: process.cwd(),
    sessionDirProvider,
    model: process.env.PI_POKER_MODEL,
    thinkingLevel: parsePiThinkingLevel(process.env.PI_POKER_THINKING_LEVEL),
  });
}

function parsePositiveInteger(value: string | undefined): number | undefined {
  if (value === undefined || value.length === 0) return undefined;

  const parsed = Number.parseInt(value, 10);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`invalid positive integer ${JSON.stringify(value)}`);
  }

  return parsed;
}

function parseFakeDecisions(value: string | undefined): ActionPayload[] | undefined {
  if (value === undefined || value.length === 0) return undefined;

  let parsed: unknown;
  try {
    parsed = JSON.parse(value);
  } catch (error) {
    throw new Error(`invalid PI_POKER_FAKE_DECISIONS_JSON: ${error instanceof Error ? error.message : String(error)}`);
  }

  if (!Array.isArray(parsed)) {
    throw new Error("invalid PI_POKER_FAKE_DECISIONS_JSON: expected JSON array");
  }

  return parsed.map((entry, index) => parseFakeDecision(entry, index));
}

function parseFakeDecision(entry: unknown, index: number): ActionPayload {
  if (!isRecord(entry)) {
    throw new Error(`invalid PI_POKER_FAKE_DECISIONS_JSON[${index}]: expected object`);
  }
  const action = entry.action;
  if (action !== "fold" && action !== "check" && action !== "call" && action !== "bet" && action !== "raise") {
    throw new Error(`invalid PI_POKER_FAKE_DECISIONS_JSON[${index}].action`);
  }
  const rawAmount = entry.amount;
  if (rawAmount === undefined) {
    return { action };
  }
  if (typeof rawAmount !== "number" || !Number.isInteger(rawAmount) || rawAmount < 0) {
    throw new Error(`invalid PI_POKER_FAKE_DECISIONS_JSON[${index}].amount`);
  }
  return { action, amount: rawAmount };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

class ScriptedDecisionClient implements DecisionClient {
  private index = 0;

  constructor(
    private readonly decisions: ActionPayload[],
    private readonly sessionDirProvider: () => string | undefined,
  ) {}

  async decide(prompt: string, legalActions: LegalActionOption[]): Promise<ActionPayload> {
    const decisionNumber = this.index + 1;
    const decision = this.decisions[Math.min(this.index, this.decisions.length - 1)];
    this.index += 1;
    if (!decision) {
      throw new Error("no scripted decision available");
    }

    const matched = legalActions.find((action) => {
      if (action.action !== decision.action) return false;
      if (decision.amount !== undefined) {
        return action.amount === decision.amount || (action.min !== undefined && action.max !== undefined && decision.amount >= action.min && decision.amount <= action.max);
      }
      return true;
    });
    if (!matched) {
      throw new Error(`scripted decision ${JSON.stringify(decision)} is not legal for this turn`);
    }

    await this.writeObservabilityLog(prompt, legalActions, decisionNumber, decision);
    return decision;
  }

  private async writeObservabilityLog(
    prompt: string,
    legalActions: LegalActionOption[],
    decisionNumber: number,
    decision: ActionPayload,
  ): Promise<void> {
    const sessionDir = this.sessionDirProvider();
    if (!sessionDir) return;

    await mkdir(sessionDir, { recursive: true });
    await appendFile(
      path.join(sessionDir, "pi-session.jsonl"),
      `${JSON.stringify({
        type: "fake_pi_session",
        decision_number: decisionNumber,
        legal_actions: legalActions,
        selected_action: decision,
        prompt,
      })}\n`,
      "utf8",
    );
  }
}

await runPokerAgent({
  strategy,
  decisionClient: createDecisionClient(),
  maxDecisionAttempts: parsePositiveInteger(process.env.PI_POKER_MAX_DECISION_ATTEMPTS),
});
