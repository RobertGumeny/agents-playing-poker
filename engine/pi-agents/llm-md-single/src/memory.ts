import type { CompletedHandContext, DecisionContext, MemoryPolicy, PromptAugmentation } from "@agent-poker/pi-agent-shared";
import { formatCompletedHand } from "@agent-poker/pi-agent-shared";

import { readNotes, runNotesUpdate } from "./update.js";

const NO_NOTES_SECTION = "Opponent notes (your running memory): none yet.";
const NOTES_HEADER = "Opponent notes (your running memory). This is the only thing you remember about this opponent between hands:";

export class MarkdownSingleMemoryPolicy implements MemoryPolicy {
  private serverMemoryDir: string | undefined;

  get memoryDir(): string | undefined {
    return this.serverMemoryDir;
  }

  async beforeDecision(context: DecisionContext): Promise<PromptAugmentation> {
    this.serverMemoryDir = context.state.session?.memoryDir;
    const notes = await readNotes(this.serverMemoryDir);
    if (notes.trim().length === 0) {
      return { sections: [NO_NOTES_SECTION] };
    }
    return { sections: [NOTES_HEADER, notes.trim()] };
  }

  async afterHandEnd(context: CompletedHandContext): Promise<void> {
    const memoryDir = context.state.session?.memoryDir ?? this.serverMemoryDir;
    this.serverMemoryDir = memoryDir;
    if (!memoryDir) return;

    await runNotesUpdate({
      memoryDir,
      handNumber: context.handNumber,
      handSummary: formatCompletedHand(context),
      model: process.env.PI_POKER_MODEL,
      thinkingLevel: process.env.PI_POKER_THINKING_LEVEL,
    });
  }
}
