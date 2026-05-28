#!/usr/bin/env node

import { runPokerAgent } from "@agent-poker/pi-agent-shared";

import { AkgDurableMemoryPolicy } from "./memory.js";
import { createDecisionEngine, parsePositiveInteger } from "./runtime.js";

const memoryPolicy = new AkgDurableMemoryPolicy();

await runPokerAgent({
  memoryPolicy,
  decisionEngine: createDecisionEngine(memoryPolicy, {
    cwd: process.cwd(),
    sessionDir: process.env.PI_POKER_PI_SESSION_DIR,
    model: process.env.PI_POKER_MODEL,
    thinkingLevel: process.env.PI_POKER_THINKING_LEVEL,
    fakeDecisionsJSON: process.env.PI_POKER_FAKE_DECISIONS_JSON,
  }),
  agentVersion: "llm-akg-durable@exp-0.2.0-once-per-hand",
  maxDecisionAttempts: parsePositiveInteger(process.env.PI_POKER_MAX_DECISION_ATTEMPTS),
});
