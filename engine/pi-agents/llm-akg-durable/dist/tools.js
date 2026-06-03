import { Type } from "typebox";
import { defineTool } from "@earendil-works/pi-coding-agent";
// Generic graph readers, shared by the decision session and the update session. The agent
// authors its own node types, ids, and edge relations, so the read surface is deliberately
// schema-agnostic: list summaries, then read one node and follow its edges.
export function createReadTools(getStore) {
    return [
        defineTool({
            name: "akg_list_nodes",
            label: "AKG Nodes",
            description: "List node summaries (type, id, title, tags) in the opponent graph. The node opponent/villain is the root index. Optionally filter by type or tag and cap the count.",
            parameters: Type.Object({
                type: Type.Optional(Type.String({ description: "Optional node type filter, for example opponent or pattern" })),
                tag: Type.Optional(Type.String({ description: "Optional tag filter" })),
                limit: Type.Optional(Type.Number({ description: "Maximum number of nodes to return" })),
            }),
            async execute(_toolCallId, params) {
                const store = await getStore();
                if (!store)
                    return jsonResult({ nodes: [] });
                const all = params.type ? store.listNodes(params.type) : store.listNodes();
                const nodes = (params.tag ? all.filter((node) => node.tags.includes(params.tag)) : all)
                    .slice(0, normalizeLimit(params.limit))
                    .map(summarizeNode);
                return jsonResult({ nodes });
            },
        }),
        defineTool({
            name: "akg_get_node",
            label: "AKG Node",
            description: "Return a node's title, body, meta, tags, plus its outbound and inbound edges. Follow an edge by reading its other endpoint. Returns found:false for a missing node.",
            parameters: Type.Object({
                type: Type.String({ description: "Node type, for example opponent" }),
                id: Type.String({ description: "Node id, for example villain" }),
            }),
            async execute(_toolCallId, params) {
                const store = await getStore();
                const node = store?.getNode(params.type, params.id) ?? null;
                if (!store || !node)
                    return jsonResult({ found: false, type: params.type, id: params.id });
                const ref = { type: params.type, id: params.id };
                return jsonResult({
                    found: true,
                    ...serializeNode(node),
                    outbound_edges: store.outboundEdges(ref).map(serializeEdge),
                    inbound_edges: store.inboundEdges(ref).map(serializeEdge),
                });
            },
        }),
    ];
}
// Write tools registered only in the post-hand update session. Open vocabulary: the model
// chooses every type/id/title/body/meta/tag and every edge relation. No delete — stale reads
// are retired by overwriting a node body or authoring a superseding edge, symmetric with the
// wiki agent's "does not delete pages" policy.
export function createWriteTools(getStore) {
    return [
        defineTool({
            name: "akg_put_node",
            label: "AKG Put Node",
            description: "Create or replace a node. You choose the type, id, title, body, meta, and tags. Reuse an existing (type, id) to update it in place. Does not delete nodes.",
            parameters: Type.Object({
                type: Type.String({ description: "Node type you choose, lowercase a-z 0-9 underscore, for example pattern or read" }),
                id: Type.String({ description: "Node id you choose, unique within the type, no colons, up to 64 chars" }),
                title: Type.String({ description: "Human-readable title" }),
                body: Type.Optional(Type.String({ description: "Free-form body text; keep your own tallies current here" })),
                meta: Type.Optional(Type.Record(Type.String(), Type.Unknown(), { description: "Optional structured fields you choose" })),
                tags: Type.Optional(Type.Array(Type.String(), { description: "Optional tags, lowercase a-z 0-9 underscore" })),
            }),
            async execute(_toolCallId, params) {
                const store = await getStore();
                if (!store)
                    return jsonResult({ written: false, type: params.type, id: params.id, error: "no store available" });
                try {
                    const ref = store.putNode(params.type, params.id, { title: params.title, body: params.body, meta: params.meta }, params.tags ?? []);
                    return jsonResult({ written: true, type: ref.type, id: ref.id });
                }
                catch (error) {
                    return jsonResult({ written: false, type: params.type, id: params.id, error: describeError(error) });
                }
            },
        }),
        defineTool({
            name: "akg_put_edge",
            label: "AKG Put Edge",
            description: "Create or replace a directed edge between two existing nodes. You choose the relation. Both endpoint nodes must already exist (create them with akg_put_node first). Does not delete edges.",
            parameters: Type.Object({
                from_type: Type.String({ description: "Source node type" }),
                from_id: Type.String({ description: "Source node id" }),
                relation: Type.String({ description: "Relation name you choose, lowercase a-z 0-9 underscore, for example supported_by or contradicts" }),
                to_type: Type.String({ description: "Target node type" }),
                to_id: Type.String({ description: "Target node id" }),
                strength: Type.Optional(Type.Number({ description: "Optional 0..1 salience, defaults to 0.5" })),
                confidence: Type.Optional(Type.Number({ description: "Optional 0..1 certainty; omit to leave unjudged" })),
                meta: Type.Optional(Type.Record(Type.String(), Type.Unknown(), { description: "Optional structured edge fields" })),
            }),
            async execute(_toolCallId, params) {
                const store = await getStore();
                if (!store)
                    return jsonResult({ written: false, error: "no store available" });
                try {
                    store.putEdge({ type: params.from_type, id: params.from_id }, params.relation, { type: params.to_type, id: params.to_id }, { strength: params.strength, confidence: params.confidence, meta: params.meta });
                    return jsonResult({
                        written: true,
                        from: { type: params.from_type, id: params.from_id },
                        relation: params.relation,
                        to: { type: params.to_type, id: params.to_id },
                    });
                }
                catch (error) {
                    return jsonResult({ written: false, error: describeError(error) });
                }
            },
        }),
    ];
}
function summarizeNode(node) {
    return { type: node.type, id: node.id, title: node.title, tags: [...node.tags] };
}
function serializeNode(node) {
    return {
        type: node.type,
        id: node.id,
        title: node.title,
        body: node.body.length > 0 ? node.body : null,
        meta: { ...node.meta },
        tags: [...node.tags],
    };
}
function serializeEdge(edge) {
    return {
        relation: edge.relation,
        from: { type: edge.from.type, id: edge.from.id },
        to: { type: edge.to.type, id: edge.to.id },
        strength: edge.strength,
        confidence: edge.confidence,
        meta: { ...edge.meta },
    };
}
function normalizeLimit(value) {
    if (value === undefined || !Number.isFinite(value))
        return 100;
    const normalized = Math.trunc(value);
    if (normalized <= 0)
        return 100;
    return normalized;
}
function describeError(error) {
    return error instanceof Error ? error.message : String(error);
}
function jsonResult(details) {
    return {
        content: [{ type: "text", text: JSON.stringify(details, null, 2) }],
        details,
    };
}
