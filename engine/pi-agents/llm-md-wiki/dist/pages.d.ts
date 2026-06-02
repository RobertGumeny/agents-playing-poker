export declare const WIKI_SUBDIR = "wiki";
export declare const ROOT_PAGE = "villain";
export declare function wikiDir(memoryDir: string): string;
export declare function resolvePagePath(dir: string, slug: string): string | null;
export declare function pageSlug(dir: string, fullPath: string): string;
export declare function listPages(dir: string): Promise<string[]>;
export interface PageReadResult {
    found: boolean;
    page: string;
    content?: string;
}
export declare function readPage(dir: string, slug: string): Promise<PageReadResult>;
export declare function writePage(dir: string, slug: string, content: string): Promise<string | null>;
export declare function ensureRootPage(dir: string): Promise<void>;
export declare function extractWikiLinks(text: string): string[];
