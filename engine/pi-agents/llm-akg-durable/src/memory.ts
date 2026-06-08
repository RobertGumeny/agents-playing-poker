// Durable AKG memory policy: injects the opponent graph's root index before each decision,
// and on hand end hands the completed hand to the model-driven post-hand graph update.

import { type Store } from "akg-ts";

import type { CompletedHandContext, DecisionContext, MemoryPolicy, PromptAugmentation } from "@agent-poker/pi-agent-shared";
import { formatCompletedHand } from "@agent-poker/pi-agent-shared";

import { ensureRootNode, openStore, readRootBody, ROOT_ID, ROOT_TYPE } from "./graph.js";
import { runDurableUpdate } from "./update.js";

const INDEX_HEADER = `Your opponent summary (node ${ROOT_TYPE}/${ROOT_ID}) follows — use it directly. Call akg_list_nodes / akg_get_node / akg_get_nodes to drill into connected nodes only if you need more than it covers:`;
const NO_READS_SECTION = `Your opponent summary (${ROOT_TYPE}/${ROOT_ID}): no reads yet. akg_list_nodes, akg_get_node, and akg_get_nodes become useful once nodes exist.`;

export class AkgDurableMemoryPolicy implements MemoryPolicy {
  private store: Store | null = null;
  private storeMemoryDir: string | null = null;
  private serverMemoryDir: string | undefined;

  get memoryDir(): string | undefined {
    return this.serverMemoryDir;
  }

  // Before each decision, inject the opponent index node's body as a system prompt section. This keeps it front and center for the model, and encourages the model to use the AKG tools to explore it further if needed — instead of trying to keep the whole graph in its head for every single hand.
  async beforeDecision(context: DecisionContext): Promise<PromptAugmentation> {
    this.serverMemoryDir = context.state.session?.memoryDir;
    const store = await this.getStore(this.serverMemoryDir);
    if (!store) {
      return { sections: [NO_READS_SECTION] };
    }

    ensureRootNode(store);
    await store.commit();
    const body = readRootBody(store).trim();
    if (body.length === 0) {
      return { sections: [NO_READS_SECTION] };
    }
    return { sections: [INDEX_HEADER, body] };
  }

  // After each hand, run a durable update that hands off the completed hand data, which the agent can process and use to update their opponent profile. This keeps the graph up to date with the latest tendencies for the opponent, without needing to wait for the (potentially slow) graph update to complete during the critical decision-making phase of the next hand.
  async afterHandEnd(context: CompletedHandContext): Promise<void> {
    const memoryDir = context.state.session?.memoryDir ?? this.serverMemoryDir;
    this.serverMemoryDir = memoryDir;
    if (!memoryDir) return;

    await runDurableUpdate({
      memoryDir,
      handNumber: context.handNumber,
      handSummary: formatCompletedHand(context),
      getStore: () => this.getStore(memoryDir),
      model: process.env.PI_POKER_MODEL,
      thinkingLevel: process.env.PI_POKER_THINKING_LEVEL,
    });
  }

  async getStore(memoryDir = this.serverMemoryDir): Promise<Store | null> {
    if (!memoryDir) return null;
    if (this.store && this.storeMemoryDir === memoryDir) {
      return this.store;
    }
    this.store = await openStore(memoryDir);
    this.storeMemoryDir = memoryDir;
    return this.store;
  }
}
