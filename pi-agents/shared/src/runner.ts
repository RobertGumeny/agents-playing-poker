// Shared poker-agent runner. This owns stdin/stdout JSONL, state updates,
// prompt construction, decision parsing, and hand-end hooks.

import { createInterface } from "node:readline";

import { validateOrFallback } from "./action.js";
import { buildDecisionPrompt } from "./prompt.js";
import {
  decodeEnvelope,
  encodeEnvelope,
  type ActionMessage,
  type HandStartMessage,
  type SessionInitMessage,
  type SessionReadyMessage,
  type YourTurnMessage,
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
          const message: SessionInitMessage = envelope;
          applySessionInit(state, message.payload);
          await writeEnvelope(stdout, {
            v: 1,
            type: "session_ready",
            id: `agent-${nextMessageID++}`,
            in_reply_to: message.id,
            payload: { version: options.strategy.version },
          });
          break;
        }
        case "hand_start": {
          const message: HandStartMessage = envelope;
          applyHandStart(state, message.payload);
          break;
        }
        case "your_turn": {
          const message: YourTurnMessage = envelope;
          const augmentation = await options.strategy.beforeDecision({
            state,
            handNumber: message.payload.hand_number,
            street: message.payload.street,
            board: message.payload.board,
            pot: message.payload.pot,
            toCall: message.payload.to_call,
            stacks: message.payload.stacks,
            actionHistory: message.payload.action_history,
            legalActions: message.payload.legal_actions,
          });
          const prompt = buildDecisionPrompt(
            {
              state,
              handNumber: message.payload.hand_number,
              street: message.payload.street,
              board: message.payload.board,
              pot: message.payload.pot,
              toCall: message.payload.to_call,
              stacks: message.payload.stacks,
              actionHistory: message.payload.action_history,
              legalActions: message.payload.legal_actions,
            },
            augmentation,
          );
          const proposedAction = await options.decisionClient.decide(prompt, message.payload.legal_actions);
          const action = validateOrFallback(proposedAction, message.payload.legal_actions);
          await writeEnvelope(stdout, {
            v: 1,
            type: "action",
            id: `agent-${nextMessageID++}`,
            in_reply_to: message.id,
            payload: action,
          });
          break;
        }
        case "hand_end": {
          await options.strategy.afterHandEnd({
            state,
            handNumber: envelope.payload.hand_number,
            board: envelope.payload.board,
            showdown: envelope.payload.showdown,
            result: envelope.payload.result,
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
