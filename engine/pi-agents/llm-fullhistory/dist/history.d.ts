import type { CompletedHandContext, MemoryPolicy, PromptAugmentation } from "@agent-poker/pi-agent-shared";
import { formatCompletedHand } from "@agent-poker/pi-agent-shared";
export { formatCompletedHand };
export declare class FullHistoryMemoryPolicy implements MemoryPolicy {
    private readonly completedHands;
    private serverMemoryDir;
    get memoryDir(): string | undefined;
    beforeDecision(context: Parameters<MemoryPolicy["beforeDecision"]>[0]): Promise<PromptAugmentation>;
    afterHandEnd(context: CompletedHandContext): Promise<void>;
}
