import { type AgentSessionEvent, type CreateAgentSessionOptions } from "@earendil-works/pi-coding-agent";
import { parsePositiveInteger, PiDecisionEngine, ScriptedDecisionEngine } from "@agent-poker/pi-agent-shared";
import type { MarkdownWikiMemoryPolicy } from "./memory.js";
export declare const WIKI_DECISION_SYSTEM_PROMPT: string;
type PiSession = {
    prompt(text: string): Promise<void>;
    subscribe(listener: (event: AgentSessionEvent) => void): () => void;
    getLastAssistantText(): string | undefined;
    exportToJsonl(outputPath?: string): string;
    dispose(): void;
};
type PiSessionFactory = (options: {
    cwd: string;
    agentDir: string;
    sessionDir?: string;
    model?: string;
    thinkingLevel?: NonNullable<CreateAgentSessionOptions["thinkingLevel"]>;
}) => Promise<PiSession>;
export interface CreateDecisionEngineOptions {
    cwd?: string;
    sessionDir?: string;
    model?: string;
    thinkingLevel?: string;
    fakeDecisionsJSON?: string;
    sessionFactory?: PiSessionFactory;
}
export declare function createWikiSessionFactory(memoryPolicy: Pick<MarkdownWikiMemoryPolicy, "getWikiDir">): PiSessionFactory;
export declare function createDecisionEngine(memoryPolicy: Pick<MarkdownWikiMemoryPolicy, "memoryDir" | "getWikiDir">, options?: CreateDecisionEngineOptions): ScriptedDecisionEngine | PiDecisionEngine;
export { parsePositiveInteger };
