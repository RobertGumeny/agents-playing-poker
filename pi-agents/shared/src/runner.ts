// Shared poker-agent runner placeholder. This will own stdin/stdout JSONL,
// state updates, prompt construction, decision parsing, and hand-end hooks.

import type { DecisionClient, MemoryStrategy } from "./strategy";

export interface RunPokerAgentOptions {
  strategy: MemoryStrategy;
  decisionClient: DecisionClient;
  stdin?: NodeJS.ReadableStream;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
}

export async function runPokerAgent(_options: RunPokerAgentOptions): Promise<void> {
  // TODO: implement protocol loop against docs/wire-protocol.md.
  throw new Error("runPokerAgent is not implemented yet");
}
