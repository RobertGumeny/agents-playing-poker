import { mkdtemp, readFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";

import { describe, expect, it } from "vitest";

import { MarkdownWikiMemoryPolicy } from "../src/memory.js";
import { extractWikiLinks, listPages, readPage, ROOT_PAGE, wikiDir, writePage } from "../src/pages.js";
import { createReadTools, createWriteTool } from "../src/tools.js";
import { buildWikiUpdatePrompt, runWikiUpdate } from "../src/update.js";

async function callTool(tool: { execute: (...args: never[]) => unknown }, params: unknown) {
  return (await (tool.execute as (...args: unknown[]) => Promise<{ details: unknown }>)(
    "call-1",
    params,
    new AbortController().signal,
    async () => {},
    undefined,
  )).details;
}

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

describe("page helpers", () => {
  it("lists, reads, and writes pages and rejects traversal", async () => {
    const dir = path.join(await mkdtemp(path.join(tmpdir(), "wiki-pages-")), "wiki");
    await writePage(dir, "villain", "# villain\n\n[[patterns/folds-to-cbet]]");
    await writePage(dir, "patterns/folds-to-cbet", "folds to flop c-bet 4/7");

    expect(await listPages(dir)).toEqual(["patterns/folds-to-cbet", "villain"]);

    const root = await readPage(dir, "villain");
    expect(root.found).toBe(true);
    expect(root.content).toContain("[[patterns/folds-to-cbet]]");

    const missing = await readPage(dir, "patterns/nope");
    expect(missing).toEqual({ found: false, page: "patterns/nope" });

    expect(await writePage(dir, "../escape", "nope")).toBeNull();
  });

  it("extracts wiki links", () => {
    expect(extractWikiLinks("see [[patterns/folds-to-cbet]] and [[hands/hand-3]]")).toEqual([
      "patterns/folds-to-cbet",
      "hands/hand-3",
    ]);
  });
});

describe("read tools", () => {
  it("md_list_pages and md_read_page return deterministic results, including not-found", async () => {
    const dir = path.join(await mkdtemp(path.join(tmpdir(), "wiki-tools-")), "wiki");
    await writePage(dir, "villain", "# villain");
    const [listTool, readTool] = createReadTools(() => dir);

    expect(await callTool(listTool, {})).toEqual({ pages: ["villain"] });
    expect(await callTool(readTool, { page: "villain" })).toMatchObject({ found: true, page: "villain" });
    expect(await callTool(readTool, { page: "patterns/ghost" })).toEqual({ found: false, page: "patterns/ghost" });
  });

  it("returns empty/not-found when no wiki dir is available", async () => {
    const [listTool, readTool] = createReadTools(() => null);
    expect(await callTool(listTool, {})).toEqual({ pages: [] });
    expect(await callTool(readTool, { page: "villain" })).toEqual({ found: false, page: "villain" });
  });
});

describe("write tool", () => {
  it("creates a page and reports the slug; rejects an invalid name", async () => {
    const dir = path.join(await mkdtemp(path.join(tmpdir(), "wiki-write-")), "wiki");
    const writeTool = createWriteTool(() => dir);
    expect(await callTool(writeTool, { page: "patterns/3bettor", content: "3-bets 5/12" })).toEqual({
      written: true,
      page: "patterns/3bettor",
    });
    expect((await readPage(dir, "patterns/3bettor")).content).toContain("3-bets 5/12");
    expect(await callTool(writeTool, { page: "../escape", content: "x" })).toMatchObject({ written: false });
  });
});

describe("buildWikiUpdatePrompt", () => {
  it("includes the page list, current root, and the hand summary", () => {
    const prompt = buildWikiUpdatePrompt(["villain", "patterns/folds-to-cbet"], "# villain\n\nfolds to c-bet 4/7", "hand=9 | hero_result=+5");
    expect(prompt).toContain("Current pages: villain, patterns/folds-to-cbet");
    expect(prompt).toContain("folds to c-bet 4/7");
    expect(prompt).toContain("hand=9 | hero_result=+5");
  });
});

describe("MarkdownWikiMemoryPolicy.beforeDecision", () => {
  it("seeds villain.md and injects only the root index", async () => {
    const policy = new MarkdownWikiMemoryPolicy();
    const dir = await mkdtemp(path.join(tmpdir(), "wiki-before-"));

    const first = await policy.beforeDecision(decisionContext(dir));
    // villain.md is seeded and injected as the index; it is the only page so far.
    expect(first.sections).toHaveLength(2);
    expect(first.sections[1]).toContain("(no reads yet)");
    expect(await listPages(wikiDir(dir))).toEqual([ROOT_PAGE]);

    // Once the root has content, it is injected as the index (and nothing deeper).
    await writePage(wikiDir(dir), ROOT_PAGE, "# villain\n\nloose-aggressive; [[patterns/3bettor]]");
    await writePage(wikiDir(dir), "patterns/3bettor", "3-bets a lot");
    const second = await policy.beforeDecision(decisionContext(dir));
    expect(second.sections).toHaveLength(2);
    expect(second.sections[1]).toContain("loose-aggressive");
    expect(second.sections[1]).not.toContain("3-bets a lot");
  });
});

describe("runWikiUpdate scripted (fake) mode", () => {
  it("updates the root, adds a hand page, and writes a separate update transcript", async () => {
    const dir = await mkdtemp(path.join(tmpdir(), "wiki-update-"));
    const previousFake = process.env.PI_POKER_FAKE_DECISIONS_JSON;
    process.env.PI_POKER_FAKE_DECISIONS_JSON = JSON.stringify([{ action: "check" }]);
    try {
      await runWikiUpdate({ memoryDir: dir, handNumber: 1, handSummary: "hand=1 | hero_pos=sb/button | hero_hole=As Kh" });
    } finally {
      if (previousFake === undefined) delete process.env.PI_POKER_FAKE_DECISIONS_JSON;
      else process.env.PI_POKER_FAKE_DECISIONS_JSON = previousFake;
    }

    const root = await readPage(wikiDir(dir), ROOT_PAGE);
    expect(root.content).toContain("[[hands/hand-1]]");
    expect(root.content).not.toContain("hand=1 | hero_pos=sb/button | hero_hole=As Kh");

    const handPage = await readPage(wikiDir(dir), "hands/hand-1");
    expect(handPage.content).toContain("hand=1 | hero_pos=sb/button | hero_hole=As Kh");

    expect(await listPages(wikiDir(dir))).toEqual(["hands/hand-1", "villain"]);

    const updateLog = await readFile(path.join(dir, "update-session.jsonl"), "utf8");
    const entries = updateLog.trim().split("\n").map((line) => JSON.parse(line) as Record<string, unknown>);
    expect(entries[0]).toMatchObject({ type: "fake_update_session", hand_number: 1 });
  });
});
