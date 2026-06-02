import { mkdir, readdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
export const WIKI_SUBDIR = "wiki";
export const ROOT_PAGE = "villain";
const ROOT_SEED = `# villain\n\n(no reads yet)\n`;
export function wikiDir(memoryDir) {
    return path.join(memoryDir, WIKI_SUBDIR);
}
// Normalizes a model-provided page name to an absolute .md path, or null if it would
// escape the wiki directory. Links and page names are the model's to choose, so this only
// guards against path traversal — it does not validate or repair the slug otherwise.
export function resolvePagePath(dir, slug) {
    const clean = slug.trim().replace(/\.md$/i, "").replace(/^[/\\]+/, "");
    if (clean.length === 0)
        return null;
    const full = path.resolve(dir, `${clean}.md`);
    const rel = path.relative(dir, full);
    if (rel.startsWith("..") || path.isAbsolute(rel))
        return null;
    return full;
}
export function pageSlug(dir, fullPath) {
    return path.relative(dir, fullPath).replace(/\\/g, "/").replace(/\.md$/i, "");
}
export async function listPages(dir) {
    const slugs = [];
    async function walk(current) {
        const entries = await readdir(current, { withFileTypes: true }).catch(() => []);
        for (const entry of entries) {
            const child = path.join(current, entry.name);
            if (entry.isDirectory()) {
                await walk(child);
            }
            else if (entry.isFile() && entry.name.toLowerCase().endsWith(".md")) {
                slugs.push(pageSlug(dir, child));
            }
        }
    }
    await walk(dir);
    return slugs.sort();
}
export async function readPage(dir, slug) {
    const full = resolvePagePath(dir, slug);
    if (!full)
        return { found: false, page: slug };
    try {
        return { found: true, page: pageSlug(dir, full), content: await readFile(full, "utf8") };
    }
    catch {
        return { found: false, page: slug };
    }
}
export async function writePage(dir, slug, content) {
    const full = resolvePagePath(dir, slug);
    if (!full)
        return null;
    await mkdir(path.dirname(full), { recursive: true });
    await writeFile(full, content.endsWith("\n") ? content : `${content}\n`, "utf8");
    return pageSlug(dir, full);
}
export async function ensureRootPage(dir) {
    const root = await readPage(dir, ROOT_PAGE);
    if (!root.found) {
        await writePage(dir, ROOT_PAGE, ROOT_SEED);
    }
}
export function extractWikiLinks(text) {
    const links = [];
    const pattern = /\[\[([^\]]+)\]\]/g;
    let match;
    while ((match = pattern.exec(text)) !== null) {
        links.push(match[1].trim());
    }
    return links;
}
