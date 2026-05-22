import { describe, expect, it } from "vitest";

import { applyHandStart, applySessionInit, applyYourTurn, createAgentState, resetHandState, resetSessionState } from "../src/state.js";

describe("state helpers", () => {
  it("tracks session, hand start, and current turn state", () => {
    const state = createAgentState();

    applySessionInit(state, {
      session_id: "ses-1",
      agent_name: "llm-stateless",
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
    applyYourTurn(state, {
      hand_number: 7,
      street: "flop",
      board: ["Td", "9h", "2c"],
      pot: 6,
      to_call: 2,
      stacks: { "0": 197, "1": 197 },
      seats: [
        { seat: 0, name: "hero" },
        { seat: 1, name: "villain" },
      ],
      action_history: [{ seat: 1, action: "call", amount: 1, street: "preflop" }],
      legal_actions: [
        { action: "fold" },
        { action: "call", amount: 2 },
      ],
    });

    expect(state.session?.matchId).toBe("mat-1");
    expect(state.hand?.yourHoleCards).toEqual(["As", "Kh"]);
    expect(state.hand?.currentTurn).toMatchObject({
      street: "flop",
      board: ["Td", "9h", "2c"],
      pot: 6,
      toCall: 2,
    });

    resetHandState(state);

    expect(state.hand).toBeUndefined();
  });

  it("clears stale hand state on session reset", () => {
    const state = createAgentState();

    applySessionInit(state, {
      session_id: "ses-1",
      agent_name: "llm-stateless",
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
      seats: [{ seat: 0, name: "hero" }],
      your_seat: 0,
      memory_dir: "/tmp/memory",
    });
    applyHandStart(state, {
      hand_number: 9,
      dealer_seat: 0,
      stacks: { "0": 200 },
      blinds_posted: [{ seat: 0, amount: 1 }],
      your_hole_cards: ["Ac", "Ad"],
    });

    resetSessionState(state);

    expect(state.session).toBeUndefined();
    expect(state.hand).toBeUndefined();
  });
});
