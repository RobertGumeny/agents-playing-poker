// Shared poker-agent runner. This owns stdin/stdout JSONL, state updates,
// prompt construction, legality validation, retry budgeting, and lifecycle
// notifications into strategy-owned memory policy and decision engine seams.

import { createInterface } from "node:readline";

import { validateOrFallback } from "./action.js";
import { buildDecisionPrompt } from "./prompt.js";
import {
  PROTOCOL_VERSION,
  decodeEnvelope,
  encodeEnvelope,
  type ActionMessage,
  type HandEndMessage,
  type HandStartMessage,
  type SessionEndMessage,
  type SessionInitMessage,
  type SessionReadyMessage,
  type YourTurnMessage,
} from "./protocol.js";
import { applyHandStart, applySessionInit, applyYourTurn, createAgentState, resetHandState, resetSessionState } from "./state.js";
import type { CompletedHandContext, DecisionContext, DecisionEngine, HandStartContext, MemoryPolicy } from "./strategy.js";

const DEFAULT_DECISION_ATTEMPTS = 2;

export interface RunPokerAgentOptions {
  memoryPolicy: MemoryPolicy;
  decisionEngine: DecisionEngine;
  stdin?: NodeJS.ReadableStream;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
  maxDecisionAttempts?: number;
  agentVersion: string;
}

export async function runPokerAgent(options: RunPokerAgentOptions): Promise<void> {
  const stdin = options.stdin ?? process.stdin;
  const stdout = options.stdout ?? process.stdout;
  const stderr = options.stderr ?? process.stderr;
  const state = createAgentState();
  const maxDecisionAttempts = normalizeDecisionAttempts(options.maxDecisionAttempts);
  let nextMessageID = 1;

  const reader = createInterface({ input: stdin, crlfDelay: Infinity });

  try {
    for await (const line of reader) {
      if (!line.trim()) continue;

      const envelope = decodeEnvelope(line);

      switch (envelope.type) {
        case "session_init": {
          const message: SessionInitMessage = envelope;
          applySessionInit(state, message.payload);
          await writeEnvelope(stdout, {
            v: PROTOCOL_VERSION,
            type: "session_ready",
            id: `agent-${nextMessageID++}`,
            in_reply_to: message.id,
            payload: { version: options.agentVersion },
          });
          break;
        }
        case "hand_start": {
          const message: HandStartMessage = envelope;
          applyHandStart(state, message.payload);
          await options.decisionEngine.onHandStart?.(buildHandStartContext(state));
          break;
        }
        case "your_turn": {
          const message: YourTurnMessage = envelope;
          applyYourTurn(state, message.payload);
          const decisionContext = buildDecisionContext(state, message);
          const augmentation = await options.memoryPolicy.beforeDecision(decisionContext);
          const prompt = buildDecisionPrompt(decisionContext, augmentation);
          const proposedAction = await decideWithRetry(options.decisionEngine, decisionContext, prompt, maxDecisionAttempts, stderr);
          const action = validateOrFallback(proposedAction, message.payload.legal_actions);
          await writeEnvelope(stdout, {
            v: PROTOCOL_VERSION,
            type: "action",
            id: `agent-${nextMessageID++}`,
            in_reply_to: message.id,
            payload: action,
          });
          break;
        }
        case "hand_end": {
          const message: HandEndMessage = envelope;
          const completedHand = buildCompletedHandContext(state, message);
          await options.memoryPolicy.afterHandEnd(completedHand);
          await options.decisionEngine.onHandEnd?.(completedHand);
          resetHandState(state);
          break;
        }
        case "session_end": {
          const _message: SessionEndMessage = envelope;
          await options.decisionEngine.onSessionEnd?.();
          resetSessionState(state);
          return;
        }
        default:
          break;
      }
    }
  } catch (error) {
    stderr.write(`${error instanceof Error ? error.stack ?? error.message : String(error)}\n`);
    throw error;
  } finally {
    reader.close();
  }
}

function buildHandStartContext(state: ReturnType<typeof createAgentState>): HandStartContext {
  const hand = state.hand;
  if (!hand) {
    throw new Error("hand_start received without shared hand state");
  }

  return {
    state,
    handNumber: hand.handNumber,
    dealerSeat: hand.dealerSeat,
    stacks: { ...hand.stacks },
    blindsPosted: hand.blindsPosted.map((blind) => ({ ...blind })),
    yourHoleCards: [...hand.yourHoleCards],
  };
}

function buildDecisionContext(state: ReturnType<typeof createAgentState>, message: YourTurnMessage): DecisionContext {
  return {
    state,
    handNumber: message.payload.hand_number,
    street: message.payload.street,
    board: [...message.payload.board],
    pot: message.payload.pot,
    toCall: message.payload.to_call,
    stacks: { ...message.payload.stacks },
    actionHistory: message.payload.action_history.map((entry) => ({ ...entry })),
    legalActions: message.payload.legal_actions.map((action) => ({ ...action })),
  };
}

function buildCompletedHandContext(state: ReturnType<typeof createAgentState>, message: HandEndMessage): CompletedHandContext {
  const hand = state.hand;
  const session = state.session;
  if (!hand || !session) {
    throw new Error("hand_end received without shared session/hand state");
  }

  return {
    state,
    handNumber: message.payload.hand_number,
    dealerSeat: hand.dealerSeat,
    heroSeat: session.yourSeat,
    seats: session.seats.map((seat) => ({ ...seat })),
    heroHoleCards: [...hand.yourHoleCards],
    board: [...message.payload.board],
    actionHistory: message.payload.action_history.map((entry) => ({ ...entry })),
    showdownReached: message.payload.showdown_reached,
    showdown: message.payload.showdown ? Object.fromEntries(Object.entries(message.payload.showdown).map(([seat, entry]) => [seat, { ...entry, hole_cards: [...entry.hole_cards] }])) : undefined,
    result: message.payload.result.map((entry) => ({ ...entry })),
  };
}

async function decideWithRetry(
  decisionEngine: DecisionEngine,
  context: DecisionContext,
  prompt: string,
  maxDecisionAttempts: number,
  stderr: NodeJS.WritableStream,
) {
  let lastError: unknown;

  for (let attempt = 1; attempt <= maxDecisionAttempts; attempt += 1) {
    try {
      return await decisionEngine.decide({
        context,
        prompt,
        legalActions: context.legalActions.map((action) => ({ ...action })),
      });
    } catch (error) {
      lastError = error;
      stderr.write(
        `decision attempt ${attempt}/${maxDecisionAttempts} failed: ${error instanceof Error ? error.message : String(error)}\n`,
      );
    }
  }

  if (lastError !== undefined) {
    stderr.write("decision engine exhausted retries; using safe fallback action\n");
  }
  return undefined;
}

function normalizeDecisionAttempts(value: number | undefined): number {
  if (value === undefined) return DEFAULT_DECISION_ATTEMPTS;
  if (!Number.isInteger(value) || value <= 0) {
    throw new Error("maxDecisionAttempts must be a positive integer");
  }
  return value;
}

async function writeEnvelope(stream: NodeJS.WritableStream, envelope: SessionReadyMessage | ActionMessage): Promise<void> {
  const line = encodeEnvelope(envelope);
  await new Promise<void>((resolve, reject) => {
    stream.write(line, (error) => {
      if (error) {
        reject(error);
        return;
      }
      resolve();
    });
  });
}
