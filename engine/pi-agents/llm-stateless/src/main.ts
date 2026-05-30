#!/usr/bin/env node

import {
  createStandardDecisionEngine,
  parsePositiveInteger,
  runPokerAgent,
  type MemoryPolicy,
} from "@agent-poker/pi-agent-shared";

let serverMemoryDir: string | undefined;

const memoryPolicy: MemoryPolicy = {
  async beforeDecision(context) {
    serverMemoryDir = context.state.session?.memoryDir;
    return { sections: [] };
  },
  async afterHandEnd() {
    // Deliberately forget prior hands. Durable Pi logs are observability artifacts,
    // not strategic memory exposed to future prompts.
  },
};

await runPokerAgent({
  memoryPolicy,
  decisionEngine: createStandardDecisionEngine({ sessionScope: "decision", memoryDirProvider: () => serverMemoryDir }),
  agentVersion: "llm-stateless/0.1.0",
  maxDecisionAttempts: parsePositiveInteger(process.env.PI_POKER_MAX_DECISION_ATTEMPTS),
});
