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

describe("llm-akg-durable package wiring", () => {
  it("publishes a stable poker-server entrypoint", async () => {
    const pkg = JSON.parse(await readFile(packageJSONPath, "utf8")) as {
      bin?: Record<string, string>;
    };

    expect(pkg.bin).toEqual({
      "poker-agent-llm-akg-durable": "./dist/main.js",
    });
  });

  it("uses a hand-scoped custom Pi session with builtin tools disabled and the durable prompt contract", async () => {
    const source = await readFile(path.join(packageDir, "src", "main.ts"), "utf8");

    expect(source).toContain('sessionScope: "hand"');
    expect(source).toContain('noTools: "builtin"');
    expect(source).toContain('customTools: createQueryTools(() => memoryPolicy.getStore())');
    expect(source).toContain("You may call akg_list_patterns, akg_get_pattern, akg_list_hands, or akg_get_hand as needed before your final answer.");
    expect(source).toContain('Your final response must be JSON only: {"action": string, "amount"?: number}.');
  });

  it("runs as a subprocess, writes the canonical pi-session artifact, and exits cleanly on session_end", async () => {
    const { spawn } = await import("node:child_process");
    const sessionDir = path.join(repoRoot, "tmp-llm-akg-durable-session-" + Date.now().toString(36));
    const child = spawn(process.execPath, [entrypoint], {
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
