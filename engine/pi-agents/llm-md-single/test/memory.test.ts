import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";

import { describe, expect, it } from "vitest";

import { MarkdownSingleMemoryPolicy } from "../src/memory.js";
import { buildUpdatePrompt, NOTES_FILENAME, runNotesUpdate } from "../src/update.js";

function decisionContext(memoryDir: string | undefined) {
  return {
    state: { session: memoryDir ? { memoryDir } : undefined },
    handNumber: 1,
    street: "preflop",
    board: [],
    pot: 3,
    toCall: 0,
    stacks: { "0": 199, "1": 198 },
    actionHistory: [],
    legalActions: [{ action: "check" }],
  } as never;
}

const handOne = {
  state: { session: undefined as { memoryDir: string } | undefined },
  handNumber: 1,
  dealerSeat: 0,
  heroSeat: 0,
  seats: [
    { seat: 0, name: "llm-md-single" },
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
  result: [
    { seat: 0, chips_delta: 3 },
    { seat: 1, chips_delta: -3 },
  ],
} as const;

describe("MarkdownSingleMemoryPolicy.beforeDecision", () => {
  it("injects a no-notes section when the file is missing", async () => {
    const policy = new MarkdownSingleMemoryPolicy();
    const dir = await mkdtemp(path.join(tmpdir(), "md-single-empty-"));
    await expect(policy.beforeDecision(decisionContext(dir))).resolves.toEqual({
      sections: ["Opponent notes (your running memory): none yet."],
    });
    expect(policy.memoryDir).toBe(dir);
  });

  it("injects the whole notes file as one section when present", async () => {
    const policy = new MarkdownSingleMemoryPolicy();
    const dir = await mkdtemp(path.join(tmpdir(), "md-single-notes-"));
    await writeFile(path.join(dir, NOTES_FILENAME), "# Villain\n\nfolds to flop c-bet 4/7\n", "utf8");

    const result = await policy.beforeDecision(decisionContext(dir));
    expect(result.sections).toHaveLength(2);
    expect(result.sections[0]).toContain("Opponent notes");
    expect(result.sections[1]).toContain("folds to flop c-bet 4/7");
  });

  it("returns no-notes when no memory dir is provided", async () => {
    const policy = new MarkdownSingleMemoryPolicy();
    await expect(policy.beforeDecision(decisionContext(undefined))).resolves.toEqual({
      sections: ["Opponent notes (your running memory): none yet."],
    });
  });
});

describe("buildUpdatePrompt", () => {
  it("includes current notes, the new hand summary, and a full-rewrite instruction", () => {
    const prompt = buildUpdatePrompt("folds to flop c-bet 4/7", "hand=8 | hero_pos=bb | hero_result=+5");
    expect(prompt).toContain("folds to flop c-bet 4/7");
    expect(prompt).toContain("hand=8 | hero_pos=bb | hero_result=+5");
    expect(prompt).toContain(`Return the complete updated ${NOTES_FILENAME}.`);
  });

  it("marks an empty current file explicitly", () => {
    const prompt = buildUpdatePrompt("", "hand=1 | hero_pos=sb/button");
    expect(prompt).toContain("(empty — no notes yet)");
  });
});

describe("runNotesUpdate scripted (fake) mode", () => {
  it("appends the hand and writes a separate update transcript without a live model", async () => {
    const dir = await mkdtemp(path.join(tmpdir(), "md-single-update-"));
    const previousFake = process.env.PI_POKER_FAKE_DECISIONS_JSON;
    process.env.PI_POKER_FAKE_DECISIONS_JSON = JSON.stringify([{ action: "check" }]);
    try {
      await runNotesUpdate({
        memoryDir: dir,
        handNumber: 1,
        handSummary: "hand=1 | hero_pos=sb/button | hero_hole=As Kh",
      });
    } finally {
      if (previousFake === undefined) delete process.env.PI_POKER_FAKE_DECISIONS_JSON;
      else process.env.PI_POKER_FAKE_DECISIONS_JSON = previousFake;
    }

    const notes = await readFile(path.join(dir, NOTES_FILENAME), "utf8");
    expect(notes).toContain("## Hand 1");
    expect(notes).toContain("hand=1 | hero_pos=sb/button | hero_hole=As Kh");

    const updateLog = await readFile(path.join(dir, "session-updates.jsonl"), "utf8");
    const entries = updateLog.trim().split("\n").map((line) => JSON.parse(line) as Record<string, unknown>);
    expect(entries).toHaveLength(1);
    expect(entries[0]).toMatchObject({ type: "fake_update_session", hand_number: 1, notes_written: true });
  });
});

describe("MarkdownSingleMemoryPolicy.afterHandEnd", () => {
  it("is a no-op when no memory dir is available", async () => {
    const policy = new MarkdownSingleMemoryPolicy();
    await expect(policy.afterHandEnd({ ...handOne } as never)).resolves.toBeUndefined();
  });
});
