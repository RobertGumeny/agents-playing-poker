import type { CompletedHandContext, DecisionContext, MemoryPolicy, PromptAugmentation } from "@agent-poker/pi-agent-shared";
export declare class MarkdownWikiMemoryPolicy implements MemoryPolicy {
    private serverMemoryDir;
    get memoryDir(): string | undefined;
    getWikiDir(): string | null;
    beforeDecision(context: DecisionContext): Promise<PromptAugmentation>;
    afterHandEnd(context: CompletedHandContext): Promise<void>;
}
