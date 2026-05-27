import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { open } from "akg-ts";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { AkgDurableMemoryPolicy } from "../src/memory.js";
import { createQueryTools } from "../src/tools.js";

import type { CompletedHandContext, DecisionContext } from "@agent-poker/pi-agent-shared";

let tmpDir: string;

beforeEach(async () => {
  tmpDir = await mkdtemp(join(tmpdir(), "akg-durable-test-"));
});

afterEach(async () => {
  await rm(tmpDir, { recursive: true, force: true });
});

function makeDecisionContext(memoryDir: string): DecisionContext {
  return {
    state: { session: { memoryDir } } as never,
    handNumber: 1,
    street: "preflop",
    board: [],
    pot: 3,
    toCall: 0,
    stacks: { "0": 199, "1": 198 },
    actionHistory: [],
    legalActions: [{ action: "check" }],
  };
}

function makeHandContext(handNumber: number, overrides: Partial<CompletedHandContext> = {}): CompletedHandContext {
  return {
    state: { session: { memoryDir: tmpDir, match: { blinds: { bb: 2 } } } } as never,
    handNumber,
    dealerSeat: 0,
    heroSeat: 0,
    seats: [
      { seat: 0, name: "hero" },
      { seat: 1, name: "villain" },
    ],
    heroHoleCards: ["As", "Kh"],
    board: ["Td", "9h", "2c"],
    actionHistory: [
      { seat: 0, action: "raise", amount: 4, street: "preflop" },
      { seat: 1, action: "call", amount: 2, street: "preflop" },
      { seat: 0, action: "bet", amount: 6, street: "flop" },
      { seat: 1, action: "fold", street: "flop" },
    ],
    showdownReached: false,
    showdown: undefined,
    result: [
      { seat: 0, chips_delta: 6 },
      { seat: 1, chips_delta: -6 },
    ],
    ...overrides,
  };
}

async function invokeTool(toolName: string, getStore: () => Promise<Awaited<ReturnType<typeof open>> | null>, params: unknown = {}) {
  const tool = createQueryTools(getStore).find((entry) => entry.name === toolName);
  if (!tool) throw new Error(`missing tool ${toolName}`);
  return tool.execute("call-1", params as never, new AbortController().signal, async () => {}, undefined as never);
}

