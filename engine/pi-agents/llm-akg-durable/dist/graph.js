import { open } from "akg-ts";
export const STORE_FILE = "memory.akg";
export const ROOT_TYPE = "opponent";
export const ROOT_ID = "villain";
const ROOT_SEED_BODY = "(no reads yet)";
export async function openStore(memoryDir) {
    const { join } = await import("node:path");
    return open(join(memoryDir, STORE_FILE));
}
// Seeds the conventional root index node the model is pointed at, mirroring the wiki agent's
// seeded villain.md. The model is otherwise free to author any node types, ids, and relations.
// Does not commit; the caller owns the durability boundary.
export function ensureRootNode(store) {
    if (store.getNode(ROOT_TYPE, ROOT_ID))
        return;
    store.putNode(ROOT_TYPE, ROOT_ID, { title: ROOT_ID, body: ROOT_SEED_BODY }, [ROOT_TYPE]);
}
export function readRootBody(store) {
    return store.getNode(ROOT_TYPE, ROOT_ID)?.body ?? "";
}
export function isRoot(node) {
    return node.type === ROOT_TYPE && node.id === ROOT_ID;
}
// Counts structural rot as a diagnostic only — never repairs it. Orphans are non-root nodes
// with no inbound or outbound edges, the typed-graph analogue of the wiki's orphan [[link]]
// count. (The SDK already refuses edges to missing endpoints, so dangling edges cannot form.)
// Each edge has exactly one source, so summing outbound degree counts every edge once.
export function countGraphRot(store) {
    const nodes = store.listNodes();
    let edges = 0;
    let orphans = 0;
    for (const node of nodes) {
        const ref = { type: node.type, id: node.id };
        const outbound = store.outboundEdges(ref).length;
        const inbound = store.inboundEdges(ref).length;
        edges += outbound;
        if (!isRoot(node) && outbound === 0 && inbound === 0)
            orphans += 1;
    }
    return { nodes: nodes.length, edges, orphan_nodes: orphans };
}
export function nodeSummaryLine(node) {
    return `${node.type}/${node.id} — ${node.title}`;
}
