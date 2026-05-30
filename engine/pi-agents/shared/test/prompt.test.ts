import { describe, expect, it } from "vitest";

import { buildDecisionPrompt } from "../src/prompt.js";
import { createAgentState } from "../src/state.js";

describe("buildDecisionPrompt", () => {
  function buildBaseState() {
    const state = createAgentState();
    state.session = {
      sessionId: "ses-1",
      matchId: "mat-1",
      match: {
        match_id: "mat-1",
        seed: 1,
        hand_count: 200,
        variant: "heads-up-nlhe",
        info_realism: "showdown-only",
        starting_stack: 200,
        blinds: { sb: 1, bb: 2 },
        decision_deadline_ms: 30000,
      },
      agentName: "llm-stateless",
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
    return state;
  }

  it("includes only current decision state in the base prompt", () => {
    const prompt = buildDecisionPrompt(
      {
        state: buildBaseState(),
        handNumber: 7,
        street: "flop",
        board: ["Td", "9h", "2c"],
        pot: 6,
        toCall: 2,
        stacks: { "0": 197, "1": 197 },
        actionHistory: [{ seat: 0, action: "check", street: "preflop" }],
        legalActions: [{ action: "call", amount: 2 }],
      },
      { sections: [] },
    );

    expect(prompt).toContain("Session: ses-1");
    expect(prompt).toContain("Match: mat-1");
    expect(prompt).toContain("Agent: llm-stateless");
    expect(prompt).toContain("Your seat: 0");
    expect(prompt).toContain("Hand: 7");
    expect(prompt).toContain('Hole cards: ["As","Kh"]');
    expect(prompt).toContain("Current hand action history:");
    expect(prompt).not.toContain("Previous hand result");
    expect(prompt).not.toContain("chips_delta");
  });

  it("includes memory/history sections only when a strategy explicitly adds them", () => {
    const prompt = buildDecisionPrompt(
      {
        state: buildBaseState(),
        handNumber: 7,
        street: "flop",
        board: ["Td", "9h", "2c"],
        pot: 6,
        toCall: 2,
        stacks: { "0": 197, "1": 197 },
        actionHistory: [{ seat: 0, action: "check", street: "preflop" }],
        legalActions: [{ action: "call", amount: 2 }],
      },
      { sections: ["Previous hand result: won 14 chips."] },
    );

    expect(prompt).toContain("Previous hand result: won 14 chips.");
  });
});
