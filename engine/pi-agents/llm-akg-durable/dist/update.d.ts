import { type StoreProvider } from "./tools.js";
export declare const DURABLE_UPDATE_SYSTEM_PROMPT: string;
export interface DurableUpdateOptions {
    memoryDir: string;
    handNumber: number;
    handSummary: string;
    getStore: StoreProvider;
    cwd?: string;
    model?: string;
    thinkingLevel?: string;
}
export declare function buildDurableUpdatePrompt(rootBody: string, handSummary: string): string;
export declare function runDurableUpdate(options: DurableUpdateOptions): Promise<void>;
