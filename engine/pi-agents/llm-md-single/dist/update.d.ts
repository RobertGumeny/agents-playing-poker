export declare const NOTES_FILENAME = "notes.md";
export declare const UPDATE_SYSTEM_PROMPT: string;
export interface NotesUpdateOptions {
    memoryDir: string;
    handNumber: number;
    handSummary: string;
    cwd?: string;
    model?: string;
    thinkingLevel?: string;
}
export declare function buildUpdatePrompt(currentNotes: string, handSummary: string): string;
export declare function readNotes(memoryDir: string | undefined): Promise<string>;
export declare function runNotesUpdate(options: NotesUpdateOptions): Promise<void>;
