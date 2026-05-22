import { describe, expect, it } from "vitest";

import { decodeEnvelope, encodeEnvelope, type Envelope } from "../src/protocol.js";

describe("protocol helpers", () => {
  it("round-trips an envelope as one JSONL line", () => {
    const envelope: Envelope<{ value: number }> = {
      v: 1,
      type: "session_end",
      id: "msg-1",
      payload: { value: 1 },
    };

    const encoded = encodeEnvelope(envelope);

    expect(encoded.endsWith("\n")).toBe(true);
    expect(decodeEnvelope(encoded)).toEqual(envelope);
  });
});