describe("AkgDurableMemoryPolicy", () => {
  it("returns a reminder when memory is available and a fallback when it is not", async () => {
    const policy = new AkgDurableMemoryPolicy();
    await expect(policy.beforeDecision(makeDecisionContext(tmpDir))).resolves.toEqual({
      sections: ["AKG memory is available. Call akg_get_opponent to read the opponent index."],
    });
    await expect(policy.beforeDecision(makeDecisionContext(""))).resolves.toEqual({
      sections: ["AKG memory is not available (no memory_dir provided)."],
    });
  });

  it("writes opponent and hand memory, including hands with no earlier your_turn", async () => {
    const policy = new AkgDurableMemoryPolicy();
    await policy.afterHandEnd(makeHandContext(1));

    const store = await open(join(tmpDir, "memory.akg"));
    const opponent = store.getNode("opponent", "villain");
    expect(opponent).not.toBeNull();
    expect(opponent!.meta.hands_played).toBe(1);
    expect(opponent!.meta.vpip).toBe(1);
    expect(opponent!.meta.pfr).toBe(0);
    expect(opponent!.meta.cbet_folds).toBe(1);

    const hands = store.listNodesByTag("hand");
    expect(hands).toHaveLength(1);
    expect(hands[0].title).toBe("Hand 1");
    expect(hands[0].tags).toContain("villain_fold");
  });

  it("accumulates all opponent stat fields across multiple hands", async () => {
    const policy = new AkgDurableMemoryPolicy();
    await policy.afterHandEnd(makeHandContext(1, {
      actionHistory: [
        { seat: 0, action: "raise", amount: 4, street: "preflop" },
        { seat: 1, action: "raise", amount: 12, street: "preflop" },
        { seat: 0, action: "call", amount: 8, street: "preflop" },
        { seat: 1, action: "bet", amount: 10, street: "flop" },
        { seat: 0, action: "call", amount: 10, street: "flop" },
        { seat: 1, action: "bet", amount: 14, street: "turn" },
        { seat: 0, action: "call", amount: 14, street: "turn" },
        { seat: 1, action: "bet", amount: 18, street: "river" },
        { seat: 0, action: "call", amount: 18, street: "river" },
      ],
      board: ["Td", "9h", "2c", "5s", "Kc"],
      showdownReached: true,
      showdown: {
        "0": { hole_cards: ["As", "Kh"], rank: "pair" },
        "1": { hole_cards: ["Qc", "Qd"], rank: "two_pair" },
      },
      result: [
        { seat: 0, chips_delta: -40 },
        { seat: 1, chips_delta: 40 },
      ],
    }));
    await policy.afterHandEnd(makeHandContext(2));

    const store = await open(join(tmpDir, "memory.akg"));
    const opponent = store.getNode("opponent", "villain");
    expect(opponent!.meta.hands_played).toBe(2);
    expect(opponent!.meta.vpip).toBe(2);
    expect(opponent!.meta.pfr).toBe(1);
    expect(opponent!.meta.aggr_preflop).toBe(1);
    expect(opponent!.meta.aggr_flop).toBe(1);
    expect(opponent!.meta.aggr_turn).toBe(1);
    expect(opponent!.meta.aggr_river).toBe(1);
    expect(opponent!.meta.cbet_opportunities).toBe(1);
    expect(opponent!.meta.cbet_folds).toBe(1);
    expect(opponent!.meta.three_bet_count).toBe(1);
    expect(opponent!.meta.river_bet_count).toBe(1);
    expect(opponent!.meta.showdown_count).toBe(1);
    expect(opponent!.meta.showdown_win).toBe(1);
  });

  it("tags hands for showdown, big pots, folds, 3-bets, and aggressive action counts", async () => {
    const policy = new AkgDurableMemoryPolicy();
    await policy.afterHandEnd(makeHandContext(3, {
      dealerSeat: 1,
      board: ["Ad", "Ks", "Qc", "Jh", "Td"],
      actionHistory: [
        { seat: 0, action: "raise", amount: 6, street: "preflop" },
        { seat: 1, action: "raise", amount: 20, street: "preflop" },
        { seat: 0, action: "call", amount: 14, street: "preflop" },
        { seat: 1, action: "bet", amount: 24, street: "flop" },
        { seat: 0, action: "raise", amount: 72, street: "flop" },
        { seat: 1, action: "call", amount: 48, street: "flop" },
        { seat: 1, action: "bet", amount: 30, street: "turn" },
        { seat: 0, action: "call", amount: 30, street: "turn" },
      ],
      showdownReached: true,
      showdown: {
        "0": { hole_cards: ["As", "Kh"], rank: "straight" },
        "1": { hole_cards: ["Ac", "Kd"], rank: "straight" },
      },
      result: [
        { seat: 0, chips_delta: 0 },
        { seat: 1, chips_delta: 0 },
      ],
    }));

    const store = await open(join(tmpDir, "memory.akg"));
    const hand = store.listNodesByTag("hand")[0];
    expect(hand.tags).toEqual(expect.arrayContaining(["hand", "showdown", "big_pot", "3bet_hand", "aggressive_hand"]));
    expect(hand.meta.hero_position).toBe("bb");
  });

  it("creates pattern nodes and edges only at the threshold and updates them idempotently", async () => {
    const policy = new AkgDurableMemoryPolicy();
    await policy.afterHandEnd(makeHandContext(1));
    await policy.afterHandEnd(makeHandContext(2));

    let store = await open(join(tmpDir, "memory.akg"));
    expect(store.getNode("pattern", "folds-to-cbet")).toBeNull();

    await policy.afterHandEnd(makeHandContext(3));
    await policy.afterHandEnd(makeHandContext(3));
    store = await open(join(tmpDir, "memory.akg"));

    const pattern = store.getNode("pattern", "folds-to-cbet");
    expect(pattern).not.toBeNull();
    expect(pattern!.body).toContain("Villain has folded to hero flop c-bet 3 times across 3 c-bet opportunities.");
    expect(pattern!.meta.count).toBe(3);
    expect(pattern!.meta.opportunities).toBe(3);

    const supportEdges = store.outboundEdges({ type: "pattern", id: "folds-to-cbet" }, "supported_by");
    expect(supportEdges).toHaveLength(3);
    expect(supportEdges.map((edge) => edge.to.type)).toEqual(["hand", "hand", "hand"]);
    expect(supportEdges.map((edge) => edge.meta.hand_number).sort((left, right) => Number(left) - Number(right))).toEqual([1, 2, 3]);
    expect(supportEdges.every((edge) => edge.strength === 1 && edge.confidence === 1)).toBe(true);

    const showsPattern = store.outboundEdges({ type: "opponent", id: "villain" }, "shows_pattern");
    expect(showsPattern.find((edge) => edge.to.id === "folds-to-cbet")).toMatchObject({
      strength: 1,
      confidence: null,
      meta: { count: 3, opportunities: 3 },
    });
  });

  it("derives river and long-horizon patterns with the expected evidence counts", async () => {
    const policy = new AkgDurableMemoryPolicy();

    for (let handNumber = 1; handNumber <= 3; handNumber += 1) {
      await policy.afterHandEnd(makeHandContext(handNumber, {
        board: ["Td", "9h", "2c", "5s", "Kc"],
        actionHistory: [
          { seat: 0, action: "raise", amount: 4, street: "preflop" },
          { seat: 1, action: "raise", amount: 12, street: "preflop" },
          { seat: 0, action: "call", amount: 8, street: "preflop" },
          { seat: 0, action: "bet", amount: 12, street: "river" },
          { seat: 1, action: "fold", street: "river" },
        ],
      }));
    }

    for (let handNumber = 4; handNumber <= 15; handNumber += 1) {
      await policy.afterHandEnd(makeHandContext(handNumber, {
        actionHistory: [
          { seat: 0, action: "raise", amount: 4, street: "preflop" },
          { seat: 1, action: "call", amount: 2, street: "preflop" },
          { seat: 0, action: "bet", amount: 6, street: "flop" },
          { seat: 1, action: "call", amount: 6, street: "flop" },
        ],
        result: [
          { seat: 0, chips_delta: 0 },
          { seat: 1, chips_delta: 0 },
        ],
      }));
    }

    const store = await open(join(tmpDir, "memory.akg"));

    expect(store.getNode("pattern", "3bet-tendency")?.meta).toMatchObject({ count: 3, opportunities: 15 });
    expect(store.getNode("pattern", "folds-to-river-bet")?.meta).toMatchObject({ count: 3, opportunities: 3 });
    expect(store.getNode("pattern", "river-aggressor")).toBeNull();

    const callsWide = store.getNode("pattern", "calls-wide");
    expect(callsWide).not.toBeNull();
    expect(callsWide!.meta).toMatchObject({ count: 12, opportunities: 15 });
    expect(callsWide!.body).toContain("only 3 times across 15 completed hands");
  });

  it("supports the AKG query tools", async () => {
    const policy = new AkgDurableMemoryPolicy();
    await policy.beforeDecision(makeDecisionContext(tmpDir));

    let result = await invokeTool("akg_get_opponent", () => policy.getStore());
    expect(result.details).toMatchObject({ body: null });
    result = await invokeTool("akg_list_patterns", () => policy.getStore());
    expect(result.details).toEqual([]);
    result = await invokeTool("akg_get_pattern", () => policy.getStore(), { slug: "missing" });
    expect(result.details).toBeNull();
    result = await invokeTool("akg_get_hand", () => policy.getStore(), { hand_number: 99 });
    expect(result.details).toBeNull();

    await policy.afterHandEnd(makeHandContext(1));
    await policy.afterHandEnd(makeHandContext(2));
    await policy.afterHandEnd(makeHandContext(3));

    result = await invokeTool("akg_list_patterns", () => policy.getStore());
    expect(Array.isArray(result.details)).toBe(true);
    expect((result.details as Array<{ id: string }>).map((entry) => entry.id)).toContain("folds-to-cbet");

    result = await invokeTool("akg_list_hands", () => policy.getStore(), { tag: "villain_fold", limit: 2 });
    expect(result.details).toHaveLength(2);
    expect((result.details as Array<{ hand_number: number }>)[0].hand_number).toBe(3);

    result = await invokeTool("akg_get_hand", () => policy.getStore(), { hand_number: 2 });
    expect(result.details).toMatchObject({ title: "Hand 2" });

    result = await invokeTool("akg_get_pattern", () => policy.getStore(), { slug: "folds-to-cbet" });
    expect(result.details).toMatchObject({ hand_ids: expect.any(Array) });
  });
});
