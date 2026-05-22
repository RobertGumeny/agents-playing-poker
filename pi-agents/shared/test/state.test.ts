import { describe, expect, it } from "vitest";

import { applyHandStart, applySessionInit, createAgentState, resetHandState } from "../src/state.js";

describe("state helpers", () => {
  it("tracks session and current hand state", () => {
    const state = createAgentState();

    applySessionInit(state, {
      session_id: "ses-1",
      agent_name: "llm-nomemory",
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
      seats: [
        { seat: 0, name: "hero" },
        { seat: 1, name: "villain" },
      ],
      your_seat: 0,
      memory_dir: "/tmp/memory",
    });
    applyHandStart(state, {
      hand_number: 7,
      dealer_seat: 1,
      stacks: { "0": 200, "1": 200 },
      blinds_posted: [
        { seat: 0, amount: 1 },
        { seat: 1, amount: 2 },
      ],
      your_hole_cards: ["As", "Kh"],
    });

    expect(state.session?.matchId).toBe("mat-1");
    expect(state.hand?.yourHoleCards).toEqual(["As", "Kh"]);

    resetHandState(state);

    expect(state.hand).toBeUndefined();
  });
});
