import { formatCompletedHand } from "@agent-poker/pi-agent-shared";
export { formatCompletedHand };
const NO_PRIOR_HANDS_SECTION = "Prior hands: none yet.";
const PRIOR_HANDS_HEADER = "Prior hands:";
export class FullHistoryMemoryPolicy {
    completedHands = [];
    serverMemoryDir;
    get memoryDir() {
        return this.serverMemoryDir;
    }
    async beforeDecision(context) {
        this.serverMemoryDir = context.state.session?.memoryDir;
        if (this.completedHands.length === 0) {
            return { sections: [NO_PRIOR_HANDS_SECTION] };
        }
        return { sections: [PRIOR_HANDS_HEADER, ...this.completedHands] };
    }
    async afterHandEnd(context) {
        this.completedHands.push(formatCompletedHand(context));
    }
}
