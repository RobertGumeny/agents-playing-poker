import type { ActionPayload, LegalActionOption } from "./protocol.js";
export declare function parseActionResponse(text: string): ActionPayload | undefined;
export declare function validateOrFallback(action: ActionPayload | undefined, legalActions: LegalActionOption[]): ActionPayload;
