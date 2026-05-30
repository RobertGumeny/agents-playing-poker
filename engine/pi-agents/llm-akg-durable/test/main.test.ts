import { mkdtemp, readFile, rm } from "node:fs/promises";
import { writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { PassThrough } from "node:stream";

import { afterEach, describe, expect, it } from "vitest";

import {
  decodeEnvelope,
  type CompletedHandContext,
  type DecisionRequest,
  type Envelope,
} from "@agent-poker/pi-agent-shared";

import { AkgDurableMemoryPolicy } from "../src/memory.js";
import { createDecisionEngine, createDurableSessionFactory, DURABLE_SYSTEM_PROMPT } from "../src/runtime.js";

const repoRoot = path.resolve(import.meta.dirname, "..", "..", "..");
const packageDir = path.resolve(import.meta.dirname, "..");
const entrypoint = path.join(packageDir, "dist", "main.js");
const packageJSONPath = path.join(packageDir, "package.json");
const childProcesses: Array<import("node:child_process").ChildProcess> = [];
const tmpDirs: string[] = [];

afterEach(async () => {
  await Promise.all(
    childProcesses.map(
      (child) =>
        new Promise<void>((resolve) => {
          if (child.exitCode !== null) {
            resolve();
            return;
          }
          child.once("exit", () => resolve());
          child.kill();
        }),
    ),
  );
  childProcesses.length = 0;

  await Promise.all(tmpDirs.map((dir) => rm(dir, { recursive: true, force: true })));
  tmpDirs.length = 0;
});

function makeDecisionRequest(handNumber = 1): DecisionRequest {
  return {
    prompt: "Return JSON only.",
    legalActions: [{ action: "call", amount: 2 }],
    context: {
      state: { session: { memoryDir: "" } } as never,
      handNumber,
      street: "flop",
      board: ["Td", "9h", "2c"],
      pot: 6,
      toCall: 2,
      stacks: { "0": 197, "1": 197 },
      actionHistory: [{ seat: 0, action: "check", street: "preflop" }],
      legalActions: [{ action: "call", amount: 2 }],
    },
  };
}

function makeCompletedHandContext(handNumber = 1, memoryDir = ""): CompletedHandContext {
  return {
    state: { session: { memoryDir, match: { blinds: { bb: 2 } } } } as never,
    handNumber,
    dealerSeat: 1,
    heroSeat: 0,
    seats: [
      { seat: 0, name: "llm-akg-durable" },
      { seat: 1, name: "caller" },
    ],
    heroHoleCards: ["As", "Kh"],
    board: ["Td", "9h", "2c", "5s", "Kc"],
    actionHistory: [
      { seat: 0, action: "raise", amount: 4, street: "preflop" },
      { seat: 1, action: "call", amount: 2, street: "preflop" },
      { seat: 0, action: "bet", amount: 6, street: "flop" },
      { seat: 1, action: "fold", street: "flop" },
    ],
    showdownReached: false,
    result: [
      { seat: 0, chips_delta: 6 },
      { seat: 1, chips_delta: -6 },
    ],
  };
}

describe("llm-akg-durable package wiring", () => {
  it("publishes a stable poker-server entrypoint", async () => {
    const pkg = JSON.parse(await readFile(packageJSONPath, "utf8")) as {
      bin?: Record<string, string>;
    };

    expect(pkg.bin).toEqual({
      "poker-agent-llm-akg-durable": "./dist/main.js",
    });
  });

  it("registers only the AKG query tools, disables builtin tools, and preserves the JSON-only durable prompt contract", async () => {
    const memoryPolicy = new AkgDurableMemoryPolicy();
    const createAgentSessionCalls: Array<Record<string, unknown>> = [];
    const resourceLoaderOptions: Array<Record<string, unknown>> = [];
    const settingsOverrides: Array<Record<string, unknown>> = [];

    const factory = createDurableSessionFactory(memoryPolicy, {
      createAuthStorage: () => ({}) as never,
      createModelRegistry: () => ({ find: () => undefined, getAll: () => [] }) as never,
      createSettingsManager: () => ({
        applyOverrides(overrides: Record<string, unknown>) {
          settingsOverrides.push(overrides);
        },
      }) as never,
      createResourceLoader: (options) => {
        resourceLoaderOptions.push(options as Record<string, unknown>);
        return { reload: async () => {} } as never;
      },
      createSessionManager: () => ({}) as never,
      createAgentSession: async (options) => {
        createAgentSessionCalls.push(options as Record<string, unknown>);
        return {
          session: {
            prompt: async () => {},
            subscribe: () => () => {},
            getLastAssistantText: () => '{"action":"call","amount":2}',
            exportToJsonl: () => "",
            dispose: () => {},
          },
        } as never;
      },
    });

    await factory({
      cwd: repoRoot,
      agentDir: packageDir,
      model: undefined,
      thinkingLevel: undefined,
    });

    expect(settingsOverrides).toEqual([{ compaction: { enabled: false }, retry: { enabled: false } }]);
    expect(resourceLoaderOptions).toHaveLength(1);
    expect(resourceLoaderOptions[0]).toMatchObject({
      noExtensions: true,
      noSkills: true,
      noPromptTemplates: true,
      noThemes: true,
      noContextFiles: true,
    });
    expect((resourceLoaderOptions[0].systemPromptOverride as () => string)()).toBe(DURABLE_SYSTEM_PROMPT);

    expect(createAgentSessionCalls).toHaveLength(1);
    expect(createAgentSessionCalls[0].noTools).toBe("builtin");
    expect(((createAgentSessionCalls[0].customTools as Array<{ name: string }>)).map((tool) => tool.name)).toEqual([
      "akg_get_opponent",
      "akg_list_patterns",
      "akg_get_pattern",
      "akg_list_hands",
      "akg_get_hand",
    ]);
    expect(DURABLE_SYSTEM_PROMPT).toContain('Your final response must be JSON only: {"action": string, "amount"?: number}.');
    expect(DURABLE_SYSTEM_PROMPT).toContain("No commentary, markdown, code fences, or extra keys in the final JSON response.");
  });

  it("keeps the canonical pi-session artifact under the hand-scoped durable session lifecycle", async () => {
    const sessionDir = await mkdtemp(path.join(tmpdir(), "llm-akg-durable-engine-"));
    tmpDirs.push(sessionDir);

    const memoryPolicy = new AkgDurableMemoryPolicy();
    let sessionFactoryCalls = 0;
    let exportPaths: string[] = [];
    let disposed = 0;

    const engine = createDecisionEngine(memoryPolicy, {
      cwd: repoRoot,
      sessionDir,
      sessionFactory: async () => {
        sessionFactoryCalls += 1;
        return {
          prompt: async () => {},
          subscribe: () => () => {},
          getLastAssistantText: () => '{"action":"call","amount":2}',
          exportToJsonl: (outputPath?: string) => {
            exportPaths = [...exportPaths, outputPath ?? ""];
            if (outputPath) {
              writeFileSync(outputPath, '{"type":"fake_pi_session","session_scope":"hand"}\n', "utf8");
            }
            return outputPath ?? "";
          },
          dispose: () => {
            disposed += 1;
          },
        };
      },
    });

    await expect(engine.decide(makeDecisionRequest())).resolves.toEqual({ action: "call", amount: 2 });
    expect(sessionFactoryCalls).toBe(1);

    await engine.onHandEnd?.(makeCompletedHandContext(1, sessionDir));

    expect(exportPaths).toEqual([expect.stringContaining("pi-session-export-0001.jsonl")]);
    expect(disposed).toBe(1);
    await expect(readFile(path.join(sessionDir, "pi-session.jsonl"), "utf8")).resolves.toContain('"type":"fake_pi_session"');
  });

  it("runs as a subprocess with repeated extra args, speaks the protocol from session_init through session_end, and exits cleanly without live credentials", async () => {
    const { spawn } = await import("node:child_process");
    const sessionDir = path.join(repoRoot, "tmp-llm-akg-durable-session-" + Date.now().toString(36));
    tmpDirs.push(sessionDir);
    const child = spawn(process.execPath, [entrypoint, "--ignored-flag", "value", "--another-ignored-flag", "value-2"], {
      cwd: repoRoot,
      env: {
        ...process.env,
        PI_POKER_FAKE_DECISIONS_JSON: JSON.stringify([{ action: "call", amount: 2 }]),
      },
      stdio: ["pipe", "pipe", "pipe"],
    });
    childProcesses.push(child);

    const stdout = new PassThrough();
    const stderr = new PassThrough();
    child.stdout.pipe(stdout);
    child.stderr.pipe(stderr);

    const outputs: string[] = [];
    stdout.on("data", (chunk) => outputs.push(chunk.toString("utf8")));

    child.stdin.write(JSON.stringify({
      v: 1,
      type: "session_init",
      id: "msg-1",
      payload: {
        session_id: "ses-1",
        agent_name: "llm-akg-durable",
        match: {
          match_id: "mat-1",
          seed: 1,
          hand_count: 1,
          variant: "heads-up-nlhe",
          info_realism: "showdown-only",
          starting_stack: 200,
          blinds: { sb: 1, bb: 2 },
          decision_deadline_ms: 30000,
        },
        seats: [
          { seat: 0, name: "llm-akg-durable" },
          { seat: 1, name: "caller" },
        ],
        your_seat: 0,
        memory_dir: sessionDir,
      },
    } satisfies Envelope) + "\n");
    child.stdin.write(JSON.stringify({
      v: 1,
      type: "hand_start",
      id: "msg-2",
      payload: {
        hand_number: 1,
        dealer_seat: 1,
        stacks: { "0": 200, "1": 200 },
        blinds_posted: [
          { seat: 0, amount: 1 },
          { seat: 1, amount: 2 },
        ],
        your_hole_cards: ["As", "Kh"],
      },
    } satisfies Envelope) + "\n");
    child.stdin.write(JSON.stringify({
      v: 1,
      type: "your_turn",
      id: "msg-3",
      payload: {
        hand_number: 1,
        street: "flop",
        board: ["Td", "9h", "2c"],
        pot: 6,
        to_call: 2,
        stacks: { "0": 197, "1": 197 },
        seats: [
          { seat: 0, name: "llm-akg-durable" },
          { seat: 1, name: "caller" },
        ],
        action_history: [{ seat: 0, action: "check", street: "preflop" }],
        legal_actions: [
          { action: "fold" },
          { action: "call", amount: 2 },
        ],
      },
    } satisfies Envelope) + "\n");
    child.stdin.write(JSON.stringify({
      v: 1,
      type: "hand_end",
      id: "msg-4",
      payload: {
        hand_number: 1,
        board: ["Td", "9h", "2c", "5s", "Kc"],
        action_history: [
          { seat: 0, action: "raise", amount: 4, street: "preflop" },
          { seat: 1, action: "call", amount: 2, street: "preflop" },
          { seat: 0, action: "bet", amount: 6, street: "flop" },
          { seat: 1, action: "fold", street: "flop" },
        ],
        showdown_reached: false,
        result: [
          { seat: 0, chips_delta: 6 },
          { seat: 1, chips_delta: -6 },
        ],
      },
    } satisfies Envelope) + "\n");
    child.stdin.write(JSON.stringify({ v: 1, type: "session_end", id: "msg-5", payload: {} satisfies Record<string, never> }) + "\n");
    child.stdin.end();

    await new Promise<void>((resolve, reject) => {
      child.once("error", reject);
      child.once("exit", (code) => {
        if (code !== 0) {
          reject(new Error(`llm-akg-durable exited with code ${code}: ${stderr.read()?.toString("utf8") ?? ""}`));
          return;
        }
        resolve();
      });
    });

    const decoded = outputs
      .join("")
      .trim()
      .split("\n")
      .filter((line) => line.length > 0)
      .map((line) => decodeEnvelope(line));

    expect(decoded).toHaveLength(2);
    expect(decoded[0]).toMatchObject({
      type: "session_ready",
      in_reply_to: "msg-1",
      payload: { version: "llm-akg-durable/0.1.0" },
    });
    expect(decoded[1]).toMatchObject({
      type: "action",
      in_reply_to: "msg-3",
      payload: { action: "call", amount: 2 },
    });
    await expect(readFile(path.join(sessionDir, "pi-session.jsonl"), "utf8")).resolves.toContain('"type":"fake_pi_session"');
  });
});
