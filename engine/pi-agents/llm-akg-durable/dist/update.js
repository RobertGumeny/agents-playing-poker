import { appendFile, mkdir, readFile, rm } from "node:fs/promises";
import path from "node:path";
import { AuthStorage, createAgentSession, DefaultResourceLoader, getAgentDir, ModelRegistry, SessionManager, SettingsManager, } from "@earendil-works/pi-coding-agent";
import { parseFakeDecisions, parsePiThinkingLevel, resolveModel } from "@agent-poker/pi-agent-shared";
import { countGraphRot, ensureRootNode, nodeSummaryLine, ROOT_ID, ROOT_TYPE } from "./graph.js";
import { createReadTools, createWriteTools } from "./tools.js";
const STDERR_LOG = "stderr.log";
const UPDATE_LOG = "update-session.jsonl";
const DIAGNOSTICS_LOG = "diagnostics.jsonl";
export const DURABLE_UPDATE_SYSTEM_PROMPT = [
    "You maintain an AKG knowledge graph modeling one opponent in heads-up no-limit Texas Hold'em.",
    `Nodes are connected by directed edges you create. The node ${ROOT_TYPE}/${ROOT_ID} is the root index.`,
    "You are given the current node list, the current root node body, and a summary of the hand that just finished. Use akg_get_node to pull up any node you intend to change.",
    `Update the graph to fold in what this hand revealed: keep the ${ROOT_TYPE}/${ROOT_ID} root summary current, create or update nodes for the tendencies you observe, optionally add a node for a notable hand, and connect them with edges (relations you choose) so the graph stays navigable.`,
    'You invent your own node types, ids, and edge relations — you are not limited to a fixed schema. Count tendencies yourself in node bodies or meta (for example "folds to flop c-bet 4/7"); no tool computes them, so keep your own counts current. Avoid duplicate nodes for the same idea, and reconcile contradictions in favor of the newest evidence.',
    "Write nodes with akg_put_node and edges with akg_put_edge. Both endpoints of an edge must exist first. Do not delete nodes or edges. When finished, reply with a one-line summary of what you changed.",
].join("\n");
export function buildDurableUpdatePrompt(nodeSummaries, rootBody, handSummary) {
    const list = nodeSummaries.length > 0 ? nodeSummaries.join("\n") : "(none yet)";
    const root = rootBody.trim().length > 0 ? rootBody.trim() : "(empty — no notes yet)";
    return [
        "Current nodes:",
        list,
        "",
        `Current ${ROOT_TYPE}/${ROOT_ID} body:`,
        root,
        "",
        "Hand that just finished:",
        handSummary,
        "",
        "Read any node you need with akg_get_node, then write your updates with akg_put_node and akg_put_edge.",
    ].join("\n");
}
export async function runDurableUpdate(options) {
    const store = await options.getStore();
    if (!store)
        return;
    ensureRootNode(store);
    // Mirror ScriptedDecisionEngine: in fake-decision mode there is no live model, so apply a
    // deterministic scripted update (append the hand to the root, add a hand node and edge) and
    // log a fake transcript. This exercises the read/inject/update loop without a model.
    if (parseFakeDecisions(process.env.PI_POKER_FAKE_DECISIONS_JSON)) {
        const handId = applyScriptedUpdate(store, options);
        await store.commit();
        await appendRotDiagnostic(store, options.memoryDir, options.handNumber);
        await appendUpdateLog(options.memoryDir, {
            type: "fake_update_session",
            hand_number: options.handNumber,
            nodes_written: [`${ROOT_TYPE}/${ROOT_ID}`, `hand/${handId}`],
        });
        return;
    }
    const nodeSummaries = store.listNodes().map(nodeSummaryLine);
    const rootBody = store.getNode(ROOT_TYPE, ROOT_ID)?.body ?? "";
    const prompt = buildDurableUpdatePrompt(nodeSummaries, rootBody, options.handSummary);
    let session;
    try {
        session = await createUpdateSession({
            cwd: options.cwd ?? process.cwd(),
            model: options.model,
            thinkingLevel: parsePiThinkingLevel(options.thinkingLevel),
            getStore: options.getStore,
        });
        await promptOnce(session, prompt);
        await store.commit();
    }
    catch (error) {
        await logStderr(options.memoryDir, `hand ${options.handNumber} durable update failed (${describeError(error)}); kept prior graph`);
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
    await appendRotDiagnostic(store, options.memoryDir, options.handNumber);
}
function applyScriptedUpdate(store, options) {
    const handId = `hand-${options.handNumber}`;
    store.putNode("hand", handId, { title: `Hand ${options.handNumber}`, body: options.handSummary }, ["hand"]);
    store.putEdge({ type: ROOT_TYPE, id: ROOT_ID }, "has_hand", { type: "hand", id: handId }, { strength: 0.5 });
    const base = (store.getNode(ROOT_TYPE, ROOT_ID)?.body ?? "").trim();
    const line = `Hand ${options.handNumber}: ${options.handSummary}`;
    const updatedBody = base.length > 0 && base !== "(no reads yet)" ? `${base}\n\n${line}` : line;
    store.putNode(ROOT_TYPE, ROOT_ID, { title: ROOT_ID, body: updatedBody }, [ROOT_TYPE]);
    return handId;
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
        systemPromptOverride: () => DURABLE_UPDATE_SYSTEM_PROMPT,
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
        customTools: [...createReadTools(options.getStore), ...createWriteTools(options.getStore)],
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
// Best-effort structural diagnostic; counts rot, never repairs it, and never throws out of
// the update path.
async function appendRotDiagnostic(store, memoryDir, handNumber) {
    try {
        const rot = countGraphRot(store);
        await mkdir(memoryDir, { recursive: true });
        await appendFile(path.join(memoryDir, DIAGNOSTICS_LOG), `${JSON.stringify({ type: "graph_rot", hand_number: handNumber, ...rot })}\n`, "utf8");
    }
    catch {
        // diagnostics are best-effort
    }
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
