// Prompt construction shared across LLM poker agents.

import type { DecisionContext, PromptAugmentation } from "./strategy";

export function buildDecisionPrompt(context: DecisionContext, augmentation: PromptAugmentation): string {
  const hand = context.state.hand;
  const session = context.state.session;

  return [
    "You are playing heads-up no-limit Texas Hold'em.",
    "Choose exactly one legal action. Respond with JSON only, with shape {\"action\": string, \"amount\"?: number}.",
    "Do not include commentary, markdown, or extra keys.",
    "",
    `Agent seat: ${session?.yourSeat ?? "unknown"}`,
    `Hand: ${context.handNumber}`,
    `Street: ${context.street}`,
    `Hole cards: ${JSON.stringify(hand?.yourHoleCards ?? [])}`,
    `Board: ${JSON.stringify(context.board)}`,
    `Pot: ${context.pot}`,
    `To call: ${context.toCall}`,
    `Stacks: ${JSON.stringify(context.stacks)}`,
    `Action history: ${JSON.stringify(context.actionHistory)}`,
    `Legal actions: ${JSON.stringify(context.legalActions)}`,
    ...augmentation.sections,
  ].join("\n");
}
