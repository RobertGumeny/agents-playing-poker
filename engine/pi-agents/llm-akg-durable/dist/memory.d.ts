import { type Node, type NodeRef, type Store } from "akg-ts";
import type { CompletedHandContext, DecisionContext, MemoryPolicy, PromptAugmentation } from "@agent-poker/pi-agent-shared";
export interface OpponentMeta {
    hands_played: number;
    vpip: number;
    pfr: number;
    fold_to_bet: number;
    aggr_preflop: number;
    aggr_flop: number;
    aggr_turn: number;
    aggr_river: number;
    cbet_opportunities: number;
    cbet_folds: number;
    three_bet_count: number;
    river_bet_count: number;
    river_bet_folds: number;
    showdown_count: number;
    showdown_win: number;
}
interface HandFeatures {
    villain_vpip: boolean;
    villain_pfr: boolean;
    fold_to_bet: boolean;
    aggr_preflop: boolean;
    aggr_flop: boolean;
    aggr_turn: boolean;
    aggr_river: boolean;
    cbet_opportunity: boolean;
    cbet_fold: boolean;
    three_bet: boolean;
    river_bet: boolean;
    river_bet_opportunity: boolean;
    river_bet_fold: boolean;
    villain_fold: boolean;
    hero_fold: boolean;
    showdown_villain: boolean;
    showdown_win: boolean;
    aggressive_action_count: number;
    final_pot: number;
}
interface PatternSnapshot {
    slug: PatternSlug;
    title: string;
    body: string;
    count: number;
    opportunities: number;
    supportedBy: NodeRef[];
}
type PatternSlug = "folds-to-cbet" | "3bet-tendency" | "river-aggressor" | "folds-to-river-bet" | "calls-wide";
export declare class AkgDurableMemoryPolicy implements MemoryPolicy {
    private store;
    private storePath;
    private serverMemoryDir;
    get memoryDir(): string | undefined;
    beforeDecision(context: DecisionContext): Promise<PromptAugmentation>;
    afterHandEnd(context: CompletedHandContext): Promise<void>;
    getStore(memoryDir?: string | undefined): Promise<Store | null>;
}
export declare function writeDurableMemory(store: Store, context: CompletedHandContext): Promise<void>;
export declare function putHand(store: Store, context: CompletedHandContext): NodeRef;
export declare function rebuildOpponent(store: Store): void;
export declare function rebuildPatterns(store: Store, _latestHand: NodeRef): void;
export declare function computePatternSnapshots(hands: Node[]): Map<PatternSlug, PatternSnapshot>;
export declare function deriveHandFeatures(context: CompletedHandContext): HandFeatures;
export {};
