import { writeFileSync } from "node:fs";
import { mkdtemp, readdir, readFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";

import { describe, expect, it, vi } from "vitest";

import { PiDecisionClient, parsePiThinkingLevel } from "../src/pi-session.js";

describe("PiDecisionClient", () => {
  it("creates a fresh Pi session per decision and appends to the canonical pi-session artifact", async () => {
    const sessionDir = await mkdtemp(path.join(os.tmpdir(), "pi-session-test-"));
    const exportedPaths: string[] = [];
    let exportCount = 0;
    const sessionFactory = vi.fn().mockImplementation(async () => ({
      async prompt() {},
      subscribe() {
        return () => {};
      },
      getLastAssistantText() {
        return '{"action":"call","amount":2}';
      },
      exportToJsonl(outputPath?: string) {
        exportedPaths.push(outputPath ?? "");
        const content = `{"decision":${++exportCount}}\n`;
        if (!outputPath) return "";
        writeFileSync(outputPath, content, "utf8");
        return outputPath;
      },
      dispose() {},
    }));

    const client = new PiDecisionClient({
      cwd: process.cwd(),
      sessionDir,
      sessionFactory,
    });

    await expect(client.decide("first", [{ action: "call", amount: 2 }])).resolves.toEqual({ action: "call", amount: 2 });
    await expect(client.decide("second", [{ action: "check" }])).resolves.toEqual({ action: "call", amount: 2 });

    expect(sessionFactory).toHaveBeenCalledTimes(2);
    expect(exportedPaths).toEqual([
      path.join(sessionDir, "pi-session-export-0001.jsonl"),
      path.join(sessionDir, "pi-session-export-0002.jsonl"),
    ]);
    await expect(readFile(path.join(sessionDir, "pi-session.jsonl"), "utf8")).resolves.toBe('{"decision":1}\n{"decision":2}\n');
    await expect(readdir(sessionDir)).resolves.toEqual(["pi-session.jsonl"]);
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
