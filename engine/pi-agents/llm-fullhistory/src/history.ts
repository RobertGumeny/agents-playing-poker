import type { CompletedHandContext, MemoryPolicy, PromptAugmentation } from "@agent-poker/pi-agent-shared";
import { formatCompletedHand } from "@agent-poker/pi-agent-shared";

export { formatCompletedHand };

const NO_PRIOR_HANDS_SECTION = "Prior hands: none yet.";
const PRIOR_HANDS_HEADER = "Prior hands:";

export class FullHistoryMemoryPolicy implements MemoryPolicy {
  private readonly completedHands: string[] = [];
  private serverMemoryDir: string | undefined;

  get memoryDir(): string | undefined {
    return this.serverMemoryDir;
  }

  async beforeDecision(context: Parameters<MemoryPolicy["beforeDecision"]>[0]): Promise<PromptAugmentation> {
    this.serverMemoryDir = context.state.session?.memoryDir;
    if (this.completedHands.length === 0) {
      return { sections: [NO_PRIOR_HANDS_SECTION] };
    }
    return { sections: [PRIOR_HANDS_HEADER, ...this.completedHands] };
  }

  async afterHandEnd(context: CompletedHandContext): Promise<void> {
    this.completedHands.push(formatCompletedHand(context));
  }
}
