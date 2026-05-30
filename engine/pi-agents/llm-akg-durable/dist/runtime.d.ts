import { type AgentSessionEvent, AuthStorage, createAgentSession, DefaultResourceLoader, ModelRegistry, SessionManager, SettingsManager, type CreateAgentSessionOptions } from "@earendil-works/pi-coding-agent";
import { PiDecisionEngine, ScriptedDecisionEngine, parsePositiveInteger } from "@agent-poker/pi-agent-shared";
import type { AkgDurableMemoryPolicy } from "./memory.js";
export declare const DURABLE_SYSTEM_PROMPT: string;
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
type SessionFactoryDeps = {
    createAuthStorage: () => ReturnType<typeof AuthStorage.create>;
    createModelRegistry: (authStorage: ReturnType<typeof AuthStorage.create>) => ReturnType<typeof ModelRegistry.create>;
    createSettingsManager: (cwd: string, agentDir: string) => ReturnType<typeof SettingsManager.create>;
    createResourceLoader: (options: ConstructorParameters<typeof DefaultResourceLoader>[0]) => DefaultResourceLoader;
    createSessionManager: (cwd: string) => ReturnType<typeof SessionManager.inMemory>;
    createAgentSession: typeof createAgentSession;
};
export interface CreateDecisionEngineOptions {
    cwd?: string;
    sessionDir?: string;
    model?: string;
    thinkingLevel?: string;
    fakeDecisionsJSON?: string;
    sessionFactory?: PiSessionFactory;
}
export declare function createDurableSessionFactory(memoryPolicy: Pick<AkgDurableMemoryPolicy, "getStore">, deps?: SessionFactoryDeps): PiSessionFactory;
export declare function createDecisionEngine(memoryPolicy: Pick<AkgDurableMemoryPolicy, "memoryDir" | "getStore">, options?: CreateDecisionEngineOptions): ScriptedDecisionEngine | PiDecisionEngine;
export declare function resolveAgentDir(): string;
export { parsePositiveInteger };
