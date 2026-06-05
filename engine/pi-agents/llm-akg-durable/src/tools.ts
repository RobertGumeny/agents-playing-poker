import { Type } from "typebox";
import { defineTool, type ToolDefinition } from "@earendil-works/pi-coding-agent";
import type { Edge, Node, Store } from "akg-ts";

export type StoreProvider = () => Promise<Store | null>;

// Generic graph readers, shared by the decision session and the update session. The agent
// authors its own node types, ids, and edge relations, so the read surface is deliberately
// schema-agnostic: list summaries, then read one node and follow its edges.
export function createReadTools(getStore: StoreProvider): ToolDefinition[] {
  return [
    defineTool({
      name: "akg_list_nodes",
      label: "AKG Nodes",
      description:
        "List node summaries (type, id, title, tags) in the opponent graph. The node opponent/villain is the root index. Optionally filter by type or tag and cap the count.",
      parameters: Type.Object({
        type: Type.Optional(Type.String({ description: "Optional node type filter, for example opponent or pattern" })),
        tag: Type.Optional(Type.String({ description: "Optional tag filter" })),
        limit: Type.Optional(Type.Number({ description: "Maximum number of nodes to return" })),
      }),
      async execute(_toolCallId, params) {
        const store = await getStore();
        if (!store) return jsonResult({ nodes: [] });
        const all = params.type ? store.listNodes(params.type) : store.listNodes();
        const nodes = (params.tag ? all.filter((node) => node.tags.includes(params.tag as string)) : all)
          .slice(0, normalizeLimit(params.limit))
          .map(summarizeNode);
        return jsonResult({ nodes });
      },
    }),
    defineTool({
      name: "akg_get_node",
      label: "AKG Node",
      description:
        "Return a node's title, body, meta, tags, plus its outbound and inbound edges. Follow an edge by reading its other endpoint. Returns found:false for a missing node. To read several nodes at once, prefer akg_get_nodes.",
      parameters: Type.Object({
        type: Type.String({ description: "Node type, for example opponent" }),
        id: Type.String({ description: "Node id, for example villain" }),
      }),
      async execute(_toolCallId, params) {
        const store = await getStore();
        return jsonResult(readNodeResult(store, params.type, params.id));
      },
    }),
    defineTool({
      name: "akg_get_nodes",
      label: "AKG Nodes (batch)",
      description:
        "Read several nodes in ONE call. Prefer this over multiple akg_get_node calls when you need more than one node, so a decision stays a single turn. Returns each node's title, body, meta, tags, and edges; a missing node yields found:false.",
      parameters: Type.Object({
        refs: Type.Array(
          Type.Object({
            type: Type.String({ description: "Node type, for example pattern" }),
            id: Type.String({ description: "Node id" }),
          }),
          { description: "Node refs to read in one call" },
        ),
      }),
      async execute(_toolCallId, params) {
        const store = await getStore();
        const nodes = (params.refs ?? []).map((ref) => readNodeResult(store, ref.type, ref.id));
        return jsonResult({ nodes });
      },
    }),
  ];
}

// Shared by akg_get_node and akg_get_nodes so the single- and batch-read surfaces return
// identical per-node shapes. Missing node (or no store) yields {found:false,type,id}.
function readNodeResult(store: Store | null, type: string, id: string) {
  const node = store?.getNode(type, id) ?? null;
  if (!store || !node) return { found: false, type, id };
  const ref = { type, id };
  return {
    found: true,
    ...serializeNode(node),
    outbound_edges: store.outboundEdges(ref).map(serializeEdge),
    inbound_edges: store.inboundEdges(ref).map(serializeEdge),
  };
}

// Node/edge parameter shapes, shared by the single-write tools and the batched akg_apply tool
// so the open-vocabulary contract is described in exactly one place.
const NODE_PARAMS = Type.Object({
  type: Type.String({ description: "Node type you choose, lowercase a-z 0-9 underscore, for example pattern or read" }),
  id: Type.String({ description: "Node id you choose, unique within the type, no colons, up to 64 chars" }),
  title: Type.String({ description: "Human-readable title" }),
  body: Type.Optional(Type.String({ description: "Free-form body text; keep your own tallies current here" })),
  meta: Type.Optional(Type.Record(Type.String(), Type.Unknown(), { description: "Optional structured fields you choose" })),
  tags: Type.Optional(Type.Array(Type.String(), { description: "Optional tags, lowercase a-z 0-9 underscore" })),
});

const EDGE_PARAMS = Type.Object({
  from_type: Type.String({ description: "Source node type" }),
  from_id: Type.String({ description: "Source node id" }),
  relation: Type.String({ description: "Relation name you choose, lowercase a-z 0-9 underscore, for example supported_by or contradicts" }),
  to_type: Type.String({ description: "Target node type" }),
  to_id: Type.String({ description: "Target node id" }),
  strength: Type.Optional(Type.Number({ description: "Optional 0..1 salience, defaults to 0.5" })),
  confidence: Type.Optional(Type.Number({ description: "Optional 0..1 certainty; omit to leave unjudged" })),
  meta: Type.Optional(Type.Record(Type.String(), Type.Unknown(), { description: "Optional structured edge fields" })),
});

