#!/usr/bin/env node

import {
  PiDecisionEngine,
  ScriptedDecisionEngine,
  parsePiThinkingLevel,
  runPokerAgent,
  type ActionPayload,
} from "@agent-poker/pi-agent-shared";

import { AkgMemoryPolicy } from "./memory.js";

const memoryPolicy = new AkgMemoryPolicy();

function createDecisionEngine() {
  const explicitSessionDir = process.env.PI_POKER_PI_SESSION_DIR;
  const sessionDirProvider = () => explicitSessionDir ?? memoryPolicy.memoryDir;
  const fakeDecisions = parseFakeDecisions(process.env.PI_POKER_FAKE_DECISIONS_JSON);
  if (fakeDecisions) {
    return new ScriptedDecisionEngine({
      decisions: fakeDecisions,
      sessionDirProvider,
      sessionScope: "hand",
    });
  }

  return new PiDecisionEngine({
    cwd: process.cwd(),
    sessionDirProvider,
    model: process.env.PI_POKER_MODEL,
    thinkingLevel: parsePiThinkingLevel(process.env.PI_POKER_THINKING_LEVEL),
    sessionScope: "hand",
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

await runPokerAgent({
  memoryPolicy,
  decisionEngine: createDecisionEngine(),
  agentVersion: "llm-akg-recent/0.1.0",
  maxDecisionAttempts: parsePositiveInteger(process.env.PI_POKER_MAX_DECISION_ATTEMPTS),
});
