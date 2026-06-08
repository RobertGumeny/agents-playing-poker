#!/usr/bin/env python3
"""
Profile-fidelity cross-validation for Phase 2 memory brackets.

Each memory agent keeps structured per-hand records of the opponent ("villain"):
  - llm-akg-durable : memory-export.json, nodes of type "hand" (body carries
                      "Villain: <c> <c>. Board: <c> ...")
  - llm-md-wiki     : wiki/hands/hand-N.md (frontmatter hand:N, body
                      "**Villain:** <c> <c>", "**Board:** <c> ...")

This script cross-validates those stated reads against engine ground truth in
hands.jsonl. It is fully deterministic — no LLM judging. It scores, per agent:

  - FABRICATION : a specific villain holding stated for a NON-showdown hand.
                  In showdown-only info mode the agent could not have known the
                  opponent's cards, so any such holding was invented. This is the
                  sharpest fidelity-drift signal.
  - CARD ERROR  : villain holding stated for a real showdown hand that does not
                  match the cards actually shown.
  - BOARD ERROR : a claimed board card that was never dealt in that hand.

Each claim is bucketed by hand number so accuracy-vs-hand-number drift is visible.

Usage:
  python3 fidelity.py <experiment-id> [--csv OUT.csv] [--report OUT.md]

Ground truth and session layout are read from the experiment definition under
research/experiments/<id>/<id>.json (groups[].seat0/seat1/seeds) so the script
is reusable across the mdsingle / wiki / fullhistory brackets.
"""

import argparse
import json
import os
import re
import sys

CARD = r"(?:10|[2-9TJQKAtjqka])[shdcSHDC]"
CARD_RE = re.compile(CARD)
REPO = os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))


def norm_card(c):
    c = c.replace("10", "T")
    return c[0].upper() + c[1].lower()


def cards_in(text):
    return [norm_card(m.group(0)) for m in CARD_RE.finditer(text)]


# ----- ground truth ----------------------------------------------------------

def load_ground_truth(session_dir):
    gt = {}
    with open(os.path.join(session_dir, "hands.jsonl")) as f:
        for line in f:
            h = json.loads(line)
            n = h["hand_number"]
            gt[n] = {
                "board": [norm_card(c) for c in h.get("board", [])],
                "hole": {int(k): [norm_card(c) for c in v]
                         for k, v in h.get("hole_cards", {}).items()},
                "showdown": bool(h.get("showdown_reached")),
            }
    return gt


# ----- claim extraction ------------------------------------------------------

def extract_akg_claims(agent_dir):
    """Return list of (hand_number, villain_cards|None, board_cards) per hand node."""
    path = os.path.join(agent_dir, "memory-export.json")
    if not os.path.exists(path):
        return []
    data = json.load(open(path))
    claims = []
    for node in data.get("nodes", []):
        if node.get("type") != "hand":
            continue
        m = re.search(r"(\d+)", node.get("id", ""))
        if not m:
            continue
        hn = int(m.group(1))
        body = node.get("body", "")
        claims.append((hn, *parse_villain_board(body)))
    return claims


def extract_wiki_claims(agent_dir):
    hands_dir = os.path.join(agent_dir, "wiki", "hands")
    if not os.path.isdir(hands_dir):
        return []
    claims = []
    for fn in os.listdir(hands_dir):
        if not fn.endswith(".md"):
            continue
        text = open(os.path.join(hands_dir, fn)).read()
        m = re.search(r"hand:\s*(\d+)", text) or re.search(r"hand-(\d+)", fn)
        if not m:
            continue
        hn = int(m.group(1))
        claims.append((hn, *parse_villain_board(text)))
    return claims


def parse_villain_board(text):
    """Pull the villain holding and board off a structured hand record."""
    villain = None
    vm = re.search(r"[Vv]illain[:* ]+\s*(" + CARD + r")\s*[, ]?\s*(" + CARD + r")", text)
    if vm:
        villain = sorted([norm_card(vm.group(1)), norm_card(vm.group(2))])
    # Board: only a "Board:" field that STARTS a line (after optional markdown
    # bullet/bold), capturing the leading contiguous card run. This excludes inline
    # prose ("paired board", or a comparison citing another hand's board) which would
    # otherwise mis-attribute a different hand's cards to this record.
    board = None
    bm = re.search(r"^[\s*_>#-]*[Bb]oard[:*\s]+(" + CARD + r"(?:[ ,]+" + CARD + r")*)",
                   text, re.MULTILINE)
    if bm:
        bc = cards_in(bm.group(1))
        if bc:
            board = bc[:5]
    return villain, board


