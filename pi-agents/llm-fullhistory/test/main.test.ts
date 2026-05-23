import { readFile } from "node:fs/promises";
import path from "node:path";
import { PassThrough } from "node:stream";

import { afterEach, describe, expect, it } from "vitest";

import { decodeEnvelope, type Envelope } from "@agent-poker/pi-agent-shared";

const repoRoot = path.resolve(import.meta.dirname, "..", "..", "..");
const packageDir = path.resolve(import.meta.dirname, "..");
const entrypoint = path.join(packageDir, "dist", "main.js");
const packageJSONPath = path.join(packageDir, "package.json");
const childProcesses: Array<import("node:child_process").ChildProcess> = [];

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
});

describe("llm-fullhistory package wiring", () => {
  it("publishes a stable poker-server entrypoint", async () => {
    const pkg = JSON.parse(await readFile(packageJSONPath, "utf8")) as {
      bin?: Record<string, string>;
    };

    expect(pkg.bin).toEqual({
      "poker-agent-llm-fullhistory": "./dist/main.js",
    });
  });

  it("runs as a subprocess, reuses one fake Pi session per hand, and injects prior-hand history into later prompts", async () => {
    const { spawn } = await import("node:child_process");
    const sessionDir = path.join(repoRoot, "tmp-llm-fullhistory-session-" + Date.now().toString(36));
    const child = spawn(process.execPath, [entrypoint], {
      cwd: repoRoot,
      env: {
        ...process.env,
        PI_POKER_FAKE_DECISIONS_JSON: JSON.stringify([{ action: "call", amount: 2 }, { action: "check" }]),
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
        agent_name: "llm-fullhistory",
        match: {
          match_id: "mat-1",
          seed: 1,
          hand_count: 2,
          variant: "heads-up-nlhe",
          info_realism: "showdown-only",
          starting_stack: 200,
          blinds: { sb: 1, bb: 2 },
          decision_deadline_ms: 30000,
        },
        seats: [
          { seat: 0, name: "llm-fullhistory" },
          { seat: 1, name: "heuristic" },
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
        dealer_seat: 0,
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
          { seat: 0, name: "llm-fullhistory" },
          { seat: 1, name: "heuristic" },
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
          { seat: 0, action: "call", amount: 1, street: "preflop" },
          { seat: 1, action: "check", street: "preflop" },
          { seat: 0, action: "bet", amount: 2, street: "flop" },
          { seat: 1, action: "fold", street: "flop" },
        ],
        showdown_reached: false,
        showdown: {
          "0": { hole_cards: ["As", "Kh"], rank: "" },
        },
        result: [
          { seat: 0, chips_delta: 3 },
          { seat: 1, chips_delta: -3 },
        ],
      },
    } satisfies Envelope) + "\n");
    child.stdin.write(JSON.stringify({
      v: 1,
      type: "hand_start",
      id: "msg-5",
      payload: {
        hand_number: 2,
        dealer_seat: 1,
        stacks: { "0": 203, "1": 197 },
        blinds_posted: [
          { seat: 0, amount: 2 },
          { seat: 1, amount: 1 },
        ],
        your_hole_cards: ["Qc", "Qd"],
      },
    } satisfies Envelope) + "\n");
    child.stdin.write(JSON.stringify({
      v: 1,
      type: "your_turn",
      id: "msg-6",
      payload: {
        hand_number: 2,
        street: "preflop",
        board: [],
        pot: 3,
        to_call: 0,
        stacks: { "0": 201, "1": 196 },
        seats: [
          { seat: 0, name: "llm-fullhistory" },
          { seat: 1, name: "heuristic" },
        ],
        action_history: [
          { seat: 0, action: "post_blind", amount: 2, street: "preflop" },
          { seat: 1, action: "post_blind", amount: 1, street: "preflop" },
        ],
        legal_actions: [{ action: "check" }],
      },
    } satisfies Envelope) + "\n");
    child.stdin.write(JSON.stringify({
      v: 1,
      type: "session_end",
      id: "msg-7",
      payload: {},
    } satisfies Envelope) + "\n");
    child.stdin.end();

    await new Promise<void>((resolve, reject) => {
      child.once("error", reject);
      child.once("exit", (code) => {
        if (code !== 0) {
          reject(new Error(`llm-fullhistory exited with code ${code}: ${stderr.read()?.toString("utf8") ?? ""}`));
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

    expect(decoded).toHaveLength(3);
    expect(decoded[0]).toMatchObject({
      type: "session_ready",
      in_reply_to: "msg-1",
      payload: { version: "llm-fullhistory/0.1.0" },
    });
    expect(decoded[1]).toMatchObject({
      type: "action",
      in_reply_to: "msg-3",
      payload: { action: "call", amount: 2 },
    });
    expect(decoded[2]).toMatchObject({
      type: "action",
      in_reply_to: "msg-6",
      payload: { action: "check" },
    });

    const sessionLog = await readFile(path.join(sessionDir, "pi-session.jsonl"), "utf8");
    const lines = sessionLog.trim().split("\n").map((line) => JSON.parse(line) as Record<string, unknown>);
    expect(lines).toHaveLength(2);
    expect(lines[0]).toMatchObject({ session_scope: "hand", session_number: 1, hand_number: 1 });
    expect(lines[1]).toMatchObject({ session_scope: "hand", session_number: 2, hand_number: 2 });
    expect(String(lines[0].prompt)).toContain("Prior hands: none yet.");
    expect(String(lines[1].prompt)).toContain("Prior hands:");
    expect(String(lines[1].prompt)).toContain("hand=1 | hero_pos=sb/button | hero_hole=As Kh");
  });
});
