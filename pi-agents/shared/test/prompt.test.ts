import { describe, expect, it } from "vitest";

import { buildDecisionPrompt } from "../src/prompt.js";
import { createAgentState } from "../src/state.js";

describe("buildDecisionPrompt", () => {
  it("includes current decision state and augmentation sections", () => {
    const state = createAgentState();
    state.session = {
      sessionId: "ses-1",
      matchId: "mat-1",
      agentName: "llm-nomemory",
      yourSeat: 0,
      seats: [
        { seat: 0, name: "hero" },
        { seat: 1, name: "villain" },
      ],
      memoryDir: "/tmp/memory",
    };
    state.hand = {
      handNumber: 7,
      dealerSeat: 1,
      stacks: { "0": 200, "1": 200 },
      blindsPosted: [
        { seat: 0, amount: 1 },
        { seat: 1, amount: 2 },
      ],
      yourHoleCards: ["As", "Kh"],
    };

    const prompt = buildDecisionPrompt(
      {
        state,
        handNumber: 7,
        street: "flop",
        board: ["Td", "9h", "2c"],
        pot: 6,
        toCall: 2,
        stacks: { "0": 197, "1": 197 },
        actionHistory: [{ seat: 0, action: "check", street: "preflop" }],
        legalActions: [{ action: "call", amount: 2 }],
      },
      { sections: ["Memory: none"] },
    );

    expect(prompt).toContain("Hand: 7");
    expect(prompt).toContain('Hole cards: ["As","Kh"]');
    expect(prompt).toContain("Memory: none");
  });
});
