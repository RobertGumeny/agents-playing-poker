import { describe, expect, it } from "vitest";

import { parseActionResponse, validateOrFallback } from "../src/action.js";

describe("parseActionResponse", () => {
  it("extracts a JSON action object from model text", () => {
    expect(parseActionResponse('answer: {"action":"call","amount":2}')).toEqual({
      action: "call",
      amount: 2,
    });
  });

  it("returns undefined for invalid JSON", () => {
    expect(parseActionResponse("not json")).toBeUndefined();
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
});
