import { mkdir } from "node:fs/promises";
import path from "node:path";

import {
  type AgentSessionEvent,
  AuthStorage,
  createAgentSession,
  DefaultResourceLoader,
  getAgentDir,
  ModelRegistry,
  SessionManager,
  SettingsManager,
  type CreateAgentSessionOptions,
} from "@earendil-works/pi-coding-agent";

import { parseActionResponse } from "./action.js";
import type { ActionPayload, LegalActionOption } from "./protocol.js";
import type { DecisionClient } from "./strategy.js";

const STATELESS_SYSTEM_PROMPT = [
  "You are a poker decision engine for heads-up no-limit Texas Hold'em.",
  "Choose exactly one legal action from the user-provided legal_actions list.",
  'Return JSON only with shape {"action": string, "amount"?: number}.',
  "Do not add commentary, markdown, code fences, or extra keys.",
  "If raising or betting, choose an integer chip amount within the server-provided legal range.",
].join("\n");

const THINKING_LEVELS = ["off", "minimal", "low", "medium", "high", "xhigh"] as const;

type PiThinkingLevel = NonNullable<CreateAgentSessionOptions["thinkingLevel"]>;
type PiSession = {
  prompt(text: string): Promise<void>;
  subscribe(listener: (event: AgentSessionEvent) => void): () => void;
  getLastAssistantText(): string | undefined;
  exportToJsonl(outputPath?: string): string;
  dispose(): void;
};

type PiSessionFactory = (options: ResolvedPiSessionOptions) => Promise<PiSession>;

interface ResolvedPiSessionOptions {
  cwd: string;
  agentDir: string;
  sessionDir?: string;
  model?: string;
  thinkingLevel?: PiThinkingLevel;
}

export interface PiDecisionClientOptions {
  cwd: string;
  agentDir?: string;
  sessionDir?: string;
  model?: string;
  thinkingLevel?: PiThinkingLevel;
  sessionFactory?: PiSessionFactory;
}

export class PiDecisionClient implements DecisionClient {
  private readonly sessionFactory: PiSessionFactory;
  private decisionCount = 0;

  constructor(private readonly options: PiDecisionClientOptions) {
    this.sessionFactory = options.sessionFactory ?? createStatelessPiSession;
  }

  async decide(prompt: string, _legalActions: LegalActionOption[]): Promise<ActionPayload> {
    let session: PiSession | undefined;
    let unsubscribe = () => {};
    let streamedAssistantText = "";

    try {
      session = await this.sessionFactory({
        cwd: this.options.cwd,
        agentDir: this.options.agentDir ?? getAgentDir(),
        sessionDir: this.options.sessionDir,
        model: this.options.model,
        thinkingLevel: this.options.thinkingLevel,
      });

      unsubscribe = session.subscribe((event) => {
        if (event.type === "message_update" && event.assistantMessageEvent.type === "text_delta") {
          streamedAssistantText += event.assistantMessageEvent.delta;
        }
      });

      await session.prompt(prompt);

      const assistantText = session.getLastAssistantText() ?? streamedAssistantText;
      const action = parseActionResponse(assistantText);
      if (!action) {
        throw new Error(`assistant returned malformed action JSON: ${JSON.stringify(assistantText)}`);
      }

      return action;
    } catch (error) {
      throw new Error(`pi decision failed: ${error instanceof Error ? error.message : String(error)}`);
    } finally {
      unsubscribe();
      if (session) {
        await persistSessionLog(session, this.options.sessionDir, ++this.decisionCount);
        session.dispose();
      }
    }
  }
}

async function createStatelessPiSession(options: ResolvedPiSessionOptions): Promise<PiSession> {
  const authStorage = AuthStorage.create();
  const modelRegistry = ModelRegistry.create(authStorage);
  const settingsManager = SettingsManager.create(options.cwd, options.agentDir);
  settingsManager.applyOverrides({
    compaction: { enabled: false },
    retry: { enabled: false },
  });

  const resourceLoader = new DefaultResourceLoader({
    cwd: options.cwd,
    agentDir: options.agentDir,
    noExtensions: true,
    noSkills: true,
    noPromptTemplates: true,
    noThemes: true,
    noContextFiles: true,
    systemPromptOverride: () => STATELESS_SYSTEM_PROMPT,
    appendSystemPromptOverride: () => [],
  });
  await resourceLoader.reload();

  const resolvedModel = resolveModel(options.model, modelRegistry);
  const { session } = await createAgentSession({
    cwd: options.cwd,
    agentDir: options.agentDir,
    authStorage,
    modelRegistry,
    model: resolvedModel,
    thinkingLevel: options.thinkingLevel,
    resourceLoader,
    sessionManager: SessionManager.inMemory(options.cwd),
    settingsManager,
    tools: [],
  });

  return session;
}

function resolveModel(spec: string | undefined, modelRegistry: ModelRegistry): CreateAgentSessionOptions["model"] {
  if (!spec) return undefined;

  const colonIndex = spec.indexOf(":");
  const slashIndex = spec.indexOf("/");
  const delimiterIndex = [colonIndex, slashIndex].filter((index) => index > 0).sort((left, right) => left - right)[0] ?? -1;
  if (delimiterIndex > 0) {
    const provider = spec.slice(0, delimiterIndex);
    const modelID = spec.slice(delimiterIndex + 1);
    const model = modelRegistry.find(provider, modelID);
    if (!model) {
      throw new Error(`unknown Pi model ${JSON.stringify(spec)}`);
    }
    return model;
  }

  const matches = modelRegistry.getAll().filter((model) => model.id === spec);
  if (matches.length === 1) {
    return matches[0];
  }
  if (matches.length > 1) {
    throw new Error(`ambiguous Pi model ${JSON.stringify(spec)}; use provider:model syntax`);
  }

  throw new Error(`unknown Pi model ${JSON.stringify(spec)}`);
}

async function persistSessionLog(session: PiSession, sessionDir: string | undefined, decisionCount: number): Promise<void> {
  if (!sessionDir) return;

  await mkdir(sessionDir, { recursive: true });
  const outputPath = path.join(sessionDir, `pi-session-${String(decisionCount).padStart(4, "0")}.jsonl`);
  session.exportToJsonl(outputPath);
}

export function parsePiThinkingLevel(value: string | undefined): PiThinkingLevel | undefined {
  if (value === undefined) return undefined;
  if ((THINKING_LEVELS as readonly string[]).includes(value)) {
    return value as PiThinkingLevel;
  }
  throw new Error(`invalid Pi thinking level ${JSON.stringify(value)}`);
}
