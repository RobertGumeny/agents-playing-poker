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
  parseFakeDecisions,
  parsePiThinkingLevel,
  parsePositiveInteger,
  resolveModel,
} from "@agent-poker/pi-agent-shared";

import type { AkgDurableMemoryPolicy } from "./memory.js";
import { createReadTools } from "./tools.js";

export const DURABLE_SYSTEM_PROMPT = [
  "You are a poker decision engine for heads-up no-limit Texas Hold'em.",
  "You have access to AKG memory tools holding a knowledge graph about your opponent. The node opponent/villain is your index: call akg_get_node with type opponent and id villain first to read the summary, then follow its edges by reading the connected nodes.",
  "Use akg_list_nodes to discover what has been recorded and akg_get_node to read any node and its edges before your final answer.",
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
      customTools: createReadTools(() => memoryPolicy.getStore()),
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

export { parsePositiveInteger };
