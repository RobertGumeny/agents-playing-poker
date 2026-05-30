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
import {
  PiDecisionEngine,
  ScriptedDecisionEngine,
  parsePiThinkingLevel,
  type ActionPayload,
} from "@agent-poker/pi-agent-shared";

import type { AkgDurableMemoryPolicy } from "./memory.js";
import { createQueryTools } from "./tools.js";

export const DURABLE_SYSTEM_PROMPT = [
  "You are a poker decision engine for heads-up no-limit Texas Hold'em.",
  "You have access to AKG memory tools. The opponent node is your index: call akg_get_opponent first to read a full behavioral summary and discover what patterns have been identified.",
  "You may call akg_list_patterns, akg_get_pattern, akg_list_hands, or akg_get_hand as needed before your final answer.",
  "After your research, choose exactly one legal action from the user-provided legal_actions list.",
  'Your final response must be JSON only: {"action": string, "amount"?: number}.',
  "No commentary, markdown, code fences, or extra keys in the final JSON response.",
  "If raising or betting, use an integer chip amount within the server-provided legal range.",
].join("\n");

type PiSession = {
  prompt(text: string): Promise<void>;
  subscribe(listener: (event: AgentSessionEvent) => void): () => void;
  getLastAssistantText(): string | undefined;
  exportToJsonl(outputPath?: string): string;
  dispose(): void;
};

type PiSessionFactory = (options: {
  cwd: string;
  agentDir: string;
  sessionDir?: string;
  model?: string;
  thinkingLevel?: NonNullable<CreateAgentSessionOptions["thinkingLevel"]>;
}) => Promise<PiSession>;

type SessionFactoryDeps = {
  createAuthStorage: () => ReturnType<typeof AuthStorage.create>;
  createModelRegistry: (authStorage: ReturnType<typeof AuthStorage.create>) => ReturnType<typeof ModelRegistry.create>;
  createSettingsManager: (cwd: string, agentDir: string) => ReturnType<typeof SettingsManager.create>;
  createResourceLoader: (options: ConstructorParameters<typeof DefaultResourceLoader>[0]) => DefaultResourceLoader;
  createSessionManager: (cwd: string) => ReturnType<typeof SessionManager.inMemory>;
  createAgentSession: typeof createAgentSession;
};

const defaultSessionFactoryDeps: SessionFactoryDeps = {
  createAuthStorage: () => AuthStorage.create(),
  createModelRegistry: (authStorage) => ModelRegistry.create(authStorage),
  createSettingsManager: (cwd, agentDir) => SettingsManager.create(cwd, agentDir),
  createResourceLoader: (options) => new DefaultResourceLoader(options),
  createSessionManager: (cwd) => SessionManager.inMemory(cwd),
  createAgentSession,
};

export interface CreateDecisionEngineOptions {
  cwd?: string;
  sessionDir?: string;
  model?: string;
  thinkingLevel?: string;
  fakeDecisionsJSON?: string;
  sessionFactory?: PiSessionFactory;
}

export function createDurableSessionFactory(
  memoryPolicy: Pick<AkgDurableMemoryPolicy, "getStore">,
  deps: SessionFactoryDeps = defaultSessionFactoryDeps,
): PiSessionFactory {
  return async (options) => {
    const authStorage = deps.createAuthStorage();
    const modelRegistry = deps.createModelRegistry(authStorage);
    const settingsManager = deps.createSettingsManager(options.cwd, options.agentDir);
    settingsManager.applyOverrides({
      compaction: { enabled: false },
      retry: { enabled: false },
    });

    const resourceLoader = deps.createResourceLoader({
      cwd: options.cwd,
      agentDir: options.agentDir,
      noExtensions: true,
      noSkills: true,
      noPromptTemplates: true,
      noThemes: true,
      noContextFiles: true,
      systemPromptOverride: () => DURABLE_SYSTEM_PROMPT,
      appendSystemPromptOverride: () => [],
    });
    await resourceLoader.reload();

    const resolvedModel = resolveModel(options.model, modelRegistry);
    const { session } = await deps.createAgentSession({
      cwd: options.cwd,
      agentDir: options.agentDir,
      authStorage,
      modelRegistry,
      model: resolvedModel,
      thinkingLevel: options.thinkingLevel,
      resourceLoader,
      sessionManager: deps.createSessionManager(options.cwd),
      settingsManager,
      noTools: "builtin",
      customTools: createQueryTools(() => memoryPolicy.getStore()),
    });

    return {
      prompt: (text: string) => session.prompt(text),
      subscribe: (listener: (event: AgentSessionEvent) => void) => session.subscribe(listener),
      getLastAssistantText: () => session.getLastAssistantText(),
      exportToJsonl: (outputPath?: string) => session.exportToJsonl(outputPath),
      dispose: () => session.dispose(),
    };
  };
}

export function createDecisionEngine(
  memoryPolicy: Pick<AkgDurableMemoryPolicy, "memoryDir" | "getStore">,
  options: CreateDecisionEngineOptions = {},
) {
  const sessionDirProvider = () => options.sessionDir ?? memoryPolicy.memoryDir;
  const fakeDecisions = parseFakeDecisions(options.fakeDecisionsJSON);
  if (fakeDecisions) {
    return new ScriptedDecisionEngine({
      decisions: fakeDecisions,
      sessionDirProvider,
      sessionScope: "hand",
    });
  }

  return new PiDecisionEngine({
    cwd: options.cwd ?? process.cwd(),
    sessionDirProvider,
    model: options.model,
    thinkingLevel: parsePiThinkingLevel(options.thinkingLevel),
    sessionScope: "hand",
    sessionFactory: options.sessionFactory ?? createDurableSessionFactory(memoryPolicy),
  });
}

export function resolveAgentDir(): string {
  return getAgentDir();
}

function resolveModel(spec: string | undefined, modelRegistry: ReturnType<typeof ModelRegistry.create>): CreateAgentSessionOptions["model"] {
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

export function parsePositiveInteger(value: string | undefined): number | undefined {
  if (value === undefined || value.length === 0) return undefined;

  const parsed = Number.parseInt(value, 10);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`invalid positive integer ${JSON.stringify(value)}`);
  }

  return parsed;
}

function parseFakeDecisions(value: string | undefined): ActionPayload[] | undefined {
  if (value === undefined || value.length === 0) return undefined;

  let parsed: unknown;
  try {
    parsed = JSON.parse(value);
  } catch (error) {
    throw new Error(`invalid PI_POKER_FAKE_DECISIONS_JSON: ${error instanceof Error ? error.message : String(error)}`);
  }

  if (!Array.isArray(parsed)) {
    throw new Error("invalid PI_POKER_FAKE_DECISIONS_JSON: expected JSON array");
  }

  return parsed.map((entry, index) => parseFakeDecision(entry, index));
}

function parseFakeDecision(entry: unknown, index: number): ActionPayload {
  if (!isRecord(entry)) {
    throw new Error(`invalid PI_POKER_FAKE_DECISIONS_JSON[${index}]: expected object`);
  }
  const action = entry.action;
  if (action !== "fold" && action !== "check" && action !== "call" && action !== "bet" && action !== "raise") {
    throw new Error(`invalid PI_POKER_FAKE_DECISIONS_JSON[${index}].action`);
  }
  const rawAmount = entry.amount;
  if (rawAmount === undefined) {
    return { action };
  }
  if (typeof rawAmount !== "number" || !Number.isInteger(rawAmount) || rawAmount < 0) {
    throw new Error(`invalid PI_POKER_FAKE_DECISIONS_JSON[${index}].amount`);
  }
  return { action, amount: rawAmount };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
