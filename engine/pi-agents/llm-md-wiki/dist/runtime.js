import { AuthStorage, createAgentSession, DefaultResourceLoader, ModelRegistry, SessionManager, SettingsManager, } from "@earendil-works/pi-coding-agent";
import { parseFakeDecisions, parsePiThinkingLevel, parsePositiveInteger, PiDecisionEngine, resolveModel, ScriptedDecisionEngine, } from "@agent-poker/pi-agent-shared";
import { createReadTools } from "./tools.js";
export const WIKI_DECISION_SYSTEM_PROMPT = [
    "You are a poker decision engine for heads-up no-limit Texas Hold'em.",
    "Your memory of this opponent is a small wiki of linked markdown pages. The page \"villain\" is your index.",
    "Call md_list_pages to see all pages and md_read_page to read one; follow [[links]] by reading their targets before you decide.",
    "After any research, choose exactly one legal action from the user-provided legal_actions list.",
    'Your final response must be JSON only: {"action": string, "amount"?: number}.',
    "No commentary, markdown, code fences, or extra keys in the final JSON response.",
    "If raising or betting, use an integer chip amount within the server-provided legal range.",
].join("\n");
export function createWikiSessionFactory(memoryPolicy) {
    return async (options) => {
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
            systemPromptOverride: () => WIKI_DECISION_SYSTEM_PROMPT,
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
            noTools: "builtin",
            customTools: createReadTools(() => memoryPolicy.getWikiDir()),
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
        sessionFactory: options.sessionFactory ?? createWikiSessionFactory(memoryPolicy),
    });
}
export { parsePositiveInteger };
