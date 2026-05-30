import type { DecisionEngine, MemoryPolicy } from "./strategy.js";
export interface RunPokerAgentOptions {
    memoryPolicy: MemoryPolicy;
    decisionEngine: DecisionEngine;
    stdin?: NodeJS.ReadableStream;
    stdout?: NodeJS.WritableStream;
    stderr?: NodeJS.WritableStream;
    maxDecisionAttempts?: number;
    agentVersion: string;
}
export declare function runPokerAgent(options: RunPokerAgentOptions): Promise<void>;
