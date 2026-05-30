import { type AgentSessionEvent, ModelRegistry, type CreateAgentSessionOptions } from "@earendil-works/pi-coding-agent";
import type { ActionPayload } from "./protocol.js";
import type { CompletedHandContext, DecisionEngine, DecisionRequest } from "./strategy.js";
type PiThinkingLevel = NonNullable<CreateAgentSessionOptions["thinkingLevel"]>;
export type PiSessionScope = "decision" | "hand";
type PiSession = {
    prompt(text: string): Promise<void>;
    subscribe(listener: (event: AgentSessionEvent) => void): () => void;
    getLastAssistantText(): string | undefined;
    exportToJsonl(outputPath?: string): string;
    dispose(): void;
};
type PiSessionFactory = (options: ResolvedPiSessionOptions) => Promise<PiSession>;
interface ResolvedPiSessionOptions {
    cwd: string;
    agentDir: string;
    sessionDir?: string;
    model?: string;
    thinkingLevel?: PiThinkingLevel;
}
export interface PiDecisionEngineOptions {
    cwd: string;
    agentDir?: string;
    sessionDir?: string;
    sessionDirProvider?: () => string | undefined;
    model?: string;
    thinkingLevel?: PiThinkingLevel;
    sessionScope?: PiSessionScope;
    sessionFactory?: PiSessionFactory;
}
export interface ScriptedDecisionEngineOptions {
    decisions: ActionPayload[];
    sessionDirProvider?: () => string | undefined;
    sessionScope?: PiSessionScope;
}
export declare class PiDecisionEngine implements DecisionEngine {
    private readonly options;
    private readonly sessionFactory;
    private readonly sessionScope;
    private exportCount;
    private activeHandSession;
    constructor(options: PiDecisionEngineOptions);
    decide(request: DecisionRequest): Promise<ActionPayload>;
    onHandEnd(context: CompletedHandContext): Promise<void>;
    onSessionEnd(): Promise<void>;
    private ensureHandSession;
    private createSession;
    private promptSession;
    private persistAndDispose;
}
export declare class ScriptedDecisionEngine implements DecisionEngine {
    private readonly options;
    private index;
    private sessionCount;
    private activeHandNumber;
    constructor(options: ScriptedDecisionEngineOptions);
    decide(request: DecisionRequest): Promise<ActionPayload>;
    onHandStart(context: {
        handNumber: number;
    }): void;
    onHandEnd(context: CompletedHandContext): void;
    private writeObservabilityLog;
}
export declare function resolveModel(spec: string | undefined, modelRegistry: ModelRegistry): CreateAgentSessionOptions["model"];
export declare function parsePiThinkingLevel(value: string | undefined): PiThinkingLevel | undefined;
export declare function parsePositiveInteger(value: string | undefined): number | undefined;
export declare function parseFakeDecisions(value: string | undefined): ActionPayload[] | undefined;
export interface CreateStandardDecisionEngineOptions {
    sessionScope: PiSessionScope;
    memoryDirProvider: () => string | undefined;
}
export declare function createStandardDecisionEngine(options: CreateStandardDecisionEngineOptions): DecisionEngine;
export {};
