import { PassThrough, Readable } from "node:stream";
import { describe, expect, it } from "vitest";

import { decodeEnvelope, type Envelope } from "../src/protocol.js";
import { runPokerAgent } from "../src/runner.js";
import type { DecisionClient, HandEndContext, MemoryStrategy } from "../src/strategy.js";

class FakeDecisionClient implements DecisionClient {
  constructor(private readonly action: { action: "fold" | "check" | "call" | "bet" | "raise"; amount?: number }) {}

  async decide(): Promise<{ action: "fold" | "check" | "call" | "bet" | "raise"; amount?: number }> {
    return this.action;
  }
}

describe("runPokerAgent", () => {
  it("replies to session_init and your_turn and notifies the strategy on hand end", async () => {
    const afterHandEnds: HandEndContext[] = [];
    const prompts: string[] = [];
    const strategy: MemoryStrategy = {
      name: "test",
      version: "test/0.1.0",
      async beforeDecision(context: Parameters<MemoryStrategy["beforeDecision"]>[0]) {
        prompts.push(`${context.handNumber}:${context.street}`);
        return { sections: ["Strategy note"] };
      },
      async afterHandEnd(context: HandEndContext) {
        afterHandEnds.push(context);
      },
    };

    const stdout = new PassThrough();
    const outputChunks: Buffer[] = [];
    stdout.on("data", (chunk) => outputChunks.push(Buffer.from(chunk)));

    await runPokerAgent({
      strategy,
      decisionClient: new FakeDecisionClient({ action: "call", amount: 2 }),
      stdin: Readable.from([
        JSON.stringify({
          v: 1,
          type: "session_init",
          id: "msg-1",
          payload: {
            session_id: "ses-1",
            agent_name: "llm-nomemory",
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
          },
        } satisfies Envelope),
        "\n",
        JSON.stringify({
          v: 1,
          type: "hand_start",
          id: "msg-2",
          payload: {
            hand_number: 7,
            dealer_seat: 1,
            stacks: { "0": 200, "1": 200 },
            blinds_posted: [
              { seat: 0, amount: 1 },
              { seat: 1, amount: 2 },
            ],
            your_hole_cards: ["As", "Kh"],
          },
        } satisfies Envelope),
        "\n",
        JSON.stringify({
          v: 1,
          type: "your_turn",
          id: "msg-3",
          payload: {
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
            action_history: [{ seat: 0, action: "check", street: "preflop" }],
            legal_actions: [
              { action: "fold" },
              { action: "call", amount: 2 },
            ],
          },
        } satisfies Envelope),
        "\n",
        JSON.stringify({
          v: 1,
          type: "hand_end",
          id: "msg-4",
          payload: {
            hand_number: 7,
            board: ["Td", "9h", "2c", "5s", "Kc"],
            showdown: {
              "0": { hole_cards: ["As", "Kh"], rank: "two pair" },
            },
            result: [
              { seat: 0, chips_delta: 14 },
              { seat: 1, chips_delta: -14 },
            ],
          },
        } satisfies Envelope),
        "\n",
        JSON.stringify({
          v: 1,
          type: "session_end",
          id: "msg-5",
          payload: {},
        } satisfies Envelope),
        "\n",
      ]),
      stdout,
    });

    const lines = Buffer.concat(outputChunks)
      .toString("utf8")
      .trim()
      .split("\n")
      .map((line) => decodeEnvelope(line));

    expect(lines).toHaveLength(2);
    expect(lines[0]).toMatchObject({
      type: "session_ready",
      in_reply_to: "msg-1",
      payload: { version: "test/0.1.0" },
    });
    expect(lines[1]).toMatchObject({
      type: "action",
      in_reply_to: "msg-3",
      payload: { action: "call", amount: 2 },
    });
    expect(prompts).toEqual(["7:flop"]);
    expect(afterHandEnds).toHaveLength(1);
    expect(afterHandEnds[0]?.handNumber).toBe(7);
  });

  it("falls back to a safe legal action when the decision client returns an illegal one", async () => {
    const stdout = new PassThrough();
    const outputChunks: Buffer[] = [];
    stdout.on("data", (chunk) => outputChunks.push(Buffer.from(chunk)));

    await runPokerAgent({
      strategy: {
        name: "test",
        version: "test/0.1.0",
        async beforeDecision() {
          return { sections: [] };
        },
        async afterHandEnd() {},
      },
      decisionClient: new FakeDecisionClient({ action: "raise", amount: 99 }),
      stdin: Readable.from([
        JSON.stringify({
          v: 1,
          type: "session_init",
          id: "msg-1",
          payload: {
            session_id: "ses-1",
            agent_name: "llm-nomemory",
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
          },
        } satisfies Envelope),
        "\n",
        JSON.stringify({
          v: 1,
          type: "hand_start",
          id: "msg-2",
          payload: {
            hand_number: 7,
            dealer_seat: 1,
            stacks: { "0": 200, "1": 200 },
            blinds_posted: [
              { seat: 0, amount: 1 },
              { seat: 1, amount: 2 },
            ],
            your_hole_cards: ["As", "Kh"],
          },
        } satisfies Envelope),
        "\n",
        JSON.stringify({
          v: 1,
          type: "your_turn",
          id: "msg-3",
          payload: {
            hand_number: 7,
            street: "flop",
            board: ["Td", "9h", "2c"],
            pot: 6,
            to_call: 0,
            stacks: { "0": 197, "1": 197 },
            seats: [
              { seat: 0, name: "hero" },
              { seat: 1, name: "villain" },
            ],
            action_history: [],
            legal_actions: [{ action: "check" }, { action: "fold" }],
          },
        } satisfies Envelope),
        "\n",
        JSON.stringify({
          v: 1,
          type: "session_end",
          id: "msg-5",
          payload: {},
        } satisfies Envelope),
        "\n",
      ]),
      stdout,
    });

    const lines = Buffer.concat(outputChunks)
      .toString("utf8")
      .trim()
      .split("\n")
      .map((line) => decodeEnvelope(line));

    expect(lines[1]).toMatchObject({
      type: "action",
      payload: { action: "check" },
    });
  });
});
