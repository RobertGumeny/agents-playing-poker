import { PiDecisionClient, parsePiThinkingLevel, runPokerAgent, type MemoryStrategy } from "@agent-poker/pi-agent-shared";

const strategy: MemoryStrategy = {
  name: "llm-stateless",
  version: "llm-stateless/0.1.0",
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
  decisionClient: new PiDecisionClient({
    cwd: process.cwd(),
    sessionDir: process.env.PI_POKER_PI_SESSION_DIR,
    model: process.env.PI_POKER_MODEL,
    thinkingLevel: parsePiThinkingLevel(process.env.PI_POKER_THINKING_LEVEL),
  }),
  maxDecisionAttempts: parsePositiveInteger(process.env.PI_POKER_MAX_DECISION_ATTEMPTS),
});

function parsePositiveInteger(value: string | undefined): number | undefined {
  if (value === undefined || value.length === 0) return undefined;

  const parsed = Number.parseInt(value, 10);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`invalid positive integer ${JSON.stringify(value)}`);
  }

  return parsed;
}
