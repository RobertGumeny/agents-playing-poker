import { join } from "node:path";
import { open } from "akg-ts";
const OPPONENT_TYPE = "opponent";
const OPPONENT_ID = "villain";
const HAND_TYPE = "hand";
const RECENT_HANDS_LIMIT = 5;
export class AkgMemoryPolicy {
    store = null;
    storePath = null;
    serverMemoryDir;
    get memoryDir() {
        return this.serverMemoryDir;
    }
    async beforeDecision(context) {
        this.serverMemoryDir = context.state.session?.memoryDir;
        const store = await this.getStore();
        if (!store) {
            return { sections: ["Memory: not available (no memory_dir provided)."] };
        }
        const sections = [];
        const opponent = store.getNode(OPPONENT_TYPE, OPPONENT_ID);
        if (opponent) {
            sections.push("Opponent profile:", opponent.body, formatOpponentMeta(opponent.meta));
        }
        else {
            sections.push("Opponent profile: no data yet.");
        }
        const allHands = store.listNodesByTag(HAND_TYPE);
        const recentHands = allHands
            .filter((n) => typeof n.meta.hand_number === "number")
            .sort((a, b) => b.meta.hand_number - a.meta.hand_number)
            .slice(0, RECENT_HANDS_LIMIT)
            .reverse();
        if (recentHands.length === 0) {
            sections.push("Recent hands: none yet.");
        }
        else {
            sections.push(`Recent hands (last ${recentHands.length}):`);
            for (const hand of recentHands) {
                sections.push(hand.body);
            }
        }
        return { sections };
    }
    async afterHandEnd(context) {
        const store = await this.getStore();
        if (!store)
            return;
        upsertOpponent(store, context);
        putHand(store, context);
        await store.commit();
    }
    async getStore() {
        if (this.store)
            return this.store;
        const dir = this.serverMemoryDir;
        if (!dir)
            return null;
        const path = join(dir, "memory.akg");
        if (this.storePath === path)
            return this.store;
        this.store = await open(path);
        this.storePath = path;
        return this.store;
    }
}
function upsertOpponent(store, context) {
    const existing = store.getNode(OPPONENT_TYPE, OPPONENT_ID);
    const prev = existing
        ? existing.meta
        : { hands_played: 0, vpip: 0, pfr: 0, fold_to_bet: 0, aggr_streets: 0, showdown_count: 0, showdown_win: 0 };
    const villain = context.seats.find((s) => s.seat !== context.heroSeat);
    const villainSeat = villain?.seat;
    const villainName = villain?.name ?? "villain";
    const villainActions = context.actionHistory.filter((a) => a.seat === villainSeat);
    const preflopActions = villainActions.filter((a) => a.street === "preflop");
    const voluntarilyPlayed = preflopActions.some((a) => a.action === "call" || a.action === "bet" || a.action === "raise");
    const preflopRaised = preflopActions.some((a) => a.action === "raise" || a.action === "bet");
    const aggrStreets = countAggressiveStreets(villainActions);
    const foldedToBet = didVillainFoldToBet(context, villainSeat);
    const showdownEntry = context.showdown ? Object.entries(context.showdown).find(([seat]) => Number(seat) === villainSeat) : undefined;
    const villainWonShowdown = showdownEntry !== undefined && context.result.some((r) => r.seat === villainSeat && r.chips_delta > 0);
    const next = {
        hands_played: prev.hands_played + 1,
        vpip: prev.vpip + (voluntarilyPlayed ? 1 : 0),
        pfr: prev.pfr + (preflopRaised ? 1 : 0),
        fold_to_bet: prev.fold_to_bet + (foldedToBet ? 1 : 0),
        aggr_streets: prev.aggr_streets + aggrStreets,
        showdown_count: prev.showdown_count + (showdownEntry !== undefined ? 1 : 0),
        showdown_win: prev.showdown_win + (villainWonShowdown ? 1 : 0),
    };
    store.putNode(OPPONENT_TYPE, OPPONENT_ID, {
        title: villainName,
        body: buildOpponentBody(next),
        meta: next,
    }, [OPPONENT_TYPE]);
}
function putHand(store, context) {
    const heroResult = context.result.find((r) => r.seat === context.heroSeat)?.chips_delta ?? 0;
    const heroPosition = context.dealerSeat === context.heroSeat ? "sb" : "bb";
    const streetReached = lastStreetReached(context);
    store.putNode(HAND_TYPE, "", {
        title: `Hand ${context.handNumber}`,
        body: buildHandBody(context, heroPosition, heroResult, streetReached),
        meta: {
            hand_number: context.handNumber,
            hero_position: heroPosition,
            hero_net: heroResult,
            street_reached: streetReached,
            showdown: context.showdownReached,
        },
    }, [HAND_TYPE]);
}
function buildOpponentBody(m) {
    const vpipPct = m.hands_played > 0 ? Math.round((m.vpip / m.hands_played) * 100) : 0;
    const pfrPct = m.hands_played > 0 ? Math.round((m.pfr / m.hands_played) * 100) : 0;
    const foldRate = m.hands_played > 0 ? Math.round((m.fold_to_bet / m.hands_played) * 100) : 0;
    const sdWinRate = m.showdown_count > 0 ? Math.round((m.showdown_win / m.showdown_count) * 100) : 0;
    return [
        `${m.hands_played} hands played.`,
        `VPIP ${vpipPct}% (${m.vpip}/${m.hands_played}), PFR ${pfrPct}% (${m.pfr}/${m.hands_played}).`,
        `Folded to hero bet/raise ${m.fold_to_bet} times (${foldRate}% of hands).`,
        `Aggression: ${m.aggr_streets} aggressive streets total.`,
        `Showdown: ${m.showdown_win}/${m.showdown_count} won (${sdWinRate}%).`,
    ].join(" ");
}
function buildHandBody(context, heroPosition, heroNet, streetReached) {
    const board = context.board.length > 0 ? context.board.join(" ") : "-";
    const hole = context.heroHoleCards.join(" ");
    const netStr = heroNet >= 0 ? `+${heroNet}` : `${heroNet}`;
    const actionSummary = summarizeActions(context);
    return `Hand ${context.handNumber}: hero ${heroPosition}, hole [${hole}], board [${board}]. ${actionSummary} Reached ${streetReached}. Net: ${netStr}.`;
}
function summarizeActions(context) {
    if (context.actionHistory.length === 0)
        return "No actions.";
    const streets = new Map();
    for (const a of context.actionHistory) {
        const actor = a.seat === context.heroSeat ? "hero" : "villain";
        const label = a.amount !== undefined ? `${actor} ${a.action} ${a.amount}` : `${actor} ${a.action}`;
        const bucket = streets.get(a.street) ?? [];
        bucket.push(label);
        streets.set(a.street, bucket);
    }
    return [...streets.entries()].map(([street, acts]) => `${street}: ${acts.join(", ")}.`).join(" ");
}
function countAggressiveStreets(actions) {
    const aggressive = new Set();
    for (const a of actions) {
        if (a.action === "bet" || a.action === "raise") {
            aggressive.add(a.street);
        }
    }
    return aggressive.size;
}
function didVillainFoldToBet(context, villainSeat) {
    if (villainSeat === undefined)
        return false;
    const history = context.actionHistory;
    for (let i = 1; i < history.length; i++) {
        const prev = history[i - 1];
        const curr = history[i];
        if ((prev.action === "bet" || prev.action === "raise") && prev.seat === context.heroSeat && curr.action === "fold" && curr.seat === villainSeat) {
            return true;
        }
    }
    return false;
}
function lastStreetReached(context) {
    if (context.showdownReached)
        return "showdown";
    const order = ["preflop", "flop", "turn", "river"];
    const streets = new Set(context.actionHistory.map((a) => a.street));
    let last = "preflop";
    for (const s of order) {
        if (streets.has(s))
            last = s;
    }
    return last;
}
function formatOpponentMeta(meta) {
    return `Stats: hands=${meta.hands_played ?? 0}, vpip=${meta.vpip ?? 0}, pfr=${meta.pfr ?? 0}, fold_to_bet=${meta.fold_to_bet ?? 0}, aggr_streets=${meta.aggr_streets ?? 0}, showdown=${meta.showdown_win ?? 0}/${meta.showdown_count ?? 0}`;
}
