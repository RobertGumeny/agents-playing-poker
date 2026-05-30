// Shared poker wire protocol types and JSONL helpers for Pi agents.
// This package intentionally mirrors docs/wire-protocol.md without importing Go internals.
export const PROTOCOL_VERSION = 1;
export const MESSAGE_TYPES = [
    "session_init",
    "session_ready",
    "hand_start",
    "your_turn",
    "action",
    "hand_end",
    "session_end",
    "log",
];
export function encodeEnvelope(envelope) {
    validateEnvelope(envelope);
    return `${JSON.stringify(envelope)}\n`;
}
export function decodeEnvelope(line) {
    const normalizedLine = normalizeSingleJSONLine(line);
    let parsed;
    try {
        parsed = JSON.parse(normalizedLine);
    }
    catch (error) {
        throw new Error(`decode envelope: ${error instanceof Error ? error.message : String(error)}`);
    }
    validateEnvelope(parsed);
    return parsed;
}
function normalizeSingleJSONLine(line) {
    if (line.length === 0) {
        throw new Error("decode envelope: empty line");
    }
    let normalized = line;
    if (normalized.endsWith("\n")) {
        normalized = normalized.slice(0, -1);
    }
    if (normalized.endsWith("\r")) {
        normalized = normalized.slice(0, -1);
    }
    if (normalized.length === 0) {
        throw new Error("decode envelope: empty line");
    }
    if (normalized.includes("\n") || normalized.includes("\r")) {
        throw new Error("decode envelope: expected exactly one JSON object per line");
    }
    return normalized;
}
function validateEnvelope(value) {
    const envelope = expectRecord(value, "envelope");
    if (envelope.v !== PROTOCOL_VERSION) {
        throw new Error(`validate envelope: unsupported protocol version ${String(envelope.v)}`);
    }
    if (!isMessageType(envelope.type)) {
        throw new Error(`validate envelope: unsupported message type ${JSON.stringify(envelope.type)}`);
    }
    if (!isNonEmptyString(envelope.id)) {
        throw new Error("validate envelope: missing id");
    }
    if (!Object.hasOwn(envelope, "payload")) {
        throw new Error("validate envelope: missing payload");
    }
    if (Object.hasOwn(envelope, "in_reply_to") && envelope.in_reply_to !== undefined && typeof envelope.in_reply_to !== "string") {
        throw new Error("validate envelope: in_reply_to must be a string");
    }
    if (requiresReplyCorrelation(envelope.type) && !isNonEmptyString(envelope.in_reply_to)) {
        throw new Error(`validate envelope: message type ${JSON.stringify(envelope.type)} requires in_reply_to`);
    }
    validatePayload(envelope.type, envelope.payload);
}
function validatePayload(type, payload) {
    switch (type) {
        case "session_init":
            validateSessionInitPayload(payload);
            return;
        case "session_ready":
            validateSessionReadyPayload(payload);
            return;
        case "hand_start":
            validateHandStartPayload(payload);
            return;
        case "your_turn":
            validateYourTurnPayload(payload);
            return;
        case "action":
            validateActionPayload(payload);
            return;
        case "hand_end":
            validateHandEndPayload(payload);
            return;
        case "session_end":
            validateSessionEndPayload(payload);
            return;
        case "log":
            validateLogPayload(payload);
            return;
    }
}
function validateSessionInitPayload(value) {
    const payload = expectRecord(value, "session_init payload");
    expectString(payload.session_id, "session_init payload.session_id");
    expectString(payload.agent_name, "session_init payload.agent_name");
    validateMatchConfig(payload.match);
    validateSeatDescriptors(payload.seats, "session_init payload.seats");
    expectNumber(payload.your_seat, "session_init payload.your_seat");
    if (payload.memory_dir !== undefined) {
        expectString(payload.memory_dir, "session_init payload.memory_dir");
    }
}
function validateMatchConfig(value) {
    const match = expectRecord(value, "session_init payload.match");
    expectString(match.match_id, "session_init payload.match.match_id");
    expectNumber(match.seed, "session_init payload.match.seed");
    expectNumber(match.hand_count, "session_init payload.match.hand_count");
    expectString(match.variant, "session_init payload.match.variant");
    expectString(match.info_realism, "session_init payload.match.info_realism");
    expectNumber(match.starting_stack, "session_init payload.match.starting_stack");
    const blinds = expectRecord(match.blinds, "session_init payload.match.blinds");
    expectNumber(blinds.sb, "session_init payload.match.blinds.sb");
    expectNumber(blinds.bb, "session_init payload.match.blinds.bb");
    expectNumber(match.decision_deadline_ms, "session_init payload.match.decision_deadline_ms");
}
function validateHandStartPayload(value) {
    const payload = expectRecord(value, "hand_start payload");
    expectNumber(payload.hand_number, "hand_start payload.hand_number");
    expectNumber(payload.dealer_seat, "hand_start payload.dealer_seat");
    validateChipMap(payload.stacks, "hand_start payload.stacks");
    validateBlindAmounts(payload.blinds_posted, "hand_start payload.blinds_posted");
    validateStringArray(payload.your_hole_cards, "hand_start payload.your_hole_cards");
}
function validateYourTurnPayload(value) {
    const payload = expectRecord(value, "your_turn payload");
    expectNumber(payload.hand_number, "your_turn payload.hand_number");
    expectStreet(payload.street, "your_turn payload.street");
    validateStringArray(payload.board, "your_turn payload.board");
    expectNumber(payload.pot, "your_turn payload.pot");
    expectNumber(payload.to_call, "your_turn payload.to_call");
    validateChipMap(payload.stacks, "your_turn payload.stacks");
    validateSeatDescriptors(payload.seats, "your_turn payload.seats");
    validateActionHistory(payload.action_history, "your_turn payload.action_history");
    validateLegalActions(payload.legal_actions, "your_turn payload.legal_actions");
}
function validateHandEndPayload(value) {
    const payload = expectRecord(value, "hand_end payload");
    expectNumber(payload.hand_number, "hand_end payload.hand_number");
    validateStringArray(payload.board, "hand_end payload.board");
    validateActionHistory(payload.action_history, "hand_end payload.action_history");
    expectBoolean(payload.showdown_reached, "hand_end payload.showdown_reached");
    if (payload.showdown !== undefined) {
        validateShowdown(payload.showdown, "hand_end payload.showdown");
    }
    validateHandResults(payload.result, "hand_end payload.result");
}
function validateSessionEndPayload(value) {
    expectRecord(value, "session_end payload");
}
function validateSessionReadyPayload(value) {
    const payload = expectRecord(value, "session_ready payload");
    expectString(payload.version, "session_ready payload.version");
}
function validateActionPayload(value) {
    const payload = expectRecord(value, "action payload");
    expectDecisionAction(payload.action, "action payload.action");
    if (payload.amount !== undefined) {
        expectNumber(payload.amount, "action payload.amount");
    }
}
function validateLogPayload(value) {
    const payload = expectRecord(value, "log payload");
    expectString(payload.level, "log payload.level");
    expectString(payload.message, "log payload.message");
    if (payload.fields !== undefined) {
        expectRecord(payload.fields, "log payload.fields");
    }
}
function validateSeatDescriptors(value, path) {
    const seats = expectArray(value, path);
    for (const [index, seat] of seats.entries()) {
        const item = expectRecord(seat, `${path}[${index}]`);
        expectNumber(item.seat, `${path}[${index}].seat`);
        expectString(item.name, `${path}[${index}].name`);
    }
}
function validateBlindAmounts(value, path) {
    const blinds = expectArray(value, path);
    for (const [index, blind] of blinds.entries()) {
        const item = expectRecord(blind, `${path}[${index}]`);
        expectNumber(item.seat, `${path}[${index}].seat`);
        expectNumber(item.amount, `${path}[${index}].amount`);
    }
}
function validateActionHistory(value, path) {
    const history = expectArray(value, path);
    for (const [index, entry] of history.entries()) {
        const item = expectRecord(entry, `${path}[${index}]`);
        expectNumber(item.seat, `${path}[${index}].seat`);
        expectPlayerAction(item.action, `${path}[${index}].action`);
        if (item.amount !== undefined) {
            expectNumber(item.amount, `${path}[${index}].amount`);
        }
        expectStreet(item.street, `${path}[${index}].street`);
        if (item.forced_reason !== undefined) {
            expectString(item.forced_reason, `${path}[${index}].forced_reason`);
        }
    }
}
function validateLegalActions(value, path) {
    const actions = expectArray(value, path);
    for (const [index, action] of actions.entries()) {
        const item = expectRecord(action, `${path}[${index}]`);
        expectDecisionAction(item.action, `${path}[${index}].action`);
        if (item.amount !== undefined) {
            expectNumber(item.amount, `${path}[${index}].amount`);
        }
        if (item.min !== undefined) {
            expectNumber(item.min, `${path}[${index}].min`);
        }
        if (item.max !== undefined) {
            expectNumber(item.max, `${path}[${index}].max`);
        }
    }
}
function validateShowdown(value, path) {
    const showdown = expectRecord(value, path);
    for (const [seat, entry] of Object.entries(showdown)) {
        const item = expectRecord(entry, `${path}.${seat}`);
        validateStringArray(item.hole_cards, `${path}.${seat}.hole_cards`);
        expectString(item.rank, `${path}.${seat}.rank`);
    }
}
function validateHandResults(value, path) {
    const results = expectArray(value, path);
    for (const [index, result] of results.entries()) {
        const item = expectRecord(result, `${path}[${index}]`);
        expectNumber(item.seat, `${path}[${index}].seat`);
        expectNumber(item.chips_delta, `${path}[${index}].chips_delta`);
    }
}
function validateChipMap(value, path) {
    const chips = expectRecord(value, path);
    for (const [seat, amount] of Object.entries(chips)) {
        expectNumber(amount, `${path}.${seat}`);
    }
}
function validateStringArray(value, path) {
    const items = expectArray(value, path);
    for (const [index, item] of items.entries()) {
        expectString(item, `${path}[${index}]`);
    }
}
function expectRecord(value, path) {
    if (!isRecord(value)) {
        throw new Error(`decode ${path}: expected object`);
    }
    return value;
}
function expectArray(value, path) {
    if (!Array.isArray(value)) {
        throw new Error(`decode ${path}: expected array`);
    }
    return value;
}
function expectString(value, path) {
    if (typeof value !== "string") {
        throw new Error(`decode ${path}: expected string`);
    }
    return value;
}
function expectNumber(value, path) {
    if (typeof value !== "number" || Number.isNaN(value)) {
        throw new Error(`decode ${path}: expected number`);
    }
    return value;
}
function expectBoolean(value, path) {
    if (typeof value !== "boolean") {
        throw new Error(`decode ${path}: expected boolean`);
    }
    return value;
}
function expectStreet(value, path) {
    if (value !== "preflop" && value !== "flop" && value !== "turn" && value !== "river") {
        throw new Error(`decode ${path}: expected street`);
    }
    return value;
}
function expectDecisionAction(value, path) {
    if (value !== "fold" && value !== "check" && value !== "call" && value !== "bet" && value !== "raise") {
        throw new Error(`decode ${path}: expected decision action`);
    }
    return value;
}
function expectPlayerAction(value, path) {
    if (value !== "post_blind" &&
        value !== "fold" &&
        value !== "check" &&
        value !== "call" &&
        value !== "bet" &&
        value !== "raise" &&
        value !== "auto_check" &&
        value !== "auto_fold") {
        throw new Error(`decode ${path}: expected player action`);
    }
    return value;
}
function requiresReplyCorrelation(type) {
    return type === "session_ready" || type === "action";
}
function isMessageType(value) {
    return typeof value === "string" && MESSAGE_TYPES.includes(value);
}
function isRecord(value) {
    return typeof value === "object" && value !== null && !Array.isArray(value);
}
function isNonEmptyString(value) {
    return typeof value === "string" && value.length > 0;
}
