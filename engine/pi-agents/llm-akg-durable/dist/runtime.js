import { AuthStorage, createAgentSession, DefaultResourceLoader, getAgentDir, ModelRegistry, SessionManager, SettingsManager, } from "@earendil-works/pi-coding-agent";
import { PiDecisionEngine, ScriptedDecisionEngine, parseFakeDecisions, parsePiThinkingLevel, parsePositiveInteger, resolveModel, } from "@agent-poker/pi-agent-shared";
import { createReadTools } from "./tools.js";
export const DURABLE_SYSTEM_PROMPT = [
    "You are a poker decision engine for heads-up no-limit Texas Hold'em.",
    "Your memory of this opponent is an AKG knowledge graph. The opponent/villain node is an index only — the real pattern data is in the tendency nodes it links to. Its body is provided inline below; do NOT spend a tool call re-fetching it.",
    "Before deciding, follow the index's edges and read the relevant tendency nodes with akg_get_node or akg_get_nodes (use akg_list_nodes to discover what else is recorded). Do not rely on the inline index summary alone.",
    "After reading, choose exactly one legal action from the user-provided legal_actions list.",
    'Your final response must be JSON only: {"action": string, "amount"?: number}.',
    "No commentary, markdown, code fences, or extra keys in the final JSON response.",
    "If raising or betting, use an integer chip amount within the server-provided legal range.",
].join("\n");
const defaultSessionFactoryDeps = {
    createAuthStorage: () => AuthStorage.create(),
    createModelRegistry: (authStorage) => ModelRegistry.create(authStorage),
    createSettingsManager: (cwd, agentDir) => SettingsManager.create(cwd, agentDir),
    createResourceLoader: (options) => new DefaultResourceLoader(options),
    createSessionManager: (cwd) => SessionManager.inMemory(cwd),
    createAgentSession,
};
export function createDurableSessionFactory(memoryPolicy, deps = defaultSessionFactoryDeps) {
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
            prompt: (text) => session.prompt(text),
            subscribe: (listener) => session.subscribe(listener),
            getLastAssistantText: () => session.getLastAssistantText(),
            exportToJsonl: (outputPath) => session.exportToJsonl(outputPath),
            dispose: () => session.dispose(),
        };
    };
}
export function createDecisionEngine(memoryPolicy, options = {}) {
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
export function resolveAgentDir() {
    return getAgentDir();
}
export { parsePositiveInteger };