type NodeParams = {
  type: string;
  id: string;
  title: string;
  body?: string;
  meta?: Record<string, unknown>;
  tags?: string[];
};

type EdgeParams = {
  from_type: string;
  from_id: string;
  relation: string;
  to_type: string;
  to_id: string;
  strength?: number;
  confidence?: number;
  meta?: Record<string, unknown>;
};

function upsertNode(store: Store, params: NodeParams) {
  try {
    const ref = store.putNode(
      params.type,
      params.id,
      { title: params.title, body: params.body, meta: params.meta },
      params.tags ?? [],
    );
    return { written: true, type: ref.type, id: ref.id };
  } catch (error) {
    return { written: false, type: params.type, id: params.id, error: describeError(error) };
  }
}

function upsertEdge(store: Store, params: EdgeParams) {
  try {
    store.putEdge(
      { type: params.from_type, id: params.from_id },
      params.relation,
      { type: params.to_type, id: params.to_id },
      { strength: params.strength, confidence: params.confidence, meta: params.meta },
    );
    return {
      written: true,
      from: { type: params.from_type, id: params.from_id },
      relation: params.relation,
      to: { type: params.to_type, id: params.to_id },
    };
  } catch (error) {
    return { written: false, from: { type: params.from_type, id: params.from_id }, relation: params.relation, to: { type: params.to_type, id: params.to_id }, error: describeError(error) };
  }
}

// Write tools registered only in the post-hand update session. Open vocabulary: the model
// chooses every type/id/title/body/meta/tag and every edge relation. No delete — stale reads
// are retired by overwriting a node body or authoring a superseding edge, symmetric with the
// wiki agent's "does not delete pages" policy. akg_apply batches a whole update into one call
// so the update session stays a single turn instead of one round-trip per write.
export function createWriteTools(getStore: StoreProvider): ToolDefinition[] {
  return [
    defineTool({
      name: "akg_apply",
      label: "AKG Apply",
      description:
        "Apply ALL of this hand's graph changes in one call: every node in `nodes` is upserted first, then every edge in `edges`, so edge endpoints exist. Prefer this single batched call over many separate akg_put_node/akg_put_edge calls — it keeps the update to one turn. Reuse an existing (type, id) to update in place. Does not delete.",
      parameters: Type.Object({
        nodes: Type.Optional(Type.Array(NODE_PARAMS, { description: "Nodes to create or update, applied before edges" })),
        edges: Type.Optional(Type.Array(EDGE_PARAMS, { description: "Edges to create or update; both endpoints must exist (here or already in the graph)" })),
      }),
      async execute(_toolCallId, params) {
        const store = await getStore();
        if (!store) return jsonResult({ applied: false, error: "no store available" });
        const nodes = (params.nodes ?? []).map((node) => upsertNode(store, node));
        const edges = (params.edges ?? []).map((edge) => upsertEdge(store, edge));
        return jsonResult({ applied: true, nodes, edges });
      },
    }),
    defineTool({
      name: "akg_put_node",
      label: "AKG Put Node",
      description:
        "Create or replace a single node. Prefer akg_apply to batch all writes; use this only for a one-off. You choose the type, id, title, body, meta, and tags. Reuse an existing (type, id) to update it in place. Does not delete nodes.",
      parameters: NODE_PARAMS,
      async execute(_toolCallId, params) {
        const store = await getStore();
        if (!store) return jsonResult({ written: false, type: params.type, id: params.id, error: "no store available" });
        return jsonResult(upsertNode(store, params));
      },
    }),
    defineTool({
      name: "akg_put_edge",
      label: "AKG Put Edge",
      description:
        "Create or replace a single directed edge between two existing nodes. Prefer akg_apply to batch all writes; use this only for a one-off. Both endpoint nodes must already exist. Does not delete edges.",
      parameters: EDGE_PARAMS,
      async execute(_toolCallId, params) {
        const store = await getStore();
        if (!store) return jsonResult({ written: false, error: "no store available" });
        return jsonResult(upsertEdge(store, params));
      },
    }),
  ];
}

function summarizeNode(node: Node) {
  return { type: node.type, id: node.id, title: node.title, tags: [...node.tags] };
}

function serializeNode(node: Node) {
  return {
    type: node.type,
    id: node.id,
    title: node.title,
    body: node.body.length > 0 ? node.body : null,
    meta: { ...node.meta },
    tags: [...node.tags],
  };
}

function serializeEdge(edge: Edge) {
  return {
    relation: edge.relation,
    from: { type: edge.from.type, id: edge.from.id },
    to: { type: edge.to.type, id: edge.to.id },
    strength: edge.strength,
    confidence: edge.confidence,
    meta: { ...edge.meta },
  };
}

function normalizeLimit(value: number | undefined): number {
  if (value === undefined || !Number.isFinite(value)) return 100;
  const normalized = Math.trunc(value);
  if (normalized <= 0) return 100;
  return normalized;
}

function describeError(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function jsonResult(details: unknown) {
  return {
    content: [{ type: "text" as const, text: JSON.stringify(details, null, 2) }],
    details,
  };
}
