import { formatCompletedHand } from "@agent-poker/pi-agent-shared";
import { readNotes, runNotesUpdate } from "./update.js";
const NO_NOTES_SECTION = "Opponent notes (your running memory): none yet.";
const NOTES_HEADER = "Opponent notes (your running memory). This is the only thing you remember about this opponent between hands:";
export class MarkdownSingleMemoryPolicy {
    serverMemoryDir;
    get memoryDir() {
        return this.serverMemoryDir;
    }
    async beforeDecision(context) {
        this.serverMemoryDir = context.state.session?.memoryDir;
        const notes = await readNotes(this.serverMemoryDir);
        if (notes.trim().length === 0) {
            return { sections: [NO_NOTES_SECTION] };
        }
        return { sections: [NOTES_HEADER, notes.trim()] };
    }
    async afterHandEnd(context) {
        const memoryDir = context.state.session?.memoryDir ?? this.serverMemoryDir;
        this.serverMemoryDir = memoryDir;
        if (!memoryDir)
            return;
        await runNotesUpdate({
            memoryDir,
            handNumber: context.handNumber,
            handSummary: formatCompletedHand(context),
            model: process.env.PI_POKER_MODEL,
            thinkingLevel: process.env.PI_POKER_THINKING_LEVEL,
        });
    }
}
