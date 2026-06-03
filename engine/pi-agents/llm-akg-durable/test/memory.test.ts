import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { open, type Store } from "akg-ts";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import type { DecisionContext } from "@agent-poker/pi-agent-shared";

import { AkgDurableMemoryPolicy } from "../src/memory.js";
import { ensureRootNode, ROOT_ID, ROOT_TYPE, STORE_FILE } from "../src/graph.js";
import { createReadTools, createWriteTools, type StoreProvider } from "../src/tools.js";
import { buildDurableUpdatePrompt, runDurableUpdate } from "../src/update.js";

let tmpDir: string;

beforeEach(async () => {
  tmpDir = await mkdtemp(join(tmpdir(), "akg-durable-test-"));
});

afterEach(async () => {
  await rm(tmpDir, { recursive: true, force: true });
});

function makeDecisionContext(memoryDir: string): DecisionContext {
  return {
    state: { session: memoryDir ? { memoryDir } : undefined } as never,
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

async function callTool(tools: ReturnType<typeof createReadTools>, name: string, params: unknown = {}) {
  const tool = tools.find((entry) => entry.name === name);
  if (!tool) throw new Error(`missing tool ${name}`);
  const result = (await (tool.execute as (...args: unknown[]) => Promise<{ details: unknown }>)(
    "call-1",
    params,
    new AbortController().signal,
    async () => {},
    undefined,
  )).details;
  return result;
}

function storeProvider(store: Store | null): StoreProvider {
  return async () => store;
}

describe("AkgDurableMemoryPolicy.beforeDecision", () => {
  it("seeds the root node and injects only the root index", async () => {
    const policy = new AkgDurableMemoryPolicy();

    const first = await policy.beforeDecision(makeDecisionContext(tmpDir));
    expect(first.sections).toHaveLength(2);
    expect(first.sections[1]).toContain("(no reads yet)");

    const store = (await policy.getStore(tmpDir))!;
    expect(store.getNode(ROOT_TYPE, ROOT_ID)).not.toBeNull();

    // Once the root body has content, it is injected as the index. Mutate through the policy's
    // own store handle so the next beforeDecision sees it (single writer, single handle).
    store.putNode(ROOT_TYPE, ROOT_ID, { title: ROOT_ID, body: "loose-aggressive; 3-bets a lot" }, [ROOT_TYPE]);
    await store.commit();
    const second = await policy.beforeDecision(makeDecisionContext(tmpDir));
    expect(second.sections).toHaveLength(2);
    expect(second.sections[1]).toContain("loose-aggressive");
  });

  it("falls back when no memory dir is provided", async () => {
    const policy = new AkgDurableMemoryPolicy();
    const result = await policy.beforeDecision(makeDecisionContext(""));
    expect(result.sections).toHaveLength(1);
    expect(result.sections[0]).toContain("no reads yet");
  });
});

describe("read tools", () => {
  it("akg_list_nodes and akg_get_node return deterministic results, including not-found", async () => {
    const store = await open(join(tmpDir, STORE_FILE));
    ensureRootNode(store);
    store.putNode("pattern", "folds-to-cbet", { title: "Folds to c-bets", body: "folds 4/7" }, ["pattern"]);
    store.putEdge({ type: ROOT_TYPE, id: ROOT_ID }, "shows", { type: "pattern", id: "folds-to-cbet" }, {});
    await store.commit();
    const read = createReadTools(storeProvider(store));

    const list = (await callTool(read, "akg_list_nodes")) as { nodes: Array<{ type: string; id: string }> };
    expect(list.nodes.map((n) => `${n.type}/${n.id}`).sort()).toEqual(["opponent/villain", "pattern/folds-to-cbet"]);

    const filtered = (await callTool(read, "akg_list_nodes", { type: "pattern" })) as { nodes: unknown[] };
    expect(filtered.nodes).toHaveLength(1);

    const node = (await callTool(read, "akg_get_node", { type: "pattern", id: "folds-to-cbet" })) as Record<string, unknown>;
    expect(node).toMatchObject({ found: true, title: "Folds to c-bets" });
    expect((node.inbound_edges as unknown[]).length).toBe(1);

    const missing = await callTool(read, "akg_get_node", { type: "pattern", id: "ghost" });
    expect(missing).toEqual({ found: false, type: "pattern", id: "ghost" });
  });

  it("returns empty/not-found when no store is available", async () => {
    const read = createReadTools(storeProvider(null));
    expect(await callTool(read, "akg_list_nodes")).toEqual({ nodes: [] });
    expect(await callTool(read, "akg_get_node", { type: "opponent", id: "villain" })).toEqual({
      found: false,
      type: "opponent",
      id: "villain",
    });
  });
});

describe("write tools", () => {
  it("creates nodes and edges and reports structured errors", async () => {
    const store = await open(join(tmpDir, STORE_FILE));
    ensureRootNode(store);
    const write = createWriteTools(storeProvider(store));

    expect(await callTool(write, "akg_put_node", { type: "pattern", id: "3bettor", title: "3-bets a lot", body: "3-bets 5/12" })).toEqual({
      written: true,
      type: "pattern",
      id: "3bettor",
    });
    expect(store.getNode("pattern", "3bettor")?.body).toContain("3-bets 5/12");

    expect(await callTool(write, "akg_put_edge", {
      from_type: ROOT_TYPE,
      from_id: ROOT_ID,
      relation: "shows",
      to_type: "pattern",
      to_id: "3bettor",
    })).toMatchObject({ written: true, relation: "shows" });

    // Edge to a missing endpoint is reported, not thrown.
    const dangling = (await callTool(write, "akg_put_edge", {
      from_type: ROOT_TYPE,
      from_id: ROOT_ID,
      relation: "shows",
      to_type: "pattern",
      to_id: "ghost",
    })) as { written: boolean; error: string };
    expect(dangling.written).toBe(false);
    expect(dangling.error).toMatch(/not found/i);

    // Invalid type name is reported, not thrown.
    const invalid = (await callTool(write, "akg_put_node", { type: "Bad Type", id: "x", title: "x" })) as { written: boolean };
    expect(invalid.written).toBe(false);
  });
});

describe("buildDurableUpdatePrompt", () => {
  it("includes the node list, current root body, and the hand summary", () => {
    const prompt = buildDurableUpdatePrompt(
      ["opponent/villain — villain", "pattern/folds-to-cbet — Folds to c-bets"],
      "loose-aggressive; folds to c-bet 4/7",
      "hand=9 | hero_result=+5",
    );
    expect(prompt).toContain("pattern/folds-to-cbet — Folds to c-bets");
    expect(prompt).toContain("loose-aggressive; folds to c-bet 4/7");
    expect(prompt).toContain("hand=9 | hero_result=+5");
  });
});

describe("runDurableUpdate scripted (fake) mode", () => {
  it("writes a hand node + root edge, updates the root body, and logs a separate transcript and diagnostic", async () => {
    const store = await open(join(tmpDir, STORE_FILE));
    const previousFake = process.env.PI_POKER_FAKE_DECISIONS_JSON;
    process.env.PI_POKER_FAKE_DECISIONS_JSON = JSON.stringify([{ action: "check" }]);
    try {
      await runDurableUpdate({
        memoryDir: tmpDir,
        handNumber: 1,
        handSummary: "hand=1 | hero_pos=sb/button | hero_hole=As Kh",
        getStore: storeProvider(store),
      });
    } finally {
      if (previousFake === undefined) delete process.env.PI_POKER_FAKE_DECISIONS_JSON;
      else process.env.PI_POKER_FAKE_DECISIONS_JSON = previousFake;
    }

    const reopened = await open(join(tmpDir, STORE_FILE));
    const hand = reopened.getNode("hand", "hand-1");
    expect(hand).not.toBeNull();
    expect(hand!.body).toContain("hand=1 | hero_pos=sb/button | hero_hole=As Kh");
    expect(reopened.getNode(ROOT_TYPE, ROOT_ID)!.body).toContain("hand=1 | hero_pos=sb/button | hero_hole=As Kh");
    expect(reopened.outboundEdges({ type: ROOT_TYPE, id: ROOT_ID }, "has_hand")).toHaveLength(1);

    const updateLog = await readFile(join(tmpDir, "update-session.jsonl"), "utf8");
    const entries = updateLog.trim().split("\n").map((line) => JSON.parse(line) as Record<string, unknown>);
    expect(entries[0]).toMatchObject({ type: "fake_update_session", hand_number: 1 });

    const diagnostics = await readFile(join(tmpDir, "diagnostics.jsonl"), "utf8");
    const diag = JSON.parse(diagnostics.trim().split("\n")[0]) as Record<string, unknown>;
    expect(diag).toMatchObject({ type: "graph_rot", hand_number: 1 });
    expect(diag.orphan_nodes).toBe(0);
  });
});
