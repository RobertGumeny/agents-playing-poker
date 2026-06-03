import { type ToolDefinition } from "@earendil-works/pi-coding-agent";
import type { Store } from "akg-ts";
export type StoreProvider = () => Promise<Store | null>;
export declare function createReadTools(getStore: StoreProvider): ToolDefinition[];
export declare function createWriteTools(getStore: StoreProvider): ToolDefinition[];
