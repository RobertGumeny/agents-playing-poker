import { formatCompletedHand } from "@agent-poker/pi-agent-shared";
import { ensureRootNode, openStore, readRootBody, ROOT_ID, ROOT_TYPE } from "./graph.js";
import { runDurableUpdate } from "./update.js";
const INDEX_HEADER = `Your opponent summary (node ${ROOT_TYPE}/${ROOT_ID}) follows — use it directly. Only if you need more than it covers, call akg_list_nodes / akg_get_node / akg_get_nodes to drill into connected nodes:`;
const NO_READS_SECTION = `Your opponent summary (${ROOT_TYPE}/${ROOT_ID}): no reads yet. akg_list_nodes, akg_get_node, and akg_get_nodes become useful once nodes exist.`;
export class AkgDurableMemoryPolicy {
    store = null;
    storeMemoryDir = null;
    serverMemoryDir;
    get memoryDir() {
        return this.serverMemoryDir;
    }
    async beforeDecision(context) {
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
    async afterHandEnd(context) {
        const memoryDir = context.state.session?.memoryDir ?? this.serverMemoryDir;
        this.serverMemoryDir = memoryDir;
        if (!memoryDir)
            return;
        await runDurableUpdate({
            memoryDir,
            handNumber: context.handNumber,
            handSummary: formatCompletedHand(context),
            getStore: () => this.getStore(memoryDir),
            model: process.env.PI_POKER_MODEL,
            thinkingLevel: process.env.PI_POKER_THINKING_LEVEL,
        });
    }
    async getStore(memoryDir = this.serverMemoryDir) {
        if (!memoryDir)
            return null;
        if (this.store && this.storeMemoryDir === memoryDir) {
            return this.store;
        }
        this.store = await openStore(memoryDir);
        this.storeMemoryDir = memoryDir;
        return this.store;
    }
}
