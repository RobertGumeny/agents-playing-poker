import { describe, expect, it } from "vitest";

import { formatCompletedHand, FullHistoryMemoryPolicy } from "../src/history.js";

describe("FullHistoryMemoryPolicy", () => {
  it("formats deterministic compact prior-hand lines and accumulates them in prompt order", async () => {
    const policy = new FullHistoryMemoryPolicy();
    const state = {
      session: {
        memoryDir: "/tmp/history",
      },
    };

    await expect(policy.beforeDecision({
      state,
      handNumber: 1,
      street: "preflop",
      board: [],
      pot: 3,
      toCall: 0,
      stacks: { "0": 199, "1": 198 },
      actionHistory: [],
      legalActions: [{ action: "check" }],
    } as never)).resolves.toEqual({ sections: ["Prior hands: none yet."] });
    expect(policy.memoryDir).toBe("/tmp/history");

    const handOne = {
      state,
      handNumber: 1,
      dealerSeat: 0,
      heroSeat: 0,
      seats: [
        { seat: 0, name: "llm-fullhistory" },
        { seat: 1, name: "heuristic" },
      ],
      heroHoleCards: ["As", "Kh"],
      board: ["Td", "9h", "2c", "5s", "Kc"],
      actionHistory: [
        { seat: 0, action: "call", amount: 1, street: "preflop" },
        { seat: 1, action: "check", street: "preflop" },
        { seat: 0, action: "bet", amount: 2, street: "flop" },
        { seat: 1, action: "fold", street: "flop" },
      ],
      showdownReached: false,
      showdown: {
        "0": { hole_cards: ["As", "Kh"], rank: "" },
      },
      result: [
        { seat: 0, chips_delta: 3 },
        { seat: 1, chips_delta: -3 },
      ],
    } as const;
    expect(formatCompletedHand(handOne as never)).toBe(
      "hand=1 | hero_pos=sb/button | hero_hole=As Kh | board=Td 9h 2c 5s Kc | actions=preflop:hero call 1, heuristic check; flop:hero bet 2, heuristic fold | showdown=no | revealed=hero As Kh | hero_result=+3",
    );

    await policy.afterHandEnd(handOne as never);
    await policy.afterHandEnd({
      ...handOne,
      handNumber: 2,
      dealerSeat: 1,
      heroHoleCards: ["Qc", "Qd"],
      board: ["7c", "4d", "2s", "Jh", "Tc"],
      actionHistory: [
        { seat: 1, action: "call", amount: 1, street: "preflop" },
        { seat: 0, action: "check", street: "preflop" },
        { seat: 1, action: "bet", amount: 4, street: "river" },
        { seat: 0, action: "call", amount: 4, street: "river" },
      ],
      showdownReached: true,
      showdown: {
        "0": { hole_cards: ["Qc", "Qd"], rank: "one pair, queens" },
        "1": { hole_cards: ["Ac", "Jc"], rank: "one pair, jacks" },
      },
      result: [
        { seat: 0, chips_delta: 12 },
        { seat: 1, chips_delta: -12 },
      ],
    } as never);

    await expect(policy.beforeDecision({
      state,
      handNumber: 3,
      street: "flop",
      board: ["Ah", "7d", "3c"],
      pot: 6,
      toCall: 2,
      stacks: { "0": 214, "1": 186 },
      actionHistory: [],
      legalActions: [{ action: "call", amount: 2 }],
    } as never)).resolves.toEqual({
      sections: [
        "Prior hands:",
        "hand=1 | hero_pos=sb/button | hero_hole=As Kh | board=Td 9h 2c 5s Kc | actions=preflop:hero call 1, heuristic check; flop:hero bet 2, heuristic fold | showdown=no | revealed=hero As Kh | hero_result=+3",
        "hand=2 | hero_pos=bb | hero_hole=Qc Qd | board=7c 4d 2s Jh Tc | actions=preflop:heuristic call 1, hero check; river:heuristic bet 4, hero call 4 | showdown=yes | revealed=hero Qc Qd (one pair, queens); heuristic Ac Jc (one pair, jacks) | hero_result=+12",
      ],
    });
  });
});
