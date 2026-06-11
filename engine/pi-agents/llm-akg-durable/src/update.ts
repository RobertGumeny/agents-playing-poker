// Post-hand update session for the durable agent: a fresh one-shot Pi session with the AKG
// read+write tools that lets the model fold the finished hand into the graph. Records the
// update transcript and the graph-rot diagnostic separately from decision-time cost.

import { appendFile, mkdir, readFile, rm } from "node:fs/promises";
import path from "node:path";

import {
  type AgentSessionEvent,
  AuthStorage,
  createAgentSession,
  type CreateAgentSessionOptions,
  DefaultResourceLoader,
  getAgentDir,
  ModelRegistry,
  SessionManager,
  SettingsManager,
} from "@earendil-works/pi-coding-agent";
import { parseFakeDecisions, parsePiThinkingLevel, resolveModel } from "@agent-poker/pi-agent-shared";
import type { Store } from "akg-ts";

import { ensureRootNode, ROOT_ID, ROOT_TYPE } from "./graph.js";
import { createReadTools, createWriteTools, type StoreProvider } from "./tools.js";

const STDERR_LOG = "stderr.log";
const UPDATE_LOG = "update-session.jsonl";

export const DURABLE_UPDATE_SYSTEM_PROMPT = [
  "You maintain an AKG knowledge graph modeling one opponent in heads-up no-limit Texas Hold'em.",
  "Nodes carry type, id, title, body, tags, and structured meta. You connect them with directed, typed edges (you name the relation).",
  "",
  "NODE ROLES — follow these exactly. You invent your own node types, ids, and edge relations; the names below describe roles, not a fixed schema. The one fixed node is the root index, " + `${ROOT_TYPE}/${ROOT_ID}.`,
  "",
  `The ${ROOT_TYPE}/${ROOT_ID} node — PURE INDEX. Allowed content only:`,
  '  • A stats summary (body lines or meta): up to ~10 one-line tendencies, each naming its tendency node (e.g. "folds to c-bet 5/10 → the folds-to-cbet node")',
  "  • A directory of the nodes that exist — one line each, one-sentence description",
  "  • Optional notable-hand pointers: edges to hand nodes ONLY for hands that changed the model — a first observation of a new tendency, a behaviour that contradicts prior reads, or an unusual showdown. Do NOT point at hands that are routine evidence for an already-established pattern; those belong only in the tendency node.",
  "  FORBIDDEN in the index node: per-hand narratives, action logs, hand histories, routine evidence entries.",
  "  If the index body exceeds ~25 lines, you are accumulating narrative — move detail into tendency nodes.",
  "",
  "Tendency nodes (one per observed pattern, named as you see fit) — pattern detail. Contain:",
  "  • tags describing the pattern (e.g. flop, c-bet)",
  '  • the running count in meta (e.g. {"count": "5/10"}) — keep it accurate each hand',
  "  • concise evidence notes in the body; edges to hand nodes for detail",
  "  • typed edges to related tendencies (relations you choose: supported_by, contradicts, superseded_by, …)",
  "",
  "Notable-hand nodes (optional, one per genuinely notable hand) — contain:",
  "  • a full action summary and what it revealed",
  "  • edges back to the tendency nodes it supports",
  "  Do not create one node per hand; add a hand node only when it is genuinely notable.",
  "",
  "WORKFLOW each hand:",
  "1. Read any nodes you intend to update (akg_list_nodes to discover, akg_get_node/akg_get_nodes to read).",
  "2. Update the relevant tendency node's count (meta) and evidence. Create it if it doesn't exist.",
  "3. Create a hand node only if the hand is notable evidence for a tendency.",
  "4. Update the index node: refresh the stats one-liners and the node directory. Do NOT add a per-hand entry.",
  "5. Apply ALL changes in a SINGLE akg_apply call (it upserts every node before every edge so endpoints exist).",
  "6. Reply with a one-line summary of what you changed.",
  "",
  'Only record what is worth remembering. Most heads-up hands — preflop folds, blind steals, uncontested pots — reveal nothing; for those, write nothing and reply "no durable change". Keep edges honest, reconcile contradictions in favor of newest evidence, avoid duplicate nodes for the same idea, and do not delete nodes or edges.',
].join("\n");

type PiSession = {
  prompt(text: string): Promise<void>;
  subscribe(listener: (event: AgentSessionEvent) => void): () => void;
  getLastAssistantText(): string | undefined;
  exportToJsonl(outputPath?: string): string;
  dispose(): void;
};

export interface DurableUpdateOptions {
  memoryDir: string;
  handNumber: number;
  handSummary: string;
  getStore: StoreProvider;
  cwd?: string;
  model?: string;
  thinkingLevel?: string;
}

