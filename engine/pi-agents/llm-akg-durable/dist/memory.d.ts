import { type Store } from "akg-ts";
import type { CompletedHandContext, DecisionContext, MemoryPolicy, PromptAugmentation } from "@agent-poker/pi-agent-shared";
export declare class AkgDurableMemoryPolicy implements MemoryPolicy {
    private store;
    private storeMemoryDir;
    private serverMemoryDir;
    get memoryDir(): string | undefined;
    beforeDecision(context: DecisionContext): Promise<PromptAugmentation>;
    afterHandEnd(context: CompletedHandContext): Promise<void>;
    getStore(memoryDir?: string | undefined): Promise<Store | null>;
}
