import { appendFile, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import path from "node:path";
import { AuthStorage, createAgentSession, DefaultResourceLoader, getAgentDir, ModelRegistry, SessionManager, SettingsManager, } from "@earendil-works/pi-coding-agent";
import { parseFakeDecisions, parsePiThinkingLevel, resolveModel } from "@agent-poker/pi-agent-shared";
export const NOTES_FILENAME = "notes.md";
const STDERR_LOG = "stderr.log";
const UPDATE_LOG = "update-session.jsonl";
export const UPDATE_SYSTEM_PROMPT = [
    "You maintain a single markdown file of notes about one opponent in heads-up no-limit Texas Hold'em.",
    "You are given your current notes and a plain summary of the hand that just finished.",
    "Rewrite the notes so they stay an accurate, useful model of this opponent: keep the durable facts and the running stat tallies, fold in what the new hand revealed, and reconcile contradictions in favor of the newest evidence.",
    "You track tendencies and counts yourself, in prose (for example \"folds to flop c-bet 4/7\") — no tool computes them for you, so keep your own count current.",
    "Return the COMPLETE updated markdown file and nothing else: no commentary, no code fences, no explanation outside the file.",
].join("\n");
export function buildUpdatePrompt(currentNotes, handSummary) {
    const notes = currentNotes.trim().length > 0 ? currentNotes.trim() : "(empty — no notes yet)";
    return [
        "Current notes.md:",
        notes,
        "",
        "Hand that just finished:",
        handSummary,
        "",
        `Return the complete updated ${NOTES_FILENAME}.`,
    ].join("\n");
}
export async function readNotes(memoryDir) {
    if (!memoryDir)
        return "";
    try {
        return await readFile(path.join(memoryDir, NOTES_FILENAME), "utf8");
    }
    catch {
        return "";
    }
}
// The model owns the file; this only strips a single ```/```markdown transport fence
// if it wrapped the whole response, which is a formatting artifact, not content.
function unwrapFence(text) {
    const trimmed = text.trim();
    if (!trimmed.startsWith("```"))
        return trimmed;
    const firstNewline = trimmed.indexOf("\n");
    if (firstNewline === -1)
        return trimmed;
    const withoutOpen = trimmed.slice(firstNewline + 1);
    const closingFence = withoutOpen.lastIndexOf("```");
    if (closingFence === -1)
        return trimmed;
    return withoutOpen.slice(0, closingFence).trim();
}
let updateExportCount = 0;
export async function runNotesUpdate(options) {
    const notesPath = path.join(options.memoryDir, NOTES_FILENAME);
    const current = await readNotes(options.memoryDir);
    const prompt = buildUpdatePrompt(current, options.handSummary);
    // Mirror ScriptedDecisionEngine: in fake-decision mode there is no live model, so apply
    // a deterministic scripted rewrite (append the hand) and log a fake update transcript.
    if (parseFakeDecisions(process.env.PI_POKER_FAKE_DECISIONS_JSON)) {
        const section = `## Hand ${options.handNumber}\n${options.handSummary}`;
        const updated = current.trim().length > 0 ? `${current.trim()}\n\n${section}` : section;
        await writeFile(notesPath, `${updated}\n`, "utf8");
        await appendUpdateLog(options.memoryDir, {
            type: "fake_update_session",
            hand_number: options.handNumber,
            prompt,
            notes_written: true,
        });
        return;
    }
    let session;
    try {
        session = await createUpdateSession({
            cwd: options.cwd ?? process.cwd(),
            model: options.model,
            thinkingLevel: parsePiThinkingLevel(options.thinkingLevel),
        });
        const text = await promptOnce(session, prompt);
        const updated = unwrapFence(text);
        if (updated.length === 0) {
            await logStderr(options.memoryDir, `hand ${options.handNumber} notes update returned empty output; kept prior ${NOTES_FILENAME}`);
            return;
        }
        await writeFile(notesPath, `${updated}\n`, "utf8");
    }
    catch (error) {
        await logStderr(options.memoryDir, `hand ${options.handNumber} notes update failed (${describeError(error)}); kept prior ${NOTES_FILENAME}`);
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
        systemPromptOverride: () => UPDATE_SYSTEM_PROMPT,
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
        tools: [],
    });
    return session;
}
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
