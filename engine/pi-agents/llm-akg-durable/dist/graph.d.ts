import { type Store } from "akg-ts";
export declare const STORE_FILE = "memory.akg";
export declare const ROOT_TYPE = "opponent";
export declare const ROOT_ID = "villain";
export declare function openStore(memoryDir: string): Promise<Store>;
export declare function ensureRootNode(store: Store): void;
export declare function readRootBody(store: Store): string;
export declare function isRoot(node: {
    type: string;
    id: string;
}): boolean;
export interface GraphRot {
    nodes: number;
    edges: number;
    orphan_nodes: number;
}
export declare function countGraphRot(store: Store): GraphRot;
