import { Type } from "typebox";
import { defineTool, type ToolDefinition } from "@earendil-works/pi-coding-agent";
import type { Node, Store } from "akg-ts";

const OPPONENT_TYPE = "opponent";
const OPPONENT_ID = "villain";
const PATTERN_TYPE = "pattern";
const HAND_TYPE = "hand";

export type StoreProvider = () => Promise<Store | null>;

export function createQueryTools(getStore: StoreProvider): ToolDefinition[] {
  return [
    defineTool({
      name: "akg_get_opponent",
      label: "AKG Opponent",
      description: "Return the villain opponent profile node with title, body, and meta stats.",
      parameters: Type.Object({}),
      async execute() {
        const store = await getStore();
        const node = store?.getNode(OPPONENT_TYPE, OPPONENT_ID) ?? null;
        const result = node
          ? serializeNode(node)
          : { id: OPPONENT_ID, title: "villain", body: null, meta: null, tags: [OPPONENT_TYPE] };
        return jsonResult(result);
      },
    }),
    defineTool({
      name: "akg_list_patterns",
      label: "AKG Patterns",
      description: "List all opponent pattern nodes with support counts and opportunities.",
      parameters: Type.Object({}),
      async execute() {
        const store = await getStore();
        const result = (store?.listNodesByTag(PATTERN_TYPE) ?? []).map((node) => ({
          id: node.id,
          title: node.title,
          body: node.body,
          meta: {
            count: asNumber(node.meta.count),
            opportunities: asNumber(node.meta.opportunities),
          },
        }));
        return jsonResult(result);
      },
    }),
    defineTool({
      name: "akg_get_pattern",
      label: "AKG Pattern",
      description: "Return a specific pattern node and its supporting hand ids.",
      parameters: Type.Object({
        slug: Type.String({ description: "Pattern id, for example folds-to-cbet" }),
      }),
      async execute(_toolCallId, params) {
        const store = await getStore();
        const node = store?.getNode(PATTERN_TYPE, params.slug) ?? null;
        const result = node
          ? {
              ...serializeNode(node),
              hand_ids: (store?.outboundEdges({ type: PATTERN_TYPE, id: params.slug }, "supported_by") ?? []).map((edge) => edge.to.id),
            }
          : null;
        return jsonResult(result);
      },
    }),
    defineTool({
      name: "akg_list_hands",
      label: "AKG Hands",
      description: "List recent hand summaries, optionally filtered by tag.",
      parameters: Type.Object({
        tag: Type.Optional(Type.String({ description: "Optional hand tag filter" })),
        limit: Type.Optional(Type.Number({ description: "Maximum number of hands to return" })),
      }),
      async execute(_toolCallId, params) {
        const store = await getStore();
        const limit = normalizeLimit(params.limit);
        const hands = (store?.listNodesByTag(HAND_TYPE) ?? [])
          .filter((node) => !params.tag || node.tags.includes(params.tag))
          .sort((left, right) => asNumber(right.meta.hand_number) - asNumber(left.meta.hand_number))
          .slice(0, limit)
          .map((node) => ({
            id: node.id,
            hand_number: asNumber(node.meta.hand_number),
            body: node.body,
            tags: [...node.tags],
          }));
        return jsonResult(hands);
      },
    }),
    defineTool({
      name: "akg_get_hand",
      label: "AKG Hand",
      description: "Return the full stored hand record for a specific hand number.",
      parameters: Type.Object({
        hand_number: Type.Number({ description: "Completed hand number" }),
      }),
      async execute(_toolCallId, params) {
        const store = await getStore();
        const node = findHandByNumber(store, params.hand_number);
        return jsonResult(node ? serializeNode(node) : null);
      },
    }),
  ];
}

function findHandByNumber(store: Store | null, handNumber: number): Node | null {
  if (!store) return null;
  return store.listNodesByTag(HAND_TYPE).find((node) => asNumber(node.meta.hand_number) === handNumber) ?? null;
}

function serializeNode(node: Node) {
  return {
    id: node.id,
    title: node.title,
    body: node.body.length > 0 ? node.body : null,
    meta: { ...node.meta },
    tags: [...node.tags],
  };
}

function normalizeLimit(value: number | undefined): number {
  if (value === undefined || !Number.isFinite(value)) return 10;
  const normalized = Math.trunc(value);
  if (normalized <= 0) return 10;
  return normalized;
}

function asNumber(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function jsonResult(details: unknown) {
  return {
    content: [{ type: "text" as const, text: JSON.stringify(details, null, 2) }],
    details,
  };
}
