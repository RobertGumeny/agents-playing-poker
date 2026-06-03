import { type Store } from "akg-ts";

import type { CompletedHandContext, DecisionContext, MemoryPolicy, PromptAugmentation } from "@agent-poker/pi-agent-shared";
import { formatCompletedHand } from "@agent-poker/pi-agent-shared";

import { ensureRootNode, openStore, readRootBody, ROOT_ID, ROOT_TYPE } from "./graph.js";
import { runDurableUpdate } from "./update.js";

const INDEX_HEADER = `Your opponent graph index is node ${ROOT_TYPE}/${ROOT_ID}. Call akg_list_nodes and akg_get_node to follow edges to deeper nodes before deciding:`;
const NO_READS_SECTION = `Your opponent graph index (${ROOT_TYPE}/${ROOT_ID}): no reads yet. akg_list_nodes and akg_get_node are available once nodes exist.`;

export class AkgDurableMemoryPolicy implements MemoryPolicy {
  private store: Store | null = null;
  private storeMemoryDir: string | null = null;
  private serverMemoryDir: string | undefined;

  get memoryDir(): string | undefined {
    return this.serverMemoryDir;
  }

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
