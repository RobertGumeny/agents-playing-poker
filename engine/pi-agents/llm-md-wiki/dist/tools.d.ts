import { type ToolDefinition } from "@earendil-works/pi-coding-agent";
export type WikiDirProvider = () => string | null;
export declare function createReadTools(getWikiDir: WikiDirProvider): ToolDefinition[];
export declare function createWriteTool(getWikiDir: WikiDirProvider): ToolDefinition;
