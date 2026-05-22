// Shared poker-agent runner. This owns stdin/stdout JSONL, state updates,
// prompt construction, decision parsing, and hand-end hooks.

import { createInterface } from "node:readline";

import { validateOrFallback } from "./action.js";
import { buildDecisionPrompt } from "./prompt.js";
import {
  decodeEnvelope,
  encodeEnvelope,
  type ActionPayload,
  type Envelope,
  type HandEndPayload,
  type HandStartPayload,
  type SessionInitPayload,
  type YourTurnPayload,
} from "./protocol.js";
import { applyHandStart, applySessionInit, createAgentState, resetHandState } from "./state.js";
import type { DecisionClient, MemoryStrategy } from "./strategy.js";

export interface RunPokerAgentOptions {
  strategy: MemoryStrategy;
  decisionClient: DecisionClient;
  stdin?: NodeJS.ReadableStream;
  stdout?: NodeJS.WritableStream;
  stderr?: NodeJS.WritableStream;
}

export async function runPokerAgent(options: RunPokerAgentOptions): Promise<void> {
  const stdin = options.stdin ?? process.stdin;
  const stdout = options.stdout ?? process.stdout;
  const stderr = options.stderr ?? process.stderr;
  const state = createAgentState();
  let nextMessageID = 1;

  const reader = createInterface({ input: stdin, crlfDelay: Infinity });

  try {
    for await (const line of reader) {
      if (!line.trim()) continue;

      const envelope = decodeEnvelope(line);

      switch (envelope.type) {
        case "session_init": {
          const payload = envelope.payload as SessionInitPayload;
          applySessionInit(state, payload);
          await writeEnvelope(stdout, {
            v: 1,
            type: "session_ready",
            id: `agent-${nextMessageID++}`,
            in_reply_to: envelope.id,
            payload: { version: options.strategy.version },
          });
          break;
        }
        case "hand_start": {
          applyHandStart(state, envelope.payload as HandStartPayload);
          break;
        }
        case "your_turn": {
          const payload = envelope.payload as YourTurnPayload;
          const augmentation = await options.strategy.beforeDecision({
            state,
            handNumber: payload.hand_number,
            street: payload.street,
            board: payload.board,
            pot: payload.pot,
            toCall: payload.to_call,
            stacks: payload.stacks,
            actionHistory: payload.action_history,
            legalActions: payload.legal_actions,
          });
          const prompt = buildDecisionPrompt(
            {
              state,
              handNumber: payload.hand_number,
              street: payload.street,
              board: payload.board,
              pot: payload.pot,
              toCall: payload.to_call,
              stacks: payload.stacks,
              actionHistory: payload.action_history,
              legalActions: payload.legal_actions,
            },
            augmentation,
          );
          const proposedAction = await options.decisionClient.decide(prompt, payload.legal_actions);
          const action = validateOrFallback(proposedAction, payload.legal_actions);
          await writeEnvelope(stdout, {
            v: 1,
            type: "action",
            id: `agent-${nextMessageID++}`,
            in_reply_to: envelope.id,
            payload: action,
          });
          break;
        }
        case "hand_end": {
          const payload = envelope.payload as HandEndPayload;
          await options.strategy.afterHandEnd({
            state,
            handNumber: payload.hand_number,
            board: payload.board,
            showdown: payload.showdown,
            result: payload.result,
          });
          resetHandState(state);
          break;
        }
        case "session_end":
          return;
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

async function writeEnvelope(stream: NodeJS.WritableStream, envelope: Envelope<ActionPayload | { version: string }>): Promise<void> {
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
