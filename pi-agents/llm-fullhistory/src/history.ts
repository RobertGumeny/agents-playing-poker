import type { CompletedHandContext, MemoryPolicy, PromptAugmentation } from "@agent-poker/pi-agent-shared";

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

export function formatCompletedHand(context: CompletedHandContext): string {
  const heroResult = context.result.find((entry) => entry.seat === context.heroSeat)?.chips_delta ?? 0;
  const showdown = formatShowdown(context);
  return [
    `hand=${context.handNumber}`,
    `hero_pos=${heroPositionLabel(context)}`,
    `hero_hole=${formatCards(context.heroHoleCards)}`,
    `board=${formatCards(context.board)}`,
    `actions=${formatActionSummary(context)}`,
    `showdown=${context.showdownReached ? "yes" : "no"}`,
    `revealed=${showdown}`,
    `hero_result=${formatChipDelta(heroResult)}`,
  ].join(" | ");
}

function heroPositionLabel(context: CompletedHandContext): string {
  return context.dealerSeat === context.heroSeat ? "sb/button" : "bb";
}

function formatCards(cards: string[]): string {
  return cards.length === 0 ? "-" : cards.join(" ");
}

function formatActionSummary(context: CompletedHandContext): string {
  if (context.actionHistory.length === 0) return "none";

  const streets = new Map<string, string[]>();
  for (const action of context.actionHistory) {
    const actor = action.seat === context.heroSeat ? "hero" : seatNameForSummary(context, action.seat);
    const summary = action.amount === undefined ? `${actor} ${action.action}` : `${actor} ${action.action} ${action.amount}`;
    const bucket = streets.get(action.street) ?? [];
    bucket.push(summary);
    streets.set(action.street, bucket);
  }

  return [...streets.entries()].map(([street, actions]) => `${street}:${actions.join(", ")}`).join("; ");
}

function seatNameForSummary(context: CompletedHandContext, seat: number): string {
  const descriptor = context.seats.find((entry) => entry.seat === seat);
  if (!descriptor) return `seat${seat}`;
  return descriptor.name === "hero" ? "hero" : descriptor.name;
}

function formatShowdown(context: CompletedHandContext): string {
  if (!context.showdown || Object.keys(context.showdown).length === 0) {
    return "none";
  }

  return Object.entries(context.showdown)
    .map(([seat, entry]) => {
      const seatNumber = Number.parseInt(seat, 10);
      const actor = seatNumber === context.heroSeat ? "hero" : seatNameForSummary(context, seatNumber);
      const rank = entry.rank ? ` (${entry.rank})` : "";
      return `${actor} ${formatCards(entry.hole_cards)}${rank}`;
    })
    .join("; ");
}

function formatChipDelta(value: number): string {
  return value >= 0 ? `+${value}` : `${value}`;
}
