import { appendFile, mkdir, readFile, rm } from "node:fs/promises";
import path from "node:path";
import { AuthStorage, createAgentSession, DefaultResourceLoader, getAgentDir, ModelRegistry, SessionManager, SettingsManager, } from "@earendil-works/pi-coding-agent";
import { parseFakeDecisions, parsePiThinkingLevel, resolveModel } from "@agent-poker/pi-agent-shared";
import { ensureRootPage, listPages, readPage, ROOT_PAGE, wikiDir, writePage } from "./pages.js";
import { createReadTools, createWriteTool } from "./tools.js";
const STDERR_LOG = "stderr.log";
const UPDATE_LOG = "update-session.jsonl";
export const WIKI_UPDATE_SYSTEM_PROMPT = `You maintain a small wiki of linked markdown pages modeling one opponent in heads-up no-limit Texas Hold'em.
Pages use YAML frontmatter and are connected by [[wiki links]].

PAGE ROLES — follow these exactly:

villain.md — PURE INDEX. Allowed content only:
  • YAML frontmatter: slug, tags, hands_observed, last_updated
  • A ## Stats section: up to ~10 one-line bullets (e.g. "- Folds to c-bet: 5/10 → [[patterns/folds-to-cbet]]")
  • A ## Pages section: one line per page — [[slug]] — one-sentence description
  • A ## Notable Hands section (optional): [[hands/hand-N]] links with one-line notes ONLY for hands that changed the model — a first observation of a new tendency, a behaviour that contradicts prior reads, or an unusual showdown. Do NOT add an entry for hands that are routine evidence for an already-established pattern; those belong only in the pattern page.
  FORBIDDEN in villain.md: per-hand narratives, positional notes, action logs, hand histories, routine evidence entries.
  If villain.md body exceeds ~25 lines, you are accumulating narrative — move detail to pages.

patterns/<slug>.md — Pattern detail page. Contains:
  • YAML frontmatter: slug, tags (e.g. [pattern, flop, c-bet]), sample_size, last_updated
  • Count line: **Count: N/M** (keep this accurate each hand)
  • Evidence: concise notes per observed instance; link to [[hands/hand-N]] for detail
  • Cross-links to related patterns

hands/hand-N.md — One page per notable hand. Contains:
  • YAML frontmatter: slug, tags: [hand], hand: N
  • Full action summary and what it revealed
  • Links back to the pattern pages it supports

WORKFLOW each hand:
1. Read any pages you intend to update (md_read_page).
2. Update pattern page counts and evidence. Create the pattern page if it doesn't exist.
3. Create hands/hand-N.md if the hand is notable evidence for a pattern.
4. Update villain.md: refresh the ## Stats one-liners and ## Pages directory. Do NOT add a per-hand entry.
5. Write all changed pages with md_write_page.
6. Reply with a one-line summary of what you changed.

Keep [[links]] honest. Reconcile contradictions in favor of newest evidence. Do not delete pages.`;
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
    const handSlug = `hands/hand-${options.handNumber}`;
    const base = (root.content ?? "").trim();
    const handsLine = `- [[${handSlug}]] — hand ${options.handNumber}`;
    let updatedRoot;
    if (base.includes("## Notable Hands")) {
        updatedRoot = base.replace("## Notable Hands", `## Notable Hands\n${handsLine}`);
    }
    else {
        updatedRoot = base.length > 0
            ? `${base}\n\n## Notable Hands\n${handsLine}`
            : `---\nslug: villain\ntags: [index, opponent-model]\nhands_observed: ${options.handNumber}\nlast_updated: hand-${options.handNumber}\n---\n# Villain\n\n## Notable Hands\n${handsLine}`;
    }
    await writePage(dir, ROOT_PAGE, updatedRoot);
    await writePage(dir, handSlug, `---\nslug: ${handSlug}\ntags: [hand]\nhand: ${options.handNumber}\n---\n# Hand ${options.handNumber}\n\n${options.handSummary}`);
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
