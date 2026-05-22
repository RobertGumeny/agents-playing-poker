import { PiDecisionClient, runPokerAgent, type MemoryStrategy } from "@agent-poker/pi-agent-shared";

const strategy: MemoryStrategy = {
  name: "llm-nomemory",
  version: "llm-nomemory/0.1.0",
  async beforeDecision() {
    return { sections: [] };
  },
  async afterHandEnd() {
    // Deliberately forget prior hands. Durable Pi logs are observability artifacts,
    // not strategic memory exposed to future prompts.
  },
};

await runPokerAgent({
  strategy,
  decisionClient: new PiDecisionClient({ cwd: process.cwd() }),
});
