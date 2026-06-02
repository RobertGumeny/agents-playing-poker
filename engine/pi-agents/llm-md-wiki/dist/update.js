import { appendFile, mkdir, readFile, rm } from "node:fs/promises";
import path from "node:path";
import { AuthStorage, createAgentSession, DefaultResourceLoader, getAgentDir, ModelRegistry, SessionManager, SettingsManager, } from "@earendil-works/pi-coding-agent";
import { parseFakeDecisions, parsePiThinkingLevel, resolveModel } from "@agent-poker/pi-agent-shared";
import { ensureRootPage, listPages, readPage, ROOT_PAGE, wikiDir, writePage } from "./pages.js";
import { createReadTools, createWriteTool } from "./tools.js";
const STDERR_LOG = "stderr.log";
const UPDATE_LOG = "update-session.jsonl";
export const WIKI_UPDATE_SYSTEM_PROMPT = [
    "You maintain a small wiki of linked markdown pages modeling one opponent in heads-up no-limit Texas Hold'em.",
    "Pages are connected by [[wiki links]]. The page \"villain\" is the root index.",
    "You are given the current page list, the current root page, and a summary of the hand that just finished. Use md_read_page to pull up any page you intend to change.",
    "Update the wiki to fold in what this hand revealed: keep the villain root summary current, create or update the relevant pages under patterns/, optionally add a page under hands/, and keep the [[links]] between pages honest.",
    "You count tendencies yourself in prose (for example \"folds to flop c-bet 4/7\"); no tool computes them, so keep your own counts current. Avoid duplicate pages for the same idea, and reconcile contradictions in favor of the newest evidence.",
    "Write each changed page with md_write_page. Do not delete pages. When finished, reply with a one-line summary of what you changed.",
].join("\n");
export function buildWikiUpdatePrompt(pages, rootContent, handSummary) {
    const pageList = pages.length > 0 ? pages.join(", ") : "(none yet)";
    const root = rootContent.trim().length > 0 ? rootContent.trim() : "(empty — no notes yet)";
    return [
        `Current pages: ${pageList}`,
        "",
        `Current ${ROOT_PAGE}.md:`,
        root,
        "",
        "Hand that just finished:",
        handSummary,
        "",
        "Read any pages you need with md_read_page, then write your updates with md_write_page.",
    ].join("\n");
}
export async function runWikiUpdate(options) {
    const dir = wikiDir(options.memoryDir);
    await ensureRootPage(dir);
    // Mirror ScriptedDecisionEngine: in fake-decision mode there is no live model, so apply
    // a deterministic scripted update (append to the root, add a hand page) and log a fake
    // transcript. This exercises the read/inject/update loop without a model.
    if (parseFakeDecisions(process.env.PI_POKER_FAKE_DECISIONS_JSON)) {
        await applyScriptedUpdate(dir, options);
        await appendUpdateLog(options.memoryDir, {
            type: "fake_update_session",
            hand_number: options.handNumber,
            pages_written: [ROOT_PAGE, `hands/hand-${options.handNumber}`],
        });
        return;
    }
    const pages = await listPages(dir);
    const root = await readPage(dir, ROOT_PAGE);
    const prompt = buildWikiUpdatePrompt(pages, root.content ?? "", options.handSummary);
    let session;
    try {
        session = await createUpdateSession({
            cwd: options.cwd ?? process.cwd(),
            model: options.model,
            thinkingLevel: parsePiThinkingLevel(options.thinkingLevel),
            getWikiDir: () => dir,
        });
        await promptOnce(session, prompt);
    }
    catch (error) {
        await logStderr(options.memoryDir, `hand ${options.handNumber} wiki update failed (${describeError(error)}); kept prior wiki`);
    }
    finally {
        if (session) {
            try {
                await exportUpdateLog(session, options.memoryDir);
            }
            catch (error) {
                await logStderr(options.memoryDir, `hand ${options.handNumber} update transcript export failed (${describeError(error)})`);
            }
            session.dispose();
        }
    }
}
async function applyScriptedUpdate(dir, options) {
    const root = await readPage(dir, ROOT_PAGE);
    const handPage = `hands/hand-${options.handNumber}`;
    const section = `## Hand ${options.handNumber}\n${options.handSummary}\n[[${handPage}]]`;
    const base = (root.content ?? "").trim();
    const updatedRoot = base.length > 0 ? `${base}\n\n${section}` : `# villain\n\n${section}`;
    await writePage(dir, ROOT_PAGE, updatedRoot);
    await writePage(dir, handPage, `# Hand ${options.handNumber}\n\n${options.handSummary}`);
}
async function promptOnce(session, prompt) {
    let streamed = "";
    const unsubscribe = session.subscribe((event) => {
        if (event.type === "message_update" && event.assistantMessageEvent.type === "text_delta") {
            streamed += event.assistantMessageEvent.delta;
        }
    });
    try {
        await session.prompt(prompt);
        return (session.getLastAssistantText() ?? streamed).trim();
    }
    finally {
        unsubscribe();
    }
}
async function createUpdateSession(options) {
    const agentDir = getAgentDir();
    const authStorage = AuthStorage.create();
    const modelRegistry = ModelRegistry.create(authStorage);
    const settingsManager = SettingsManager.create(options.cwd, agentDir);
    settingsManager.applyOverrides({
        compaction: { enabled: false },
        retry: { enabled: false },
    });
    const resourceLoader = new DefaultResourceLoader({
        cwd: options.cwd,
        agentDir,
        noExtensions: true,
        noSkills: true,
        noPromptTemplates: true,
        noThemes: true,
        noContextFiles: true,
        systemPromptOverride: () => WIKI_UPDATE_SYSTEM_PROMPT,
        appendSystemPromptOverride: () => [],
    });
    await resourceLoader.reload();
    const resolvedModel = resolveModel(options.model, modelRegistry);
    const { session } = await createAgentSession({
        cwd: options.cwd,
        agentDir,
        authStorage,
        modelRegistry,
        model: resolvedModel,
        thinkingLevel: options.thinkingLevel,
        resourceLoader,
        sessionManager: SessionManager.inMemory(options.cwd),
        settingsManager,
        noTools: "builtin",
        customTools: [...createReadTools(options.getWikiDir), createWriteTool(options.getWikiDir)],
    });
    return session;
}
let updateExportCount = 0;
async function exportUpdateLog(session, memoryDir) {
    await mkdir(memoryDir, { recursive: true });
    const exportPath = path.join(memoryDir, `update-session-export-${String(++updateExportCount).padStart(4, "0")}.jsonl`);
    const canonicalPath = path.join(memoryDir, UPDATE_LOG);
    session.exportToJsonl(exportPath);
    try {
        const exported = await readFile(exportPath, "utf8");
        if (exported.length > 0) {
            await appendFile(canonicalPath, exported);
        }
    }
    finally {
        await rm(exportPath, { force: true });
    }
}
async function appendUpdateLog(memoryDir, entry) {
    await mkdir(memoryDir, { recursive: true });
    await appendFile(path.join(memoryDir, UPDATE_LOG), `${JSON.stringify(entry)}\n`, "utf8");
}
async function logStderr(memoryDir, message) {
    try {
        await mkdir(memoryDir, { recursive: true });
        await appendFile(path.join(memoryDir, STDERR_LOG), `${message}\n`, "utf8");
    }
    catch {
        // best-effort diagnostics; never throw out of the update path
    }
}
function describeError(error) {
    return error instanceof Error ? error.message : String(error);
}