# ----- verification ----------------------------------------------------------

def verify(claims, gt, villain_seat):
    rows = []
    for hn, villain_cards, board in claims:
        g = gt.get(hn)
        rec = {"hand": hn, "fabrication": False, "card_error": False,
               "board_error": False, "checked_cards": False, "checked_board": False}
        if g is None:
            rec["board_error"] = bool(board)  # references a hand that never happened
            rec["note"] = "no such hand in ground truth"
            rows.append(rec)
            continue
        if villain_cards:
            rec["checked_cards"] = True
            actual = sorted(g["hole"].get(villain_seat, []))
            if not g["showdown"]:
                rec["fabrication"] = True
                rec["note"] = f"villain {villain_cards} stated on non-showdown hand"
            elif villain_cards != actual:
                rec["card_error"] = True
                rec["note"] = f"claimed {villain_cards} vs actual {actual}"
        if board:
            rec["checked_board"] = True
            dealt = g["board"]
            stray = [c for c in board if c not in dealt]
            if stray:
                rec["board_error"] = True
                rec.setdefault("note", "")
                rec["note"] = (rec.get("note", "") + f" board stray {stray} (dealt {dealt})").strip()
        rows.append(rec)
    return rows


# ----- experiment layout -----------------------------------------------------

def load_layout(exp_id):
    path = os.path.join(REPO, "research", "experiments", exp_id, f"{exp_id}.json")
    d = json.load(open(path))
    sessions = []
    for grp in d["groups"]:
        for seed in grp["seeds"]:
            sid = f"{grp['session_base']}-{seed}"
            sessions.append({
                "id": sid,
                "seat0": grp["seat0"],
                "seat1": grp["seat1"],
            })
    return d, sessions


def agg(rows):
    n = len(rows)
    card_claims = sum(r["checked_cards"] for r in rows)
    fab = sum(r["fabrication"] for r in rows)
    cerr = sum(r["card_error"] for r in rows)
    berr = sum(r["board_error"] for r in rows)
    return {"records": n, "card_claims": card_claims, "fabrications": fab,
            "card_errors": cerr, "board_errors": berr}


