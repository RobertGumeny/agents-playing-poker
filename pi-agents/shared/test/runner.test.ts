import { PassThrough, Readable } from "node:stream";
import { describe, expect, it } from "vitest";

import { decodeEnvelope, type Envelope } from "../src/protocol.js";
import { runPokerAgent } from "../src/runner.js";
import type { DecisionClient, HandEndContext, MemoryStrategy } from "../src/strategy.js";

class FakeDecisionClient implements DecisionClient {
  private attempt = 0;

  constructor(
    private readonly outcomes: Array<
      | { type: "action"; value: { action: "fold" | "check" | "call" | "bet" | "raise"; amount?: number } }
      | { type: "error"; value: Error }
    >,
  ) {}

  async decide(): Promise<{ action: "fold" | "check" | "call" | "bet" | "raise"; amount?: number }> {
    const outcome = this.outcomes[Math.min(this.attempt, this.outcomes.length - 1)];
    this.attempt += 1;
    if (!outcome || outcome.type === "error") {
      throw outcome?.value ?? new Error("missing fake decision outcome");
    }
    return outcome.value;
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
      decisionClient: new FakeDecisionClient([{ type: "action", value: { action: "call", amount: 2 } }]),
      stdin: Readable.from([
        JSON.stringify({
          v: 1,
          type: "session_init",
          id: "msg-1",
          payload: {
            session_id: "ses-1",
            agent_name: "llm-stateless",
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
      decisionClient: new FakeDecisionClient([{ type: "action", value: { action: "raise", amount: 99 } }]),
      stdin: Readable.from([
        JSON.stringify({
          v: 1,
          type: "session_init",
          id: "msg-1",
          payload: {
            session_id: "ses-1",
            agent_name: "llm-stateless",
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

  it("retries decision failures, falls back safely, and clears shared state at hand/session end", async () => {
    const stderr = new PassThrough();
    const stderrChunks: Buffer[] = [];
    stderr.on("data", (chunk) => stderrChunks.push(Buffer.from(chunk)));

    let stateBeforeDecisionHandNumber: number | undefined;
    let handStateSeenOnHandEnd = false;
    let stateAfterRun: Parameters<MemoryStrategy["beforeDecision"]>[0]["state"] | undefined;

    const strategy: MemoryStrategy = {
      name: "test",
      version: "test/0.1.0",
      async beforeDecision(context) {
        stateBeforeDecisionHandNumber = context.state.hand?.handNumber;
        stateAfterRun = context.state;
        return { sections: [] };
      },
      async afterHandEnd(context) {
        handStateSeenOnHandEnd = context.state.hand?.handNumber === context.handNumber;
      },
    };

    const stdout = new PassThrough();
    const outputChunks: Buffer[] = [];
    stdout.on("data", (chunk) => outputChunks.push(Buffer.from(chunk)));

    await runPokerAgent({
      strategy,
      decisionClient: new FakeDecisionClient([
        { type: "error", value: new Error("temporary failure") },
        { type: "error", value: new Error("still failing") },
      ]),
      stdin: Readable.from([
        JSON.stringify({
          v: 1,
          type: "session_init",
          id: "msg-1",
          payload: {
            session_id: "ses-1",
            agent_name: "llm-stateless",
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
            hand_number: 11,
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
            hand_number: 11,
            street: "turn",
            board: ["Td", "9h", "2c", "5s"],
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
          type: "hand_end",
          id: "msg-4",
          payload: {
            hand_number: 11,
            board: ["Td", "9h", "2c", "5s", "Kc"],
            result: [
              { seat: 0, chips_delta: 2 },
              { seat: 1, chips_delta: -2 },
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
      stderr,
      maxDecisionAttempts: 2,
    });

    const lines = Buffer.concat(outputChunks)
      .toString("utf8")
      .trim()
      .split("\n")
      .map((line) => decodeEnvelope(line));

    expect(lines[1]).toMatchObject({
      type: "action",
      in_reply_to: "msg-3",
      payload: { action: "check" },
    });
    expect(stateBeforeDecisionHandNumber).toBe(11);
    expect(handStateSeenOnHandEnd).toBe(true);
    expect(stateAfterRun?.hand).toBeUndefined();
    expect(stateAfterRun?.session).toBeUndefined();
    expect(Buffer.concat(stderrChunks).toString("utf8")).toContain("decision client exhausted retries");
  });
});
