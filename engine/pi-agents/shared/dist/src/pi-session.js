import { appendFile, mkdir, readFile, rm } from "node:fs/promises";
import path from "node:path";
import { AuthStorage, createAgentSession, DefaultResourceLoader, getAgentDir, ModelRegistry, SessionManager, SettingsManager, } from "@earendil-works/pi-coding-agent";
import { parseActionResponse } from "./action.js";
const POKER_SYSTEM_PROMPT = [
    "You are a poker decision engine for heads-up no-limit Texas Hold'em.",
    "Choose exactly one legal action from the user-provided legal_actions list.",
    'Return JSON only with shape {"action": string, "amount"?: number}.',
    "Do not add commentary, markdown, code fences, or extra keys.",
    "If raising or betting, choose an integer chip amount within the server-provided legal range.",
].join("\n");
const THINKING_LEVELS = ["off", "minimal", "low", "medium", "high", "xhigh"];
export class PiDecisionEngine {
    options;
    sessionFactory;
    sessionScope;
    exportCount = 0;
    activeHandSession;
    constructor(options) {
        this.options = options;
        this.sessionFactory = options.sessionFactory ?? createPiSession;
        this.sessionScope = options.sessionScope ?? "decision";
    }
    async decide(request) {
        if (this.sessionScope === "hand") {
            const session = await this.ensureHandSession(request.context.handNumber);
            return this.promptSession(session, request.prompt);
        }
        const session = await this.createSession();
        try {
            return await this.promptSession(session, request.prompt);
        }
        finally {
            await this.persistAndDispose(session);
        }
    }
    async onHandEnd(context) {
        if (this.sessionScope !== "hand")
            return;
        if (this.activeHandSession?.handNumber !== context.handNumber)
            return;
        const session = this.activeHandSession.session;
        this.activeHandSession = undefined;
        await this.persistAndDispose(session);
    }
    async onSessionEnd() {
        if (!this.activeHandSession)
            return;
        const session = this.activeHandSession.session;
        this.activeHandSession = undefined;
        await this.persistAndDispose(session);
    }
    async ensureHandSession(handNumber) {
        if (this.activeHandSession?.handNumber === handNumber) {
            return this.activeHandSession.session;
        }
        if (this.activeHandSession) {
            const session = this.activeHandSession.session;
            this.activeHandSession = undefined;
            await this.persistAndDispose(session);
        }
        const session = await this.createSession();
        this.activeHandSession = { handNumber, session };
        return session;
    }
    async createSession() {
        return this.sessionFactory({
            cwd: this.options.cwd,
            agentDir: this.options.agentDir ?? getAgentDir(),
            sessionDir: this.options.sessionDirProvider?.() ?? this.options.sessionDir,
            model: this.options.model,
            thinkingLevel: this.options.thinkingLevel,
        });
    }
    async promptSession(session, prompt) {
        let unsubscribe = () => { };
        let streamedAssistantText = "";
        try {
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
        }
        catch (error) {
            throw new Error(`pi decision failed: ${error instanceof Error ? error.message : String(error)}`);
        }
        finally {
            unsubscribe();
        }
    }
    async persistAndDispose(session) {
        try {
            const sessionDir = this.options.sessionDirProvider?.() ?? this.options.sessionDir;
            await persistSessionLog(session, sessionDir, ++this.exportCount);
        }
        finally {
            session.dispose();
        }
    }
}
export class ScriptedDecisionEngine {
    options;
    index = 0;
    sessionCount = 0;
    activeHandNumber;
    constructor(options) {
        this.options = options;
    }
    async decide(request) {
        const decisionNumber = this.index + 1;
        const decision = this.options.decisions[Math.min(this.index, this.options.decisions.length - 1)];
        this.index += 1;
        if (!decision) {
            throw new Error("no scripted decision available");
        }
        const matched = request.legalActions.find((action) => {
            if (action.action !== decision.action)
                return false;
            if (decision.amount !== undefined) {
                return action.amount === decision.amount || (action.min !== undefined && action.max !== undefined && decision.amount >= action.min && decision.amount <= action.max);
            }
            return true;
        });
        if (!matched) {
            throw new Error(`scripted decision ${JSON.stringify(decision)} is not legal for this turn`);
        }
        await this.writeObservabilityLog(request.prompt, request.legalActions, request.context.handNumber, decisionNumber, decision);
        return decision;
    }
    onHandStart(context) {
        if (this.options.sessionScope === "hand") {
            this.sessionCount += 1;
            this.activeHandNumber = context.handNumber;
        }
    }
    onHandEnd(context) {
        if (this.options.sessionScope === "hand" && this.activeHandNumber === context.handNumber) {
            this.activeHandNumber = undefined;
        }
    }
    async writeObservabilityLog(prompt, legalActions, handNumber, decisionNumber, decision) {
        const sessionDir = this.options.sessionDirProvider?.();
        if (!sessionDir)
            return;
        if (this.options.sessionScope !== "hand") {
            this.sessionCount += 1;
        }
        await mkdir(sessionDir, { recursive: true });
        await appendFile(path.join(sessionDir, "pi-session.jsonl"), `${JSON.stringify({
            type: "fake_pi_session",
            session_scope: this.options.sessionScope ?? "decision",
            session_number: this.sessionCount,
            hand_number: handNumber,
            decision_number: decisionNumber,
            legal_actions: legalActions,
            selected_action: decision,
            prompt,
        })}\n`, "utf8");
    }
}
async function createPiSession(options) {
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
        systemPromptOverride: () => POKER_SYSTEM_PROMPT,
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
export function resolveModel(spec, modelRegistry) {
    if (!spec)
        return undefined;
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
async function persistSessionLog(session, sessionDir, exportCount) {
    if (!sessionDir)
        return;
    await mkdir(sessionDir, { recursive: true });
    const exportPath = path.join(sessionDir, `pi-session-export-${String(exportCount).padStart(4, "0")}.jsonl`);
    const canonicalPath = path.join(sessionDir, "pi-session.jsonl");
    session.exportToJsonl(exportPath);
    const exported = await readFile(exportPath, "utf8");
    if (exported.length > 0) {
        await appendFile(canonicalPath, exported);
    }
    await rm(exportPath, { force: true });
}
export function parsePiThinkingLevel(value) {
    if (value === undefined)
        return undefined;
    if (THINKING_LEVELS.includes(value)) {
        return value;
    }
    throw new Error(`invalid Pi thinking level ${JSON.stringify(value)}`);
}
export function parsePositiveInteger(value) {
    if (value === undefined || value.length === 0)
        return undefined;
    const parsed = Number.parseInt(value, 10);
    if (!Number.isInteger(parsed) || parsed <= 0) {
        throw new Error(`invalid positive integer ${JSON.stringify(value)}`);
    }
    return parsed;
}
export function parseFakeDecisions(value) {
    if (value === undefined || value.length === 0)
        return undefined;
    let parsed;
    try {
        parsed = JSON.parse(value);
    }
    catch (error) {
        throw new Error(`invalid PI_POKER_FAKE_DECISIONS_JSON: ${error instanceof Error ? error.message : String(error)}`);
    }
    if (!Array.isArray(parsed)) {
        throw new Error("invalid PI_POKER_FAKE_DECISIONS_JSON: expected JSON array");
    }
    return parsed.map((entry, index) => parseFakeDecision(entry, index));
}
function parseFakeDecision(entry, index) {
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
function isRecord(value) {
    return typeof value === "object" && value !== null && !Array.isArray(value);
}
export function createStandardDecisionEngine(options) {
    const explicitSessionDir = process.env.PI_POKER_PI_SESSION_DIR;
    const sessionDirProvider = () => explicitSessionDir ?? options.memoryDirProvider();
    const fakeDecisions = parseFakeDecisions(process.env.PI_POKER_FAKE_DECISIONS_JSON);
    if (fakeDecisions) {
        return new ScriptedDecisionEngine({
            decisions: fakeDecisions,
            sessionDirProvider,
            sessionScope: options.sessionScope,
        });
    }
    return new PiDecisionEngine({
        cwd: process.cwd(),
        sessionDirProvider,
        model: process.env.PI_POKER_MODEL,
        thinkingLevel: parsePiThinkingLevel(process.env.PI_POKER_THINKING_LEVEL),
        sessionScope: options.sessionScope,
    });
}
