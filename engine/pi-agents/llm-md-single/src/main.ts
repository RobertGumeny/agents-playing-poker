#!/usr/bin/env node

import {
  createStandardDecisionEngine,
  parsePositiveInteger,
  runPokerAgent,
} from "@agent-poker/pi-agent-shared";

import { MarkdownSingleMemoryPolicy } from "./memory.js";

const memoryPolicy = new MarkdownSingleMemoryPolicy();

await runPokerAgent({
  memoryPolicy,
  decisionEngine: createStandardDecisionEngine({ sessionScope: "hand", memoryDirProvider: () => memoryPolicy.memoryDir }),
  agentVersion: "llm-md-single/0.1.0",
  maxDecisionAttempts: parsePositiveInteger(process.env.PI_POKER_MAX_DECISION_ATTEMPTS),
});
