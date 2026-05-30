import type { CompletedHandContext, DecisionContext, MemoryPolicy, PromptAugmentation } from "@agent-poker/pi-agent-shared";
export declare class AkgMemoryPolicy implements MemoryPolicy {
    private store;
    private storePath;
    private serverMemoryDir;
    get memoryDir(): string | undefined;
    beforeDecision(context: DecisionContext): Promise<PromptAugmentation>;
    afterHandEnd(context: CompletedHandContext): Promise<void>;
    private getStore;
}
