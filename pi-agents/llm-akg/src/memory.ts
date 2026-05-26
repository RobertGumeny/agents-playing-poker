import { join } from "node:path";

import { open, type Store } from "akg-ts";

import type { CompletedHandContext, DecisionContext, MemoryPolicy, PromptAugmentation } from "@agent-poker/pi-agent-shared";

const OPPONENT_TYPE = "opponent";
const OPPONENT_ID = "villain";
const HAND_TYPE = "hand";
const RECENT_HANDS_LIMIT = 5;

interface OpponentMeta {
  hands_played: number;
  vpip: number;
  pfr: number;
  fold_to_bet: number;
  aggr_streets: number;
  showdown_count: number;
  showdown_win: number;
}

export class AkgMemoryPolicy implements MemoryPolicy {
  private store: Store | null = null;
  private storePath: string | null = null;
  private serverMemoryDir: string | undefined;

  get memoryDir(): string | undefined {
    return this.serverMemoryDir;
  }

  async beforeDecision(context: DecisionContext): Promise<PromptAugmentation> {
    this.serverMemoryDir = context.state.session?.memoryDir;
    const store = await this.getStore();
    if (!store) {
      return { sections: ["Memory: not available (no memory_dir provided)."] };
    }

    const sections: string[] = [];

    const opponent = store.getNode(OPPONENT_TYPE, OPPONENT_ID);
    if (opponent) {
      sections.push("Opponent profile:", opponent.body, formatOpponentMeta(opponent.meta as Partial<OpponentMeta>));
    } else {
      sections.push("Opponent profile: no data yet.");
    }

    const allHands = store.listNodesByTag(HAND_TYPE);
    const recentHands = allHands
      .filter((n) => typeof (n.meta as Record<string, unknown>).hand_number === "number")
      .sort((a, b) => (b.meta as Record<string, number>).hand_number - (a.meta as Record<string, number>).hand_number)
      .slice(0, RECENT_HANDS_LIMIT)
      .reverse();

    if (recentHands.length === 0) {
      sections.push("Recent hands: none yet.");
    } else {
      sections.push(`Recent hands (last ${recentHands.length}):`);
      for (const hand of recentHands) {
        sections.push(hand.body);
      }
    }

    return { sections };
  }

  async afterHandEnd(context: CompletedHandContext): Promise<void> {
    const store = await this.getStore();
    if (!store) return;

    upsertOpponent(store, context);
    putHand(store, context);
    await store.commit();
  }

  private async getStore(): Promise<Store | null> {
    if (this.store) return this.store;
    const dir = this.serverMemoryDir;
    if (!dir) return null;
    const path = join(dir, "memory.akg");
    if (this.storePath === path) return this.store;
    this.store = await open(path);
    this.storePath = path;
    return this.store;
  }
}

function upsertOpponent(store: Store, context: CompletedHandContext): void {
  const existing = store.getNode(OPPONENT_TYPE, OPPONENT_ID);
  const prev: OpponentMeta = existing
    ? (existing.meta as unknown as OpponentMeta)
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

  const next: OpponentMeta = {
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
    meta: next as unknown as Record<string, unknown>,
  }, [OPPONENT_TYPE]);
}

function putHand(store: Store, context: CompletedHandContext): void {
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
    } as Record<string, unknown>,
  }, [HAND_TYPE]);
}

function buildOpponentBody(m: OpponentMeta): string {
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

function buildHandBody(context: CompletedHandContext, heroPosition: string, heroNet: number, streetReached: string): string {
  const board = context.board.length > 0 ? context.board.join(" ") : "-";
  const hole = context.heroHoleCards.join(" ");
  const netStr = heroNet >= 0 ? `+${heroNet}` : `${heroNet}`;
  const actionSummary = summarizeActions(context);
  return `Hand ${context.handNumber}: hero ${heroPosition}, hole [${hole}], board [${board}]. ${actionSummary} Reached ${streetReached}. Net: ${netStr}.`;
}

function summarizeActions(context: CompletedHandContext): string {
  if (context.actionHistory.length === 0) return "No actions.";

  const streets = new Map<string, string[]>();
  for (const a of context.actionHistory) {
    const actor = a.seat === context.heroSeat ? "hero" : "villain";
    const label = a.amount !== undefined ? `${actor} ${a.action} ${a.amount}` : `${actor} ${a.action}`;
    const bucket = streets.get(a.street) ?? [];
    bucket.push(label);
    streets.set(a.street, bucket);
  }

  return [...streets.entries()].map(([street, acts]) => `${street}: ${acts.join(", ")}.`).join(" ");
}

function countAggressiveStreets(actions: CompletedHandContext["actionHistory"]): number {
  const aggressive = new Set<string>();
  for (const a of actions) {
    if (a.action === "bet" || a.action === "raise") {
      aggressive.add(a.street);
    }
  }
  return aggressive.size;
}

function didVillainFoldToBet(context: CompletedHandContext, villainSeat: number | undefined): boolean {
  if (villainSeat === undefined) return false;
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

function lastStreetReached(context: CompletedHandContext): string {
  if (context.showdownReached) return "showdown";
  const order = ["preflop", "flop", "turn", "river"] as const;
  const streets = new Set(context.actionHistory.map((a) => a.street));
  let last: string = "preflop";
  for (const s of order) {
    if (streets.has(s)) last = s;
  }
  return last;
}

function formatOpponentMeta(meta: Partial<OpponentMeta>): string {
  return `Stats: hands=${meta.hands_played ?? 0}, vpip=${meta.vpip ?? 0}, pfr=${meta.pfr ?? 0}, fold_to_bet=${meta.fold_to_bet ?? 0}, aggr_streets=${meta.aggr_streets ?? 0}, showdown=${meta.showdown_win ?? 0}/${meta.showdown_count ?? 0}`;
}
