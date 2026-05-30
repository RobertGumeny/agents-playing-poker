// Model response parsing and server-legal action validation.
const DECISION_ACTIONS = ["fold", "check", "call", "bet", "raise"];
export function parseActionResponse(text) {
    const trimmed = text.trim();
    if (!trimmed)
        return undefined;
    let parsed;
    try {
        parsed = JSON.parse(trimmed);
    }
    catch {
        return undefined;
    }
    return parseActionPayload(parsed);
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
