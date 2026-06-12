// Linked-markdown ("wiki") memory policy: injects only the root index page before each
// decision, and on hand end hands the completed hand to the model-driven post-hand page update.
import { formatCompletedHand } from "@agent-poker/pi-agent-shared";
import { ensureRootPage, readPage, ROOT_PAGE, wikiDir } from "./pages.js";
import { runWikiUpdate } from "./update.js";
const INDEX_HEADER = `Your opponent wiki index (${ROOT_PAGE}.md) — index only; pattern details are in linked pages. Use md_read_page to follow [[links]] before deciding:`;
const NO_READS_SECTION = `Your opponent wiki index (${ROOT_PAGE}.md): no reads yet. md_list_pages and md_read_page are available once pages exist.`;
export class MarkdownWikiMemoryPolicy {
    serverMemoryDir;
    get memoryDir() {
        return this.serverMemoryDir;
    }
    getWikiDir() {
        return this.serverMemoryDir ? wikiDir(this.serverMemoryDir) : null;
    }
    async beforeDecision(context) {
        this.serverMemoryDir = context.state.session?.memoryDir;
        if (!this.serverMemoryDir) {
            return { sections: [NO_READS_SECTION] };
        }
        const dir = wikiDir(this.serverMemoryDir);
        await ensureRootPage(dir);
        const root = await readPage(dir, ROOT_PAGE);
        if (!root.found || !root.content || root.content.trim().length === 0) {
            return { sections: [NO_READS_SECTION] };
        }
        return { sections: [INDEX_HEADER, root.content.trim()] };
    }
    async afterHandEnd(context) {
        const memoryDir = context.state.session?.memoryDir ?? this.serverMemoryDir;
        this.serverMemoryDir = memoryDir;
        if (!memoryDir)
            return;
        await runWikiUpdate({
            memoryDir,
            handNumber: context.handNumber,
            handSummary: formatCompletedHand(context),
            model: process.env.PI_POKER_MODEL,
            thinkingLevel: process.env.PI_POKER_THINKING_LEVEL,
        });
    }
}
