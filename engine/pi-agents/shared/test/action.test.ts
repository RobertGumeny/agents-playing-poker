import { describe, expect, it } from "vitest";

import { parseActionResponse, validateOrFallback } from "../src/action.js";

describe("parseActionResponse", () => {
  it("accepts a strict JSON action object", () => {
    expect(parseActionResponse('{"action":"call","amount":2}')).toEqual({
      action: "call",
      amount: 2,
    });
  });

  it("recovers the action when the model leaks reasoning prose before the JSON", () => {
    expect(parseActionResponse('answer: {"action":"call","amount":2}')).toEqual({
      action: "call",
      amount: 2,
    });
    expect(
      parseActionResponse('QJsuited is a strong hand here.\n\n{"action":"raise","amount":8}'),
    ).toEqual({ action: "raise", amount: 8 });
  });

  it("recovers the action from a fenced code block", () => {
    expect(parseActionResponse('```json\n{"action":"fold"}\n```')).toEqual({ action: "fold" });
  });

  it("prefers the final action object when several appear", () => {
    expect(
      parseActionResponse('I considered {"action":"fold"} but instead {"action":"check"}'),
    ).toEqual({ action: "check" });
  });

  it("ignores embedded objects that are not legal action shapes", () => {
    expect(parseActionResponse('context {"note":"villain folds a lot"} {"action":"bet","amount":5}')).toEqual({
      action: "bet",
      amount: 5,
    });
  });

  it("rejects malformed, missing, or unsupported actions", () => {
    expect(parseActionResponse("not json")).toBeUndefined();
    expect(parseActionResponse("just some prose with no object at all")).toBeUndefined();
    expect(parseActionResponse('{"amount":2}')).toBeUndefined();
    expect(parseActionResponse('{"action":"dance"}')).toBeUndefined();
    expect(parseActionResponse('{"action":"raise"}')).toBeUndefined();
  });

  it("rejects extra keys and invalid amount shapes", () => {
    expect(parseActionResponse('{"action":"fold","foo":"bar"}')).toBeUndefined();
    expect(parseActionResponse('{"action":"check","amount":0}')).toBeUndefined();
    expect(parseActionResponse('{"action":"call","amount":2.5}')).toBeUndefined();
  });
});

describe("validateOrFallback", () => {
  it("keeps a legal action", () => {
    expect(
      validateOrFallback(
        { action: "raise", amount: 12 },
        [
          { action: "fold" },
          { action: "raise", min: 10, max: 20 },
        ],
      ),
    ).toEqual({ action: "raise", amount: 12 });
  });

  it("requires the exact server-advertised call amount", () => {
    expect(
      validateOrFallback(
        { action: "call", amount: 3 },
        [
          { action: "fold" },
          { action: "call", amount: 2 },
        ],
      ),
    ).toEqual({ action: "fold" });
  });

  it("accepts inclusive raise bounds and rejects out-of-range raises", () => {
    expect(validateOrFallback({ action: "raise", amount: 10 }, [{ action: "raise", min: 10, max: 20 }])).toEqual({
      action: "raise",
      amount: 10,
    });
    expect(validateOrFallback({ action: "raise", amount: 20 }, [{ action: "raise", min: 10, max: 20 }])).toEqual({
      action: "raise",
      amount: 20,
    });
    expect(
      validateOrFallback(
        { action: "raise", amount: 21 },
        [
          { action: "fold" },
          { action: "raise", min: 10, max: 20 },
        ],
      ),
    ).toEqual({ action: "fold" });
  });

  it("falls back to check before fold or call", () => {
    expect(
      validateOrFallback(
        { action: "raise", amount: 100 },
        [
          { action: "check" },
          { action: "fold" },
          { action: "call", amount: 2 },
        ],
      ),
    ).toEqual({ action: "check" });
  });

  it("falls back to fold before call when checking is unavailable", () => {
    expect(
      validateOrFallback(
        { action: "raise", amount: 100 },
        [
          { action: "fold" },
          { action: "call", amount: 2 },
        ],
      ),
    ).toEqual({ action: "fold" });
  });

  it("falls back to the exact call amount only when no safer option exists", () => {
    expect(validateOrFallback(undefined, [{ action: "call", amount: 7 }])).toEqual({ action: "call", amount: 7 });
  });
});
