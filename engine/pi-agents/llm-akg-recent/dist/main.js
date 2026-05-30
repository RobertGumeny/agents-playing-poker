#!/usr/bin/env node
import { createStandardDecisionEngine, parsePositiveInteger, runPokerAgent, } from "@agent-poker/pi-agent-shared";
import { AkgMemoryPolicy } from "./memory.js";
const memoryPolicy = new AkgMemoryPolicy();
await runPokerAgent({
    memoryPolicy,
    decisionEngine: createStandardDecisionEngine({ sessionScope: "hand", memoryDirProvider: () => memoryPolicy.memoryDir }),
    agentVersion: "llm-akg-recent/0.1.0",
    maxDecisionAttempts: parsePositiveInteger(process.env.PI_POKER_MAX_DECISION_ATTEMPTS),
});
