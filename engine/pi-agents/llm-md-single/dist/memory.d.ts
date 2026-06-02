import type { CompletedHandContext, DecisionContext, MemoryPolicy, PromptAugmentation } from "@agent-poker/pi-agent-shared";
export declare class MarkdownSingleMemoryPolicy implements MemoryPolicy {
    private serverMemoryDir;
    get memoryDir(): string | undefined;
    beforeDecision(context: DecisionContext): Promise<PromptAugmentation>;
    afterHandEnd(context: CompletedHandContext): Promise<void>;
}
