import { describe, expect, it, vi } from "vitest";

import { PiDecisionClient, parsePiThinkingLevel } from "../src/pi-session.js";

describe("PiDecisionClient", () => {
  it("creates a fresh Pi session per decision and parses assistant JSON", async () => {
    const exportedPaths: string[] = [];
    const sessionFactory = vi
      .fn()
      .mockImplementation(async () => ({
        async prompt() {},
        subscribe() {
          return () => {};
        },
        getLastAssistantText() {
          return '{"action":"call","amount":2}';
        },
        exportToJsonl(outputPath?: string) {
          exportedPaths.push(outputPath ?? "");
          return outputPath ?? "";
        },
        dispose() {},
      }));

    const client = new PiDecisionClient({
      cwd: process.cwd(),
      sessionDir: "/tmp/pi-audit",
      sessionFactory,
    });

    await expect(client.decide("first", [{ action: "call", amount: 2 }])).resolves.toEqual({ action: "call", amount: 2 });
    await expect(client.decide("second", [{ action: "check" }])).resolves.toEqual({ action: "call", amount: 2 });

    expect(sessionFactory).toHaveBeenCalledTimes(2);
    expect(exportedPaths).toEqual(["/tmp/pi-audit/pi-session-0001.jsonl", "/tmp/pi-audit/pi-session-0002.jsonl"]);
  });

  it("falls back to streamed assistant text when needed and surfaces malformed JSON", async () => {
    const sessionFactory = vi.fn().mockImplementation(async () => ({
      async prompt() {
        listener?.({
          type: "message_update",
          assistantMessageEvent: { type: "text_delta", delta: '{"action":"dance"}' },
        } as never);
      },
      subscribe(next: (event: never) => void) {
        listener = next;
        return () => {
          listener = undefined;
        };
      },
      getLastAssistantText() {
        return undefined;
      },
      exportToJsonl() {
        return "";
      },
      dispose() {},
    }));

    let listener: ((event: never) => void) | undefined;
    const client = new PiDecisionClient({ cwd: process.cwd(), sessionFactory });

    await expect(client.decide("decision", [{ action: "fold" }])).rejects.toThrow(
      'pi decision failed: assistant returned malformed action JSON: "{\\"action\\":\\"dance\\"}"',
    );
  });
});

describe("parsePiThinkingLevel", () => {
  it("accepts supported Pi thinking levels", () => {
    expect(parsePiThinkingLevel(undefined)).toBeUndefined();
    expect(parsePiThinkingLevel("off")).toBe("off");
    expect(parsePiThinkingLevel("high")).toBe("high");
  });

  it("rejects unsupported Pi thinking levels", () => {
    expect(() => parsePiThinkingLevel("extreme")).toThrow('invalid Pi thinking level "extreme"');
  });
});
