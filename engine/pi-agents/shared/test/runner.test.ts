import { PassThrough, Readable } from "node:stream";
import { describe, expect, it } from "vitest";

import { decodeEnvelope, type Envelope } from "../src/protocol.js";
import { runPokerAgent } from "../src/runner.js";
import type { CompletedHandContext, DecisionEngine, MemoryPolicy } from "../src/strategy.js";

class FakeDecisionEngine implements DecisionEngine {
  private attempt = 0;
  readonly prompts: string[] = [];
  readonly legalActionsSeen: Array<Array<{ action: "fold" | "check" | "call" | "bet" | "raise"; amount?: number; min?: number; max?: number }>> = [];

  constructor(
    private readonly outcomes: Array<
      | { type: "action"; value: { action: "fold" | "check" | "call" | "bet" | "raise"; amount?: number } }
      | { type: "error"; value: Error }
    >,
  ) {}

  async decide(request: Parameters<DecisionEngine["decide"]>[0]): Promise<{ action: "fold" | "check" | "call" | "bet" | "raise"; amount?: number }> {
    this.prompts.push(request.prompt);
    this.legalActionsSeen.push(request.legalActions.map((action) => ({ ...action })));

    const outcome = this.outcomes[Math.min(this.attempt, this.outcomes.length - 1)];
    this.attempt += 1;
    if (!outcome || outcome.type === "error") {
      throw outcome?.value ?? new Error("missing fake decision outcome");
    }
    return outcome.value;
  }
}

function sessionInitEnvelope(): Envelope {
  return {
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
  };
}

