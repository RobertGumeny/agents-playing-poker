import type { CompletedHandContext, MemoryPolicy, PromptAugmentation } from "@agent-poker/pi-agent-shared";
export declare class FullHistoryMemoryPolicy implements MemoryPolicy {
    private readonly completedHands;
    private serverMemoryDir;
    get memoryDir(): string | undefined;
    beforeDecision(context: Parameters<MemoryPolicy["beforeDecision"]>[0]): Promise<PromptAugmentation>;
    afterHandEnd(context: CompletedHandContext): Promise<void>;
}
export declare function formatCompletedHand(context: CompletedHandContext): string;
