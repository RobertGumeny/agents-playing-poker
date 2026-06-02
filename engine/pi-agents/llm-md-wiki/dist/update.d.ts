export declare const WIKI_UPDATE_SYSTEM_PROMPT: string;
export interface WikiUpdateOptions {
    memoryDir: string;
    handNumber: number;
    handSummary: string;
    cwd?: string;
    model?: string;
    thinkingLevel?: string;
}
export declare function buildWikiUpdatePrompt(pages: string[], rootContent: string, handSummary: string): string;
export declare function runWikiUpdate(options: WikiUpdateOptions): Promise<void>;
