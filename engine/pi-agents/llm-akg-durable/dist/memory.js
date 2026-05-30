import { join } from "node:path";
import { open } from "akg-ts";
const OPPONENT_TYPE = "opponent";
const OPPONENT_ID = "villain";
const HAND_TYPE = "hand";
const PATTERN_TYPE = "pattern";
const SHOWS_PATTERN = "shows_pattern";
const SUPPORTED_BY = "supported_by";
const DEFAULT_BIG_BLIND = 2;
const PATTERN_CONFIGS = [
    { slug: "folds-to-cbet", title: "Folds to flop c-bets" },
    { slug: "3bet-tendency", title: "Frequent preflop 3-bettor" },
    { slug: "river-aggressor", title: "River aggressor" },
    { slug: "folds-to-river-bet", title: "Folds to river bets" },
    { slug: "calls-wide", title: "Calls wide" },
];
export class AkgDurableMemoryPolicy {
    store = null;
    storePath = null;
    serverMemoryDir;
    get memoryDir() {
        return this.serverMemoryDir;
    }
    async beforeDecision(context) {
        this.serverMemoryDir = context.state.session?.memoryDir;
        const store = await this.getStore(context.state.session?.memoryDir);
        if (!store) {
            return { sections: ["AKG memory is not available (no memory_dir provided)."] };
        }
        return { sections: ["AKG memory is available. Call akg_get_opponent to read the opponent index."] };
    }
    async afterHandEnd(context) {
        const memoryDir = context.state.session?.memoryDir ?? this.serverMemoryDir;
        this.serverMemoryDir = memoryDir;
        const store = await this.getStore(memoryDir);
        if (!store)
            return;
        await writeDurableMemory(store, context);
        await store.commit();
    }
    async getStore(memoryDir = this.serverMemoryDir) {
        if (!memoryDir)
            return null;
        const path = join(memoryDir, "memory.akg");
        if (this.store && this.storePath === path) {
            return this.store;
        }
        this.store = await open(path);
        this.storePath = path;
        return this.store;
    }
}
export async function writeDurableMemory(store, context) {
    const handRef = putHand(store, context);
    rebuildOpponent(store);
    rebuildPatterns(store, handRef);
}
export function putHand(store, context) {
    const features = deriveHandFeatures(context);
    const existing = findHandByNumber(store, context.handNumber);
    const heroResult = context.result.find((entry) => entry.seat === context.heroSeat)?.chips_delta ?? 0;
    const heroPosition = context.dealerSeat === context.heroSeat ? "sb" : "bb";
    const streetReached = lastStreetReached(context);
    const tags = buildHandTags(context, features);
    const ref = store.putNode(HAND_TYPE, existing?.id ?? "", {
        title: `Hand ${context.handNumber}`,
        body: buildHandBody(context, heroPosition, heroResult, streetReached),
        meta: {
            hand_number: context.handNumber,
            hero_position: heroPosition,
            hero_net: heroResult,
            street_reached: streetReached,
            showdown: context.showdownReached,
            villain_vpip: features.villain_vpip,
            villain_pfr: features.villain_pfr,
            fold_to_bet: features.fold_to_bet,
            aggr_preflop: features.aggr_preflop,
            aggr_flop: features.aggr_flop,
            aggr_turn: features.aggr_turn,
            aggr_river: features.aggr_river,
            cbet_opportunity: features.cbet_opportunity,
            cbet_fold: features.cbet_fold,
            three_bet: features.three_bet,
            river_bet: features.river_bet,
            river_bet_opportunity: features.river_bet_opportunity,
            river_bet_fold: features.river_bet_fold,
            showdown_villain: features.showdown_villain,
            showdown_win: features.showdown_win,
            villain_fold: features.villain_fold,
            hero_fold: features.hero_fold,
            aggressive_action_count: features.aggressive_action_count,
            final_pot: features.final_pot,
        },
    }, tags);
    return ref;
}
export function rebuildOpponent(store) {
    const hands = listHandNodes(store);
    const meta = hands.reduce((acc, hand) => {
        acc.hands_played += 1;
        acc.vpip += boolCount(hand.meta.villain_vpip);
        acc.pfr += boolCount(hand.meta.villain_pfr);
        acc.fold_to_bet += boolCount(hand.meta.fold_to_bet);
        acc.aggr_preflop += boolCount(hand.meta.aggr_preflop);
        acc.aggr_flop += boolCount(hand.meta.aggr_flop);
        acc.aggr_turn += boolCount(hand.meta.aggr_turn);
        acc.aggr_river += boolCount(hand.meta.aggr_river);
        acc.cbet_opportunities += boolCount(hand.meta.cbet_opportunity);
        acc.cbet_folds += boolCount(hand.meta.cbet_fold);
        acc.three_bet_count += boolCount(hand.meta.three_bet);
        acc.river_bet_count += boolCount(hand.meta.river_bet);
        acc.river_bet_folds += boolCount(hand.meta.river_bet_fold);
        acc.showdown_count += boolCount(hand.meta.showdown_villain);
        acc.showdown_win += boolCount(hand.meta.showdown_win);
        return acc;
    }, emptyOpponentMeta());
    store.putNode(OPPONENT_TYPE, OPPONENT_ID, {
        title: "villain",
        body: buildOpponentBody(meta),
        meta: meta,
    }, [OPPONENT_TYPE]);
}
export function rebuildPatterns(store, _latestHand) {
    const hands = listHandNodes(store);
    const patternSnapshots = computePatternSnapshots(hands);
    const opponentRef = { type: OPPONENT_TYPE, id: OPPONENT_ID };
    for (const config of PATTERN_CONFIGS) {
        const existing = store.getNode(PATTERN_TYPE, config.slug);
        const snapshot = patternSnapshots.get(config.slug);
        if (!snapshot) {
            if (!existing)
                continue;
            cleanupPattern(store, config.slug);
            continue;
        }
        const patternRef = store.putNode(PATTERN_TYPE, config.slug, {
            title: snapshot.title,
            body: snapshot.body,
            meta: {
                count: snapshot.count,
                opportunities: snapshot.opportunities,
            },
        }, [PATTERN_TYPE]);
        store.putEdge(opponentRef, SHOWS_PATTERN, patternRef, {
            strength: 1,
            confidence: null,
            meta: {
                count: snapshot.count,
                opportunities: snapshot.opportunities,
            },
        });
        const desired = new Set(snapshot.supportedBy.map((ref) => ref.id));
        for (const edge of store.outboundEdges(patternRef, SUPPORTED_BY)) {
            if (!desired.has(edge.to.id)) {
                store.deleteEdge(patternRef, SUPPORTED_BY, edge.to);
            }
        }
        for (const handRef of snapshot.supportedBy) {
            const hand = store.getNode(HAND_TYPE, handRef.id);
            store.putEdge(patternRef, SUPPORTED_BY, handRef, {
                strength: 1,
                confidence: 1,
                meta: {
                    hand_number: asNumber(hand?.meta.hand_number),
                },
            });
        }
    }
}
export function computePatternSnapshots(hands) {
    const bySlug = new Map();
    const cbetSupported = hands.filter((hand) => truthy(hand.meta.cbet_fold));
    if (cbetSupported.length >= 3) {
        const opportunities = hands.filter((hand) => truthy(hand.meta.cbet_opportunity)).length;
        bySlug.set("folds-to-cbet", {
            slug: "folds-to-cbet",
            title: "Folds to flop c-bets",
            body: `Villain has folded to hero flop c-bet ${cbetSupported.length} times across ${opportunities} c-bet opportunities.`,
            count: cbetSupported.length,
            opportunities,
            supportedBy: cbetSupported.map(toRef),
        });
    }
    const threeBetSupported = hands.filter((hand) => truthy(hand.meta.three_bet));
    if (threeBetSupported.length >= 3) {
        bySlug.set("3bet-tendency", {
            slug: "3bet-tendency",
            title: "Frequent preflop 3-bettor",
            body: `Villain has 3-bet preflop in ${threeBetSupported.length} completed hands.`,
            count: threeBetSupported.length,
            opportunities: hands.length,
            supportedBy: threeBetSupported.map(toRef),
        });
    }
    const riverAggSupported = hands.filter((hand) => truthy(hand.meta.river_bet));
    if (riverAggSupported.length >= 3) {
        bySlug.set("river-aggressor", {
            slug: "river-aggressor",
            title: "River aggressor",
            body: `Villain has bet or raised the river in ${riverAggSupported.length} completed hands.`,
            count: riverAggSupported.length,
            opportunities: hands.length,
            supportedBy: riverAggSupported.map(toRef),
        });
    }
    const riverFoldSupported = hands.filter((hand) => truthy(hand.meta.river_bet_fold));
    if (riverFoldSupported.length >= 3) {
        const opportunities = hands.filter((hand) => truthy(hand.meta.river_bet_opportunity)).length;
        bySlug.set("folds-to-river-bet", {
            slug: "folds-to-river-bet",
            title: "Folds to river bets",
            body: `Villain has folded to hero river bets ${riverFoldSupported.length} times across ${opportunities} river bet opportunities.`,
            count: riverFoldSupported.length,
            opportunities,
            supportedBy: riverFoldSupported.map(toRef),
        });
    }
    const foldToBetCount = hands.filter((hand) => truthy(hand.meta.fold_to_bet)).length;
    if (hands.length >= 15 && hands.length > 0 && foldToBetCount / hands.length <= 0.2) {
        bySlug.set("calls-wide", {
            slug: "calls-wide",
            title: "Calls wide",
            body: `Villain has folded to hero bets or raises only ${foldToBetCount} times across ${hands.length} completed hands.`,
            count: hands.length - foldToBetCount,
            opportunities: hands.length,
            supportedBy: hands.filter((hand) => !truthy(hand.meta.fold_to_bet)).map(toRef),
        });
    }
    return bySlug;
}
export function deriveHandFeatures(context) {
    const villainSeat = context.seats.find((seat) => seat.seat !== context.heroSeat)?.seat;
    const villainActions = context.actionHistory.filter((entry) => entry.seat === villainSeat);
    const heroActions = context.actionHistory.filter((entry) => entry.seat === context.heroSeat);
    const preflopVillainActions = villainActions.filter((entry) => entry.street === "preflop");
    const preflopHeroActions = heroActions.filter((entry) => entry.street === "preflop");
    const preflopAggressive = context.actionHistory.filter((entry) => entry.street === "preflop" && isAggressive(entry.action));
    const villainAggressiveByStreet = new Set(villainActions.filter((entry) => isAggressive(entry.action)).map((entry) => entry.street));
    const villainShowdown = context.showdown ? Object.hasOwn(context.showdown, String(villainSeat)) : false;
    const villainWonShowdown = villainShowdown && context.result.some((entry) => entry.seat === villainSeat && entry.chips_delta > 0);
    return {
        villain_vpip: preflopVillainActions.some((entry) => entry.action === "call" || entry.action === "bet" || entry.action === "raise"),
        villain_pfr: preflopVillainActions.some((entry) => isAggressive(entry.action)),
        fold_to_bet: didSeatFoldToHeroBet(context.actionHistory, context.heroSeat, villainSeat),
        aggr_preflop: villainAggressiveByStreet.has("preflop"),
        aggr_flop: villainAggressiveByStreet.has("flop"),
        aggr_turn: villainAggressiveByStreet.has("turn"),
        aggr_river: villainAggressiveByStreet.has("river"),
        cbet_opportunity: heroMadeFlopCBet(preflopHeroActions, heroActions),
        cbet_fold: heroMadeFlopCBet(preflopHeroActions, heroActions) && villainFoldedToHeroStreetAggression(context.actionHistory, context.heroSeat, villainSeat, "flop"),
        three_bet: preflopAggressive.length >= 2 && preflopVillainActions.some((entry) => isAggressive(entry.action)) && context.actionHistory.some((entry) => entry.seat === villainSeat && isAggressive(entry.action) && entry.street === "preflop" && preflopAggressive.findIndex((candidate) => candidate === entry) >= 1),
        river_bet: villainActions.some((entry) => entry.street === "river" && isAggressive(entry.action)),
        river_bet_opportunity: seatFacedHeroStreetAggression(context.actionHistory, context.heroSeat, villainSeat, "river"),
        river_bet_fold: villainFoldedToHeroStreetAggression(context.actionHistory, context.heroSeat, villainSeat, "river"),
        villain_fold: villainActions.some((entry) => entry.action === "fold" || entry.action === "auto_fold"),
        hero_fold: heroActions.some((entry) => entry.action === "fold" || entry.action === "auto_fold"),
        showdown_villain: villainShowdown,
        showdown_win: villainWonShowdown,
        aggressive_action_count: context.actionHistory.filter((entry) => isAggressive(entry.action)).length,
        final_pot: estimateFinalPot(context.actionHistory),
    };
}
function buildHandTags(context, features) {
    const tags = [HAND_TYPE];
    const bb = context.state.session?.match.blinds.bb ?? DEFAULT_BIG_BLIND;
    if (context.showdownReached)
        tags.push("showdown");
    if (features.final_pot >= bb * 50)
        tags.push("big_pot");
    if (features.hero_fold)
        tags.push("hero_fold");
    if (features.villain_fold)
        tags.push("villain_fold");
    if (features.three_bet)
        tags.push("3bet_hand");
    if (features.aggressive_action_count >= 4)
        tags.push("aggressive_hand");
    return tags;
}
function cleanupPattern(store, slug) {
    const patternRef = { type: PATTERN_TYPE, id: slug };
    for (const edge of store.inboundEdges(patternRef)) {
        store.deleteEdge(edge.from, edge.relation, patternRef);
    }
    for (const edge of store.outboundEdges(patternRef)) {
        store.deleteEdge(patternRef, edge.relation, edge.to);
    }
    store.deleteNode(PATTERN_TYPE, slug);
}
function listHandNodes(store) {
    return store
        .listNodesByTag(HAND_TYPE)
        .sort((left, right) => asNumber(left.meta.hand_number) - asNumber(right.meta.hand_number));
}
function findHandByNumber(store, handNumber) {
    return store.listNodesByTag(HAND_TYPE).find((node) => asNumber(node.meta.hand_number) === handNumber) ?? null;
}
function emptyOpponentMeta() {
    return {
        hands_played: 0,
        vpip: 0,
        pfr: 0,
        fold_to_bet: 0,
        aggr_preflop: 0,
        aggr_flop: 0,
        aggr_turn: 0,
        aggr_river: 0,
        cbet_opportunities: 0,
        cbet_folds: 0,
        three_bet_count: 0,
        river_bet_count: 0,
        river_bet_folds: 0,
        showdown_count: 0,
        showdown_win: 0,
    };
}
function buildOpponentBody(meta) {
    const vpipPct = percent(meta.vpip, meta.hands_played);
    const pfrPct = percent(meta.pfr, meta.hands_played);
    const cbetFoldPct = percent(meta.cbet_folds, meta.cbet_opportunities);
    const riverAggPct = percent(meta.river_bet_count, meta.hands_played);
    const style = vpipPct >= 45 && pfrPct >= 25 ? "loose-aggressive" : vpipPct >= 45 ? "loose-passive" : pfrPct >= 25 ? "aggressive" : "tight";
    const showdownText = meta.showdown_count > 0 ? `Villain has won ${meta.showdown_win} of ${meta.showdown_count} showdowns.` : "No villain showdown data yet.";
    return `Villain is ${style} (VPIP ${vpipPct}%, PFR ${pfrPct}%) and folds to hero flop c-bets ${meta.cbet_folds}/${meta.cbet_opportunities} times (${cbetFoldPct}%). River aggression shows up in ${meta.river_bet_count}/${meta.hands_played} hands (${riverAggPct}%). ${showdownText}`;
}
function buildHandBody(context, heroPosition, heroNet, streetReached) {
    const board = context.board.length > 0 ? context.board.join(" ") : "-";
    const hole = context.heroHoleCards.join(" ");
    const netStr = heroNet >= 0 ? `+${heroNet}` : `${heroNet}`;
    const actionSummary = summarizeActions(context.actionHistory, context.heroSeat);
    return `Hand ${context.handNumber}: hero ${heroPosition}, hole [${hole}], board [${board}]. ${actionSummary} Reached ${streetReached}. Net: ${netStr}.`;
}
function summarizeActions(history, heroSeat) {
    if (history.length === 0)
        return "No actions.";
    const streets = new Map();
    for (const action of history) {
        const actor = action.seat === heroSeat ? "hero" : "villain";
        const label = action.amount !== undefined ? `${actor} ${action.action} ${action.amount}` : `${actor} ${action.action}`;
        const bucket = streets.get(action.street) ?? [];
        bucket.push(label);
        streets.set(action.street, bucket);
    }
    return [...streets.entries()].map(([street, actions]) => `${street}: ${actions.join(", ")}.`).join(" ");
}
function heroMadeFlopCBet(preflopHeroActions, heroActions) {
    const heroRaisedPreflop = preflopHeroActions.some((entry) => isAggressive(entry.action));
    const heroBetFlop = heroActions.some((entry) => entry.street === "flop" && isAggressive(entry.action));
    return heroRaisedPreflop && heroBetFlop;
}
function didSeatFoldToHeroBet(history, heroSeat, targetSeat) {
    if (targetSeat === undefined)
        return false;
    for (let index = 1; index < history.length; index += 1) {
        const prev = history[index - 1];
        const current = history[index];
        if (prev.seat === heroSeat && isAggressive(prev.action) && current.seat === targetSeat && (current.action === "fold" || current.action === "auto_fold")) {
            return true;
        }
    }
    return false;
}
function seatFacedHeroStreetAggression(history, heroSeat, targetSeat, street) {
    if (targetSeat === undefined)
        return false;
    for (let index = 1; index < history.length; index += 1) {
        const prev = history[index - 1];
        const current = history[index];
        if (prev.street !== street || current.street !== street)
            continue;
        if (prev.seat === heroSeat && isAggressive(prev.action) && current.seat === targetSeat) {
            return true;
        }
    }
    return false;
}
function villainFoldedToHeroStreetAggression(history, heroSeat, targetSeat, street) {
    if (targetSeat === undefined)
        return false;
    for (let index = 1; index < history.length; index += 1) {
        const prev = history[index - 1];
        const current = history[index];
        if (prev.street !== street || current.street !== street)
            continue;
        if (prev.seat === heroSeat && isAggressive(prev.action) && current.seat === targetSeat && (current.action === "fold" || current.action === "auto_fold")) {
            return true;
        }
    }
    return false;
}
function estimateFinalPot(history) {
    return history.reduce((sum, action) => sum + (action.amount ?? 0), 0);
}
function lastStreetReached(context) {
    if (context.showdownReached)
        return "showdown";
    const order = ["preflop", "flop", "turn", "river"];
    const streets = new Set(context.actionHistory.map((action) => action.street));
    let last = "preflop";
    for (const street of order) {
        if (streets.has(street))
            last = street;
    }
    return last;
}
function isAggressive(action) {
    return action === "bet" || action === "raise";
}
function boolCount(value) {
    return truthy(value) ? 1 : 0;
}
function truthy(value) {
    return value === true;
}
function percent(numerator, denominator) {
    if (denominator <= 0)
        return 0;
    return Math.round((numerator / denominator) * 100);
}
function toRef(node) {
    return { type: node.type, id: node.id };
}
function asNumber(value) {
    return typeof value === "number" && Number.isFinite(value) ? value : 0;
}
