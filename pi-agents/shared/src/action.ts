// Model response parsing and server-legal action validation.

import type { ActionPayload, LegalActionOption } from "./protocol";

export function parseActionResponse(text: string): ActionPayload | undefined {
  const trimmed = text.trim();
  const match = trimmed.match(/\{[\s\S]*\}/);
  if (!match) return undefined;

  try {
    const parsed = JSON.parse(match[0]) as ActionPayload;
    if (typeof parsed.action !== "string") return undefined;
    return parsed;
  } catch {
    return undefined;
  }
}

export function validateOrFallback(action: ActionPayload | undefined, legalActions: LegalActionOption[]): ActionPayload {
  if (action && isLegal(action, legalActions)) return action;

  const check = legalActions.find((option) => option.action === "check");
  if (check) return { action: "check" };

  const fold = legalActions.find((option) => option.action === "fold");
  if (fold) return { action: "fold" };

  const call = legalActions.find((option) => option.action === "call" && typeof option.amount === "number");
  if (call) return { action: "call", amount: call.amount };

  throw new Error("no safe fallback action available");
}

function isLegal(action: ActionPayload, legalActions: LegalActionOption[]): boolean {
  const option = legalActions.find((candidate) => candidate.action === action.action);
  if (!option) return false;

  if (action.action === "call") return action.amount === option.amount;
  if (action.action === "bet" || action.action === "raise") {
    return typeof action.amount === "number" && typeof option.min === "number" && typeof option.max === "number" && action.amount >= option.min && action.amount <= option.max;
  }
  return action.amount === undefined;
}
