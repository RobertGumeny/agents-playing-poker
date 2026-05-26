import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { open } from "akg-ts";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { AkgMemoryPolicy } from "../src/memory.js";

import type { CompletedHandContext, DecisionContext } from "@agent-poker/pi-agent-shared";

let tmpDir: string;

beforeEach(async () => {
  tmpDir = await mkdtemp(join(tmpdir(), "akg-memory-test-"));
});

afterEach(async () => {
  await rm(tmpDir, { recursive: true, force: true });
});

function makeDecisionContext(memoryDir: string, handNumber = 1): DecisionContext {
  return {
    state: { session: { memoryDir } } as never,
    handNumber,
    street: "preflop",
    board: [],
    pot: 3,
    toCall: 0,
    stacks: { "0": 199, "1": 198 },
    actionHistory: [],
    legalActions: [{ action: "check" }],
  };
}

function makeHandContext(handNumber: number, heroSeat = 0, dealerSeat = 0): CompletedHandContext {
  return {
    state: {} as never,
    handNumber,
    dealerSeat,
    heroSeat,
    seats: [
      { seat: 0, name: "hero" },
      { seat: 1, name: "heuristic" },
    ],
    heroHoleCards: ["As", "Kh"],
    board: ["Td", "9h", "2c"],
    actionHistory: [
      { seat: 0, action: "bet", amount: 20, street: "flop" },
      { seat: 1, action: "fold", street: "flop" },
    ],
    showdownReached: false,
    showdown: undefined,
    result: [
      { seat: 0, chips_delta: 3 },
      { seat: 1, chips_delta: -3 },
    ],
  };
}

describe("AkgMemoryPolicy", () => {
  it("returns no-data sections when memory_dir is absent", async () => {
    const policy = new AkgMemoryPolicy();
    const result = await policy.beforeDecision(makeDecisionContext(""));
    expect(result.sections[0]).toContain("not available");
  });

  it("returns no-data sections before any hands are played", async () => {
    const policy = new AkgMemoryPolicy();
    const result = await policy.beforeDecision(makeDecisionContext(tmpDir));
    expect(result.sections.join("\n")).toContain("no data yet");
    expect(result.sections.join("\n")).toContain("none yet");
  });

  it("writes a hand node and opponent node after afterHandEnd", async () => {
    const policy = new AkgMemoryPolicy();
    await policy.beforeDecision(makeDecisionContext(tmpDir));
    await policy.afterHandEnd(makeHandContext(1));

    const store = await open(join(tmpDir, "memory.akg"));
    const opponent = store.getNode("opponent", "villain");
    expect(opponent).not.toBeNull();
    expect(opponent!.meta.hands_played).toBe(1);
    expect(opponent!.meta.fold_to_bet).toBe(1);

    const hands = store.listNodesByTag("hand");
    expect(hands).toHaveLength(1);
    expect(hands[0].title).toBe("Hand 1");
    expect(hands[0].body).toContain("hero sb");
    expect(hands[0].body).toContain("Net: +3");
  });

  it("accumulates opponent stats across multiple hands", async () => {
    const policy = new AkgMemoryPolicy();
    await policy.beforeDecision(makeDecisionContext(tmpDir));
    await policy.afterHandEnd(makeHandContext(1));
    await policy.afterHandEnd(makeHandContext(2));
    await policy.afterHandEnd(makeHandContext(3));

    const store = await open(join(tmpDir, "memory.akg"));
    const opponent = store.getNode("opponent", "villain");
    expect(opponent!.meta.hands_played).toBe(3);
    expect(opponent!.meta.fold_to_bet).toBe(3);
  });

  it("injects opponent profile and recent hands into beforeDecision after hands are played", async () => {
    const policy = new AkgMemoryPolicy();
    await policy.beforeDecision(makeDecisionContext(tmpDir, 1));
    await policy.afterHandEnd(makeHandContext(1));

    const result = await policy.beforeDecision(makeDecisionContext(tmpDir, 2));
    const joined = result.sections.join("\n");
    expect(joined).toContain("Opponent profile:");
    expect(joined).toContain("1 hands played");
    expect(joined).toContain("Hand 1");
  });

  it("limits recent hands to 5 in the prompt", async () => {
    const policy = new AkgMemoryPolicy();
    await policy.beforeDecision(makeDecisionContext(tmpDir, 1));
    for (let i = 1; i <= 8; i++) {
      await policy.afterHandEnd(makeHandContext(i));
    }

    const result = await policy.beforeDecision(makeDecisionContext(tmpDir, 9));
    const handMentions = result.sections.filter((s) => s.startsWith("Hand "));
    expect(handMentions).toHaveLength(5);
    expect(handMentions[0]).toContain("Hand 4");
    expect(handMentions[4]).toContain("Hand 8");
  });

  it("exposes memoryDir from session state", async () => {
    const policy = new AkgMemoryPolicy();
    expect(policy.memoryDir).toBeUndefined();
    await policy.beforeDecision(makeDecisionContext(tmpDir));
    expect(policy.memoryDir).toBe(tmpDir);
  });
});
