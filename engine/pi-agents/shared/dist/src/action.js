// Model response parsing and server-legal action validation.
const DECISION_ACTIONS = ["fold", "check", "call", "bet", "raise"];
export function parseActionResponse(text) {
    const trimmed = text.trim();
    if (!trimmed)
        return undefined;
    // Happy path: the whole message is the action JSON, per the prompt contract.
    const direct = tryParseActionObject(trimmed);
    if (direct)
        return direct;
    // Tolerant path: models intermittently leak reasoning prose or wrap the action
    // in code fences before the JSON. Rather than forcing a full extra decision
    // round-trip (the retry), recover the trailing action object from the message.
    // The final balanced object that parses to a legal action wins.
    const candidates = extractBalancedObjects(trimmed);
    for (let index = candidates.length - 1; index >= 0; index -= 1) {
        const action = tryParseActionObject(candidates[index]);
        if (action)
            return action;
    }
    return undefined;
}
function tryParseActionObject(candidate) {
    let parsed;
    try {
        parsed = JSON.parse(candidate);
    }
    catch {
        return undefined;
    }
    return parseActionPayload(parsed);
}
// Every top-level {...} substring, scanned with string/escape awareness so braces
// inside JSON string values never desync the brace matching.
function extractBalancedObjects(text) {
    const objects = [];
    let depth = 0;
    let start = -1;
    let inString = false;
    let escaped = false;
    for (let index = 0; index < text.length; index += 1) {
        const char = text[index];
        if (inString) {
            if (escaped)
                escaped = false;
            else if (char === "\\")
                escaped = true;
            else if (char === '"')
                inString = false;
            continue;
        }
        if (char === '"') {
            inString = true;
        }
        else if (char === "{") {
            if (depth === 0)
                start = index;
            depth += 1;
        }
        else if (char === "}" && depth > 0) {
            depth -= 1;
            if (depth === 0 && start >= 0) {
                objects.push(text.slice(start, index + 1));
                start = -1;
            }
        }
    }
    return objects;
}
export function validateOrFallback(action, legalActions) {
    if (action && isLegal(action, legalActions))
        return action;
    const check = legalActions.find((option) => option.action === "check");
    if (check)
        return { action: "check" };
    const fold = legalActions.find((option) => option.action === "fold");
    if (fold)
        return { action: "fold" };
    const call = legalActions.find((option) => option.action === "call" && Number.isInteger(option.amount));
    if (call?.amount !== undefined)
        return { action: "call", amount: call.amount };
    throw new Error("no safe fallback action available");
}
function parseActionPayload(value) {
    if (!isRecord(value))
        return undefined;
    const keys = Object.keys(value);
    if (keys.length === 0 || keys.some((key) => key !== "action" && key !== "amount")) {
        return undefined;
    }
    if (!isDecisionAction(value.action))
        return undefined;
    const amount = value.amount;
    switch (value.action) {
        case "fold":
        case "check":
            return amount === undefined ? { action: value.action } : undefined;
        case "call":
        case "bet":
        case "raise":
            return isIntegerChipAmount(amount) ? { action: value.action, amount } : undefined;
    }
}
function isLegal(action, legalActions) {
    switch (action.action) {
        case "fold":
        case "check":
            return action.amount === undefined && legalActions.some((candidate) => candidate.action === action.action);
        case "call":
            return (isIntegerChipAmount(action.amount) &&
                legalActions.some((candidate) => candidate.action === "call" && candidate.amount === action.amount));
        case "bet":
        case "raise": {
            const amount = action.amount;
            return (isIntegerChipAmount(amount) &&
                legalActions.some((candidate) => candidate.action === action.action &&
                    isIntegerChipAmount(candidate.min) &&
                    isIntegerChipAmount(candidate.max) &&
                    amount >= candidate.min &&
                    amount <= candidate.max));
        }
    }
}
function isDecisionAction(value) {
    return typeof value === "string" && DECISION_ACTIONS.includes(value);
}
function isIntegerChipAmount(value) {
    return typeof value === "number" && Number.isInteger(value);
}
function isRecord(value) {
    return typeof value === "object" && value !== null && !Array.isArray(value);
}
