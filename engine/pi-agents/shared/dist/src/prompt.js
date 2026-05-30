// Prompt construction shared across LLM poker agents.
export function buildDecisionPrompt(context, augmentation) {
    const hand = context.state.hand;
    const session = context.state.session;
    const sections = [
        "You are playing heads-up no-limit Texas Hold'em.",
        "Choose exactly one server-advertised legal action.",
        "Respond with JSON only, with shape {\"action\": string, \"amount\"?: number}.",
        "Do not include commentary, markdown, or extra keys.",
        "",
        `Session: ${session?.sessionId ?? "unknown"}`,
        `Match: ${session?.matchId ?? "unknown"}`,
        `Agent: ${session?.agentName ?? "unknown"}`,
        `Your seat: ${session?.yourSeat ?? "unknown"}`,
        `Seats: ${JSON.stringify(session?.seats ?? [])}`,
        `Hand: ${context.handNumber}`,
        `Street: ${context.street}`,
        `Hole cards: ${JSON.stringify(hand?.yourHoleCards ?? [])}`,
        `Board: ${JSON.stringify(context.board)}`,
        `Pot: ${context.pot}`,
        `To call: ${context.toCall}`,
        `Stacks: ${JSON.stringify(context.stacks)}`,
        `Current hand action history: ${JSON.stringify(context.actionHistory)}`,
        `Legal actions: ${JSON.stringify(context.legalActions)}`,
    ];
    return sections.concat(augmentation.sections).join("\n");
}