describe("runPokerAgent", () => {
  it("replies to session_init and your_turn and notifies memory policy on hand end with normalized completed-hand data", async () => {
    const afterHandEnds: CompletedHandContext[] = [];
    const prompts: string[] = [];
    const memoryPolicy: MemoryPolicy = {
      async beforeDecision(context) {
        prompts.push(`${context.handNumber}:${context.street}`);
        return { sections: ["Strategy note"] };
      },
      async afterHandEnd(context) {
        afterHandEnds.push(context);
      },
    };

    const stdout = new PassThrough();
    const outputChunks: Buffer[] = [];
    stdout.on("data", (chunk) => outputChunks.push(Buffer.from(chunk)));

    await runPokerAgent({
      memoryPolicy,
      decisionEngine: new FakeDecisionEngine([{ type: "action", value: { action: "call", amount: 2 } }]),
      agentVersion: "test/0.1.0",
      stdin: Readable.from([
        JSON.stringify(sessionInitEnvelope()),
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
            action_history: [
              { seat: 1, action: "call", amount: 1, street: "preflop" },
              { seat: 0, action: "check", street: "preflop" },
              { seat: 0, action: "bet", amount: 2, street: "flop" },
              { seat: 1, action: "fold", street: "flop" },
            ],
            showdown_reached: false,
            showdown: {
              "0": { hole_cards: ["As", "Kh"], rank: "" },
            },
            result: [
              { seat: 0, chips_delta: 14 },
              { seat: 1, chips_delta: -14 },
            ],
          },
        } satisfies Envelope),
        "\n",
        JSON.stringify({ v: 1, type: "session_end", id: "msg-5", payload: {} } satisfies Envelope),
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
    expect(afterHandEnds[0]).toMatchObject({
      handNumber: 7,
      dealerSeat: 1,
      heroSeat: 0,
      heroHoleCards: ["As", "Kh"],
      showdownReached: false,
    });
    expect(afterHandEnds[0]?.actionHistory).toHaveLength(4);
  });

  it("drives a complete two-hand stateless protocol conversation with correct reply correlation", async () => {
    const afterHandEnds: CompletedHandContext[] = [];
    const decisionEngine = new FakeDecisionEngine([
      { type: "action", value: { action: "call", amount: 2 } },
      { type: "action", value: { action: "check" } },
    ]);

    const stdout = new PassThrough();
    const outputChunks: Buffer[] = [];
    stdout.on("data", (chunk) => outputChunks.push(Buffer.from(chunk)));

    await runPokerAgent({
      memoryPolicy: {
        async beforeDecision() {
          return { sections: [] };
        },
        async afterHandEnd(context) {
          afterHandEnds.push(context);
        },
      },
      decisionEngine,
      agentVersion: "llm-stateless/0.1.0",
      stdin: Readable.from([
        JSON.stringify(sessionInitEnvelope()),
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
            action_history: [
              { seat: 0, action: "call", amount: 1, street: "preflop" },
              { seat: 1, action: "check", street: "preflop" },
            ],
            showdown_reached: true,
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
          type: "hand_start",
          id: "msg-5",
          payload: {
            hand_number: 8,
            dealer_seat: 0,
            stacks: { "0": 214, "1": 186 },
            blinds_posted: [
              { seat: 0, amount: 2 },
              { seat: 1, amount: 1 },
            ],
            your_hole_cards: ["Qc", "Qd"],
          },
        } satisfies Envelope),
        "\n",
        JSON.stringify({
          v: 1,
          type: "your_turn",
          id: "msg-6",
          payload: {
            hand_number: 8,
            street: "preflop",
            board: [],
            pot: 3,
            to_call: 0,
            stacks: { "0": 212, "1": 185 },
            seats: [
              { seat: 0, name: "hero" },
              { seat: 1, name: "villain" },
            ],
            action_history: [
              { seat: 0, action: "post_blind", amount: 2, street: "preflop" },
              { seat: 1, action: "post_blind", amount: 1, street: "preflop" },
            ],
            legal_actions: [{ action: "check" }, { action: "raise", min: 2, max: 212 }],
          },
        } satisfies Envelope),
        "\n",
        JSON.stringify({
          v: 1,
          type: "hand_end",
          id: "msg-7",
          payload: {
            hand_number: 8,
            board: ["7c", "4d", "2s", "Jh", "Tc"],
            action_history: [
              { seat: 0, action: "post_blind", amount: 2, street: "preflop" },
              { seat: 1, action: "post_blind", amount: 1, street: "preflop" },
              { seat: 0, action: "check", street: "preflop" },
            ],
            showdown_reached: false,
            result: [
              { seat: 0, chips_delta: 3 },
              { seat: 1, chips_delta: -3 },
            ],
          },
        } satisfies Envelope),
        "\n",
        JSON.stringify({ v: 1, type: "session_end", id: "msg-8", payload: {} } satisfies Envelope),
        "\n",
      ]),
      stdout,
    });

    const lines = Buffer.concat(outputChunks)
      .toString("utf8")
      .trim()
      .split("\n")
      .map((line) => decodeEnvelope(line));

    expect(lines).toHaveLength(3);
    expect(lines[0]).toMatchObject({
      type: "session_ready",
      in_reply_to: "msg-1",
      payload: { version: "llm-stateless/0.1.0" },
    });
    expect(lines[1]).toMatchObject({
      type: "action",
      in_reply_to: "msg-3",
      payload: { action: "call", amount: 2 },
    });
    expect(lines[2]).toMatchObject({
      type: "action",
      in_reply_to: "msg-6",
      payload: { action: "check" },
    });

    expect(afterHandEnds).toHaveLength(2);
    expect(afterHandEnds.map((entry) => entry.handNumber)).toEqual([7, 8]);

    expect(decisionEngine.prompts).toHaveLength(2);
    expect(decisionEngine.legalActionsSeen).toEqual([
      [{ action: "fold" }, { action: "call", amount: 2 }],
      [{ action: "check" }, { action: "raise", min: 2, max: 212 }],
    ]);

    expect(decisionEngine.prompts[0]).toContain("Hand: 7");
    expect(decisionEngine.prompts[0]).toContain('Board: ["Td","9h","2c"]');
    expect(decisionEngine.prompts[1]).toContain("Hand: 8");
    expect(decisionEngine.prompts[1]).toContain('Hole cards: ["Qc","Qd"]');
    expect(decisionEngine.prompts[1]).toContain('Board: []');
    expect(decisionEngine.prompts[1]).not.toContain("Previous hand result");
    expect(decisionEngine.prompts[1]).not.toContain("chips_delta");
    expect(decisionEngine.prompts[1]).not.toContain("two pair");
    expect(decisionEngine.prompts[1]).not.toContain('Board: ["Td","9h","2c","5s","Kc"]');
  });

  it("falls back to a safe legal action when the decision engine returns an illegal one", async () => {
    const stdout = new PassThrough();
    const outputChunks: Buffer[] = [];
    stdout.on("data", (chunk) => outputChunks.push(Buffer.from(chunk)));

    await runPokerAgent({
      memoryPolicy: {
        async beforeDecision() {
          return { sections: [] };
        },
        async afterHandEnd() {},
      },
      decisionEngine: new FakeDecisionEngine([{ type: "action", value: { action: "raise", amount: 99 } }]),
      agentVersion: "test/0.1.0",
      stdin: Readable.from([
        JSON.stringify(sessionInitEnvelope()),
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
        JSON.stringify({ v: 1, type: "session_end", id: "msg-5", payload: {} } satisfies Envelope),
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
    let stateAfterRun: Parameters<MemoryPolicy["beforeDecision"]>[0]["state"] | undefined;

    const memoryPolicy: MemoryPolicy = {
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
      memoryPolicy,
      decisionEngine: new FakeDecisionEngine([
        { type: "error", value: new Error("temporary failure") },
        { type: "error", value: new Error("still failing") },
      ]),
      agentVersion: "test/0.1.0",
      stdin: Readable.from([
        JSON.stringify(sessionInitEnvelope()),
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
            action_history: [
              { seat: 0, action: "check", street: "turn" },
              { seat: 1, action: "check", street: "turn" },
            ],
            showdown_reached: false,
            result: [
              { seat: 0, chips_delta: 2 },
              { seat: 1, chips_delta: -2 },
            ],
          },
        } satisfies Envelope),
        "\n",
        JSON.stringify({ v: 1, type: "session_end", id: "msg-5", payload: {} } satisfies Envelope),
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
    expect(Buffer.concat(stderrChunks).toString("utf8")).toContain("decision engine exhausted retries");
  });
});