def drift_buckets(rows, edges=(50, 100)):
    buckets = {}
    bounds = [(0, edges[0]), (edges[0], edges[1]), (edges[1], 10**9)]
    labels = [f"1-{edges[0]}", f"{edges[0]+1}-{edges[1]}", f"{edges[1]+1}+"]
    for (lo, hi), lab in zip(bounds, labels):
        sub = [r for r in rows if lo < r["hand"] <= hi]
        cc = sum(r["checked_cards"] for r in sub)
        buckets[lab] = {
            "records": len(sub),
            "card_claims": cc,
            "fabrications": sum(r["fabrication"] for r in sub),
            "card_errors": sum(r["card_error"] for r in sub),
        }
    return buckets


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("experiment")
    ap.add_argument("--csv")
    ap.add_argument("--report")
    args = ap.parse_args()

    d, sessions = load_layout(args.experiment)
    sess_root = os.path.join(REPO, "research", "experiments", args.experiment, "sessions")

    extractors = {"llm-akg-durable": extract_akg_claims, "llm-md-wiki": extract_wiki_claims}
    per_agent_rows = {}   # agent name -> list of rows (across sessions)
    per_session = []      # (session, agent, agg)
    csv_rows = []

    for s in sessions:
        sdir = os.path.join(sess_root, s["id"])
        if not os.path.isdir(sdir):
            print(f"skip missing session {s['id']}", file=sys.stderr)
            continue
        gt = load_ground_truth(sdir)
        for seat, name in ((0, s["seat0"]), (1, s["seat1"])):
            ext = extractors.get(name)
            if not ext:
                continue
            adir = os.path.join(sdir, "agents", name)
            claims = ext(adir)
            villain_seat = 1 - seat
            rows = verify(claims, gt, villain_seat)
            per_agent_rows.setdefault(name, []).extend(rows)
            per_session.append((s["id"], name, agg(rows)))
            for r in rows:
                csv_rows.append({"session": s["id"], "agent": name, **r})

    # ---- print ----
    print(f"Fidelity cross-validation: {args.experiment}")
    print(f"Strategies: {' vs '.join(sorted(per_agent_rows))}")
    print("\nStructured per-hand records cross-validated against hands.jsonl ground truth.")
    print("(showdown-only info mode: any specific villain holding on a non-showdown hand is a fabrication)\n")

    hdr = f"{'Agent':16} {'recs':>5} {'cardClaims':>10} {'fabricated':>10} {'cardErr':>8} {'boardErr':>8} {'fab%ofCards':>11}"
    print(hdr); print("-" * len(hdr))
    for name in sorted(per_agent_rows):
        a = agg(per_agent_rows[name])
        fabpct = (100.0 * a["fabrications"] / a["card_claims"]) if a["card_claims"] else 0.0
        print(f"{name:16} {a['records']:>5} {a['card_claims']:>10} {a['fabrications']:>10} "
              f"{a['card_errors']:>8} {a['board_errors']:>8} {fabpct:>10.1f}%")

    print("\nFidelity drift (by hand-number bucket):")
    for name in sorted(per_agent_rows):
        b = drift_buckets(per_agent_rows[name])
        print(f"  {name}")
        for lab, v in b.items():
            fabpct = (100.0 * v["fabrications"] / v["card_claims"]) if v["card_claims"] else 0.0
            print(f"    hands {lab:>7}: {v['records']:>3} recs, {v['card_claims']:>3} card-claims, "
                  f"{v['fabrications']:>3} fabricated ({fabpct:.0f}%), {v['card_errors']:>2} card-err")

    print("\nPer session:")
    ph = f"  {'session':16} {'agent':16} {'recs':>4} {'cardClaims':>10} {'fab':>4} {'cardErr':>7} {'boardErr':>8}"
    print(ph)
    for sid, name, a in per_session:
        print(f"  {sid:16} {name:16} {a['records']:>4} {a['card_claims']:>10} "
              f"{a['fabrications']:>4} {a['card_errors']:>7} {a['board_errors']:>8}")

    # ---- csv ----
    if args.csv:
        import csv
        keys = ["session", "agent", "hand", "fabrication", "card_error",
                "board_error", "checked_cards", "checked_board", "note"]
        with open(args.csv, "w", newline="") as f:
            w = csv.DictWriter(f, fieldnames=keys, extrasaction="ignore")
            w.writeheader()
            for r in csv_rows:
                w.writerow(r)
        print(f"\nPer-claim CSV: {args.csv}")

    # ---- markdown report ----
    if args.report:
        with open(args.report, "w") as f:
            f.write(f"# Fidelity Cross-Validation: {args.experiment}\n\n")
            f.write("Objective pass — structured per-hand opponent records cross-validated "
                    "against `hands.jsonl` ground truth. Deterministic, no LLM judging.\n\n")
            f.write("**Fabrication** = a specific villain holding stated for a non-showdown hand "
                    "(unknowable in showdown-only info mode). **Card error** = holding stated for a "
                    "real showdown that doesn't match the cards shown. **Board error** = a claimed "
                    "board card never dealt that hand.\n\n")
            f.write("## Per agent\n\n")
            f.write("| Agent | records | card-claims | fabricated | card-err | board-err | fab % of card-claims |\n")
            f.write("|---|--:|--:|--:|--:|--:|--:|\n")
            for name in sorted(per_agent_rows):
                a = agg(per_agent_rows[name])
                fabpct = (100.0 * a["fabrications"] / a["card_claims"]) if a["card_claims"] else 0.0
                f.write(f"| {name} | {a['records']} | {a['card_claims']} | {a['fabrications']} | "
                        f"{a['card_errors']} | {a['board_errors']} | {fabpct:.1f}% |\n")
            f.write("\n## Fidelity drift (hand-number buckets)\n\n")
            for name in sorted(per_agent_rows):
                f.write(f"### {name}\n\n")
                f.write("| hands | records | card-claims | fabricated | fab % | card-err |\n|---|--:|--:|--:|--:|--:|\n")
                for lab, v in drift_buckets(per_agent_rows[name]).items():
                    fabpct = (100.0 * v["fabrications"] / v["card_claims"]) if v["card_claims"] else 0.0
                    f.write(f"| {lab} | {v['records']} | {v['card_claims']} | {v['fabrications']} | "
                            f"{fabpct:.0f}% | {v['card_errors']} |\n")
                f.write("\n")
            f.write("## Per session\n\n")
            f.write("| session | agent | records | card-claims | fabricated | card-err | board-err |\n|---|---|--:|--:|--:|--:|--:|\n")
            for sid, name, a in per_session:
                f.write(f"| {sid} | {name} | {a['records']} | {a['card_claims']} | "
                        f"{a['fabrications']} | {a['card_errors']} | {a['board_errors']} |\n")
        print(f"Report: {args.report}")


if __name__ == "__main__":
    main()
