#!/usr/bin/env node

import {
  createStandardDecisionEngine,
  parsePositiveInteger,
  runPokerAgent,
} from "@agent-poker/pi-agent-shared";

import { FullHistoryMemoryPolicy } from "./history.js";

const memoryPolicy = new FullHistoryMemoryPolicy();

await runPokerAgent({
  memoryPolicy,
  decisionEngine: createStandardDecisionEngine({ sessionScope: "hand", memoryDirProvider: () => memoryPolicy.memoryDir }),
  agentVersion: "llm-fullhistory/0.1.0",
  maxDecisionAttempts: parsePositiveInteger(process.env.PI_POKER_MAX_DECISION_ATTEMPTS),
});