export function buildDurableUpdatePrompt(rootBody: string, handSummary: string): string {
  const root = rootBody.trim().length > 0 ? rootBody.trim() : "(empty — no notes yet)";
  return [
    `Current ${ROOT_TYPE}/${ROOT_ID} summary:`,
    root,
    "",
    "Hand that just finished:",
    handSummary,
    "",
    "If this hand is worth recording, read any node you need with akg_list_nodes / akg_get_node, then apply every change in one akg_apply call. If it reveals nothing durable, reply that there is no durable change and write nothing.",
  ].join("\n");
}

export async function runDurableUpdate(options: DurableUpdateOptions): Promise<void> {
  const store = await options.getStore();
  if (!store) return;
  ensureRootNode(store);

  // Mirror ScriptedDecisionEngine: in fake-decision mode there is no live model, so apply a
  // deterministic scripted update (append the hand to the root, add a hand node and edge) and
  // log a fake transcript. This exercises the read/inject/update loop without a model.
  if (parseFakeDecisions(process.env.PI_POKER_FAKE_DECISIONS_JSON)) {
    const handId = applyScriptedUpdate(store, options);
    await store.commit();
    await appendUpdateLog(options.memoryDir, {
      type: "fake_update_session",
      hand_number: options.handNumber,
      nodes_written: [`${ROOT_TYPE}/${ROOT_ID}`, `hand/${handId}`],
    });
    return;
  }

  const rootBody = store.getNode(ROOT_TYPE, ROOT_ID)?.body ?? "";
  const prompt = buildDurableUpdatePrompt(rootBody, options.handSummary);

  let session: PiSession | undefined;
  try {
    session = await createUpdateSession({
      cwd: options.cwd ?? process.cwd(),
      model: options.model,
      thinkingLevel: parsePiThinkingLevel(options.thinkingLevel),
      getStore: options.getStore,
    });
    await promptOnce(session, prompt);
    await store.commit();
  } catch (error) {
    await logStderr(options.memoryDir, `hand ${options.handNumber} durable update failed (${describeError(error)}); kept prior graph`);
  } finally {
    if (session) {
      try {
        await exportUpdateLog(session, options.memoryDir);
      } catch (error) {
        await logStderr(options.memoryDir, `hand ${options.handNumber} update transcript export failed (${describeError(error)})`);
      }
      session.dispose();
    }
  }
}

function applyScriptedUpdate(store: Store, options: DurableUpdateOptions): string {
  const handId = `hand-${options.handNumber}`;
  store.putNode("hand", handId, { title: `Hand ${options.handNumber}`, body: options.handSummary }, ["hand"]);
  store.putEdge({ type: ROOT_TYPE, id: ROOT_ID }, "has_hand", { type: "hand", id: handId }, { strength: 0.5 });

  const base = (store.getNode(ROOT_TYPE, ROOT_ID)?.body ?? "").trim();
  const line = `Hand ${options.handNumber}: ${options.handSummary}`;
  const updatedBody = base.length > 0 && base !== "(no reads yet)" ? `${base}\n\n${line}` : line;
  store.putNode(ROOT_TYPE, ROOT_ID, { title: ROOT_ID, body: updatedBody }, [ROOT_TYPE]);
  return handId;
}

async function promptOnce(session: PiSession, prompt: string): Promise<string> {
  let streamed = "";
  const unsubscribe = session.subscribe((event) => {
    if (event.type === "message_update" && event.assistantMessageEvent.type === "text_delta") {
      streamed += event.assistantMessageEvent.delta;
    }
  });
  try {
    await session.prompt(prompt);
    return (session.getLastAssistantText() ?? streamed).trim();
  } finally {
    unsubscribe();
  }
}

async function createUpdateSession(options: {
  cwd: string;
  model?: string;
  thinkingLevel?: NonNullable<CreateAgentSessionOptions["thinkingLevel"]>;
  getStore: StoreProvider;
}): Promise<PiSession> {
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

async function exportUpdateLog(session: PiSession, memoryDir: string): Promise<void> {
  await mkdir(memoryDir, { recursive: true });
  const exportPath = path.join(memoryDir, `update-session-export-${String(++updateExportCount).padStart(4, "0")}.jsonl`);
  const canonicalPath = path.join(memoryDir, UPDATE_LOG);
  session.exportToJsonl(exportPath);
  try {
    const exported = await readFile(exportPath, "utf8");
    if (exported.length > 0) {
      await appendFile(canonicalPath, exported);
    }
  } finally {
    await rm(exportPath, { force: true });
  }
}

async function appendUpdateLog(memoryDir: string, entry: Record<string, unknown>): Promise<void> {
  await mkdir(memoryDir, { recursive: true });
  await appendFile(path.join(memoryDir, UPDATE_LOG), `${JSON.stringify(entry)}\n`, "utf8");
}

async function logStderr(memoryDir: string, message: string): Promise<void> {
  try {
    await mkdir(memoryDir, { recursive: true });
    await appendFile(path.join(memoryDir, STDERR_LOG), `${message}\n`, "utf8");
  } catch {
    // best-effort diagnostics; never throw out of the update path
  }
}

function describeError(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
