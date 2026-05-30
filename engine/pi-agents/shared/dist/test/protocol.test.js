import { describe, expect, it } from "vitest";
import { decodeEnvelope, encodeEnvelope } from "../src/protocol.js";
describe("protocol helpers", () => {
    it.each([
        {
            name: "session_init",
            message: {
                v: 1,
                type: "session_init",
                id: "msg-1",
                payload: {
                    session_id: "ses-1",
                    agent_name: "llm-akg-recent",
                    match: {
                        match_id: "mat-1",
                        seed: 12345,
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
            },
        },
        {
            name: "session_ready",
            message: {
                v: 1,
                type: "session_ready",
                id: "msg-2",
                in_reply_to: "msg-1",
                payload: { version: "llm-stateless/0.1.0" },
            },
        },
        {
            name: "session_init without memory_dir",
            message: {
                v: 1,
                type: "session_init",
                id: "msg-1b",
                payload: {
                    session_id: "ses-2",
                    agent_name: "llm-stateless",
                    match: {
                        match_id: "mat-2",
                        seed: 99,
                        hand_count: 50,
                        variant: "heads-up-nlhe",
                        info_realism: "perfect-info",
                        starting_stack: 100,
                        blinds: { sb: 1, bb: 2 },
                        decision_deadline_ms: 15000,
                    },
                    seats: [{ seat: 0, name: "hero" }],
                    your_seat: 0,
                },
            },
        },
        {
            name: "hand_start",
            message: {
                v: 1,
                type: "hand_start",
                id: "msg-3",
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
            },
        },
        {
            name: "your_turn",
            message: {
                v: 1,
                type: "your_turn",
                id: "msg-4",
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
                    action_history: [
                        { seat: 1, action: "call", amount: 1, street: "preflop" },
                        { seat: 0, action: "check", street: "preflop" },
                        { seat: 0, action: "bet", amount: 2, street: "flop" },
                    ],
                    legal_actions: [
                        { action: "fold" },
                        { action: "call", amount: 2 },
                        { action: "raise", min: 4, max: 197 },
                    ],
                },
            },
        },
        {
            name: "action",
            message: {
                v: 1,
                type: "action",
                id: "msg-5",
                in_reply_to: "msg-4",
                payload: { action: "call", amount: 2 },
            },
        },
        {
            name: "hand_end",
            message: {
                v: 1,
                type: "hand_end",
                id: "msg-6",
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
                        { seat: 1, chips_delta: 14 },
                        { seat: 0, chips_delta: -14 },
                    ],
                },
            },
        },
        {
            name: "session_end",
            message: {
                v: 1,
                type: "session_end",
                id: "msg-7",
                payload: {},
            },
        },
        {
            name: "log",
            message: {
                v: 1,
                type: "log",
                id: "msg-8",
                payload: {
                    level: "info",
                    message: "raised turn blocker candidate",
                    fields: { hand_number: 47, street: "turn" },
                },
            },
        },
    ])("round-trips $name as one JSONL line", ({ message }) => {
        const encoded = encodeEnvelope(message);
        expect(encoded.endsWith("\n")).toBe(true);
        expect(encoded.slice(0, -1)).not.toContain("\n");
        expect(decodeEnvelope(encoded)).toEqual(message);
    });
    it.each([
        {
            name: "invalid json",
            input: '{"v":1,',
            want: "decode envelope",
        },
        {
            name: "unsupported protocol version",
            input: '{"v":2,"type":"session_init","id":"msg-1","payload":{}}',
            want: "unsupported protocol version",
        },
        {
            name: "missing payload",
            input: '{"v":1,"type":"session_init","id":"msg-1"}',
            want: "missing payload",
        },
        {
            name: "missing required reply correlation",
            input: '{"v":1,"type":"action","id":"msg-1","payload":{"action":"fold"}}',
            want: "requires in_reply_to",
        },
        {
            name: "unsupported message type",
            input: '{"v":1,"type":"bogus","id":"msg-1","payload":{}}',
            want: "unsupported message type",
        },
        {
            name: "session_end payload must be an object",
            input: '{"v":1,"type":"session_end","id":"msg-1","payload":[]}',
            want: "decode session_end payload",
        },
        {
            name: "your_turn payload rejects malformed street",
            input: '{"v":1,"type":"your_turn","id":"msg-1","payload":{"hand_number":7,"street":[],"board":[],"pot":1,"to_call":0,"stacks":{},"seats":[],"action_history":[],"legal_actions":[]}}',
            want: "decode your_turn payload.street",
        },
        {
            name: "hand_end payload requires final action history and showdown flag",
            input: '{"v":1,"type":"hand_end","id":"msg-1","payload":{"hand_number":7,"board":[],"showdown":{},"result":[]}}',
            want: "decode hand_end payload.action_history",
        },
        {
            name: "multiple json objects in one input are rejected",
            input: '{"v":1,"type":"session_end","id":"msg-1","payload":{}}\n{"v":1,"type":"session_end","id":"msg-2","payload":{}}',
            want: "exactly one JSON object per line",
        },
    ])("rejects $name", ({ input, want }) => {
        expect(() => decodeEnvelope(input)).toThrowError(want);
    });
});
