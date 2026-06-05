#!/usr/bin/env python3
"""
Eval analysis script for agent-poker experiments.

Usage:
  python3 analyze.py <experiment-id> [--traces [KEYWORD]] [--hand N]

Examples:
  python3 analyze.py test-2e-pattern-confidence
  python3 analyze.py test-2e-pattern-confidence --traces strong-showdown-caller
  python3 analyze.py test-2e-pattern-confidence --hand 97
"""

import json
import os
import sys
import re
import csv
import argparse
import textwrap
from datetime import date

SESSIONS_DIR = "research/sessions"
EXPERIMENTS_DIR = "research/experiments"
RESULTS_DIR = "docs/research/results"


# Set by main() once the experiment is resolved. Sessions for a claimed experiment
# live at research/experiments/<id>/sessions/<sid>/; unclaimed ones fall back to
# the flat research/sessions/<sid>/ area.
SCOPED_SESSIONS_DIR = None


def load_experiment(experiment_id):
    nested = os.path.join(EXPERIMENTS_DIR, experiment_id, f"{experiment_id}.json")
    flat = os.path.join(EXPERIMENTS_DIR, f"{experiment_id}.json")
    path = nested if os.path.exists(nested) else flat
    with open(path) as f:
        return json.load(f)


def resolve_scoped_sessions_dir(experiment_id):
    scoped = os.path.join(EXPERIMENTS_DIR, experiment_id, "sessions")
    return scoped if os.path.isdir(scoped) else None


def session_dir(sid):
    """First existing of the experiment-scoped then flat session location."""
    candidates = []
    if SCOPED_SESSIONS_DIR:
        candidates.append(os.path.join(SCOPED_SESSIONS_DIR, sid))
    candidates.append(os.path.join(SESSIONS_DIR, sid))
    for c in candidates:
        if os.path.isdir(c):
            return c
    return candidates[0]


def group_session_ids(group):
    """Session ids for one group: an explicit `sessions` list wins, else expand
    `session_base` + `sessions_count` per the experiment-definition contract."""
    explicit = group.get("sessions")
    if explicit:
        return list(explicit)
    base = group.get("session_base")
    count = group.get("sessions_count", 0)
    if base and count:
        return [f"{base}-{i}" for i in range(1, count + 1)]
    return []


def session_ids(exp):
    """Return [(label, session_id), ...] for all control and treatment sessions."""
    results = []
    for label in ("control", "treatment"):
        for sid in group_session_ids(exp.get(label, {})):
            results.append((label, sid))
    return results


def load_eval(session_id):
    path = os.path.join(session_dir(session_id), "eval.json")
    with open(path) as f:
        return json.load(f)


def find_focal_seat(ev, exp):
    """Return the seat dict for the focal agent (treatment/control agent name)."""
    agent_name = exp.get("treatment", {}).get("agent") or exp.get("control", {}).get("agent")
    for seat in ev.get("seats", []):
        if seat["name"] == agent_name:
            return seat
    # Fallback: first seat
    return ev["seats"][0] if ev["seats"] else {}


def build_comparison_table(exp):
    """Return (rows, control_mean_cph, treatment_mean_cph)."""
    rows = []
    group_totals = {"control": [], "treatment": []}

    for label, sid in session_ids(exp):
        try:
            ev = load_eval(sid)
        except FileNotFoundError:
            rows.append((label, sid, "MISSING", "", "", "", ""))
            continue

        hands = ev["session"]["hand_count"]
        seat = find_focal_seat(ev, exp)
        delta = seat.get("chips_delta", 0)
        cph = delta / hands if hands else 0
        sdr = ev["metrics"].get("showdown_rate", 0)
        fallbacks = ev["metrics"].get("fallback_action_count", 0)
        mem = seat.get("memory_export") or {}
        nodes = mem.get("NodesByType", {})
        edges = mem.get("EdgesByRelation", {})
        # Agents author an open vocabulary (their own node types and edge relations),
        # so show the actual histogram rather than fixed pattern/supported_by keys.
        # Totals plus the top three of each by count.
        n_total = sum(nodes.values())
        e_total = sum(edges.values())
        top_nodes = ",".join(f"{t}:{c}" for t, c in sorted(nodes.items(), key=lambda kv: -kv[1])[:3])
        top_edges = ",".join(f"{r}:{c}" for r, c in sorted(edges.items(), key=lambda kv: -kv[1])[:3])
        mem_summary = (
            f"n={n_total}[{top_nodes}] e={e_total}[{top_edges}]"
            if (nodes or edges) else "n=0 e=0"
        )
        rows.append((label, sid, delta, hands, cph, sdr, fallbacks, mem_summary))
        group_totals[label].append(cph)

    ctrl_mean = (sum(group_totals["control"]) / len(group_totals["control"])
                 if group_totals["control"] else 0)
    treat_mean = (sum(group_totals["treatment"]) / len(group_totals["treatment"])
                  if group_totals["treatment"] else 0)
    return rows, ctrl_mean, treat_mean


def format_table(rows, ctrl_mean, treat_mean):
    lines = []
    lines.append(f"{'Group':<12} {'Session':<45} {'chips_delta':>11} {'hands':>6} {'c/h':>7} {'sdr':>6} {'fallbacks':>10} {'memory'}")
    lines.append("-" * 120)
    prev_label = None
    for row in rows:
        label = row[0]
        if label != prev_label and prev_label is not None:
            lines.append("")
        prev_label = label
        if row[2] == "MISSING":
            lines.append(f"{label:<12} {row[1]:<45} {'MISSING'}")
            continue
        label_, sid, delta, hands, cph, sdr, fallbacks, mem_summary = row
        sign = "+" if delta >= 0 else ""
        lines.append(
            f"{label:<12} {sid:<45} {sign}{delta:>10} {hands:>6} {cph:>+7.2f} {sdr:>6.2%} {fallbacks:>10} {mem_summary}"
        )

    lines.append("-" * 120)
    lines.append(f"{'control mean c/h':>70}  {ctrl_mean:>+7.2f}")
    lines.append(f"{'treatment mean c/h':>70}  {treat_mean:>+7.2f}")
    delta_str = f"{treat_mean - ctrl_mean:>+7.2f}"
    lines.append(f"{'delta (treatment - control)':>70}  {delta_str}")
    return "\n".join(lines)


def count_reasoning_mentions(session_path, keyword):
    mentions = 0
    total_turns = 0
    try:
        with open(session_path) as f:
            for line in f:
                try:
                    obj = json.loads(line)
                except json.JSONDecodeError:
                    continue
                # pi-session.jsonl wraps messages as {type: "message", message: {role, content, ...}}
                if obj.get("type") == "message":
                    obj = obj.get("message", {})
                if obj.get("role") != "assistant":
                    continue
                total_turns += 1
                content = obj.get("content", "")
                if isinstance(content, list):
                    text = " ".join(b.get("text", "") for b in content if isinstance(b, dict))
                else:
                    text = str(content)
                if keyword.lower() in text.lower():
                    mentions += 1
    except FileNotFoundError:
        return None, None
    return mentions, total_turns


def traces_table(exp, keyword):
    lines = []
    lines.append(f"\nPattern reasoning mentions for keyword: '{keyword}'")
    lines.append(f"{'Group':<12} {'Session':<45} {'mentions':>9} {'asst_turns':>11} {'rate':>7}")
    lines.append("-" * 90)
    for label, sid in session_ids(exp):
        pi_path = os.path.join(
            session_dir(sid),
            f"agents/{exp.get('treatment', {}).get('agent') or exp.get('control', {}).get('agent')}/pi-session.jsonl"
        )
        mentions, total = count_reasoning_mentions(pi_path, keyword)
        if mentions is None:
            lines.append(f"{label:<12} {sid:<45} {'MISSING':>9}")
        else:
            rate = mentions / total if total else 0
            lines.append(f"{label:<12} {sid:<45} {mentions:>9} {total:>11} {rate:>7.2%}")
    return "\n".join(lines)


def find_hand_context(session_path, hand_number):
    """Yield (line_number, text_excerpt) for lines near hand_number in pi-session.jsonl."""
    target_lines = []
    try:
        with open(session_path) as f:
            for i, line in enumerate(f, 1):
                if f'"hand_number": {hand_number}' in line or f'"hand_number":{hand_number}' in line:
                    target_lines.append(i)
    except FileNotFoundError:
        return []

    results = []
    try:
        with open(session_path) as f:
            all_lines = f.readlines()
        for match_line in target_lines:
            start = max(0, match_line - 3)
            end = min(len(all_lines), match_line + 8)
            for j in range(start, end):
                raw = all_lines[j]
                try:
                    obj = json.loads(raw)
                    excerpt = json.dumps(obj)[:400]
                except json.JSONDecodeError:
                    excerpt = raw[:200]
                results.append((j + 1, excerpt))
    except FileNotFoundError:
        pass
    return results


def hand_drill(exp, hand_number):
    agent_name = exp.get("treatment", {}).get("agent") or exp.get("control", {}).get("agent")
    lines = []
    lines.append(f"\nHand #{hand_number} context across sessions")
    for label, sid in session_ids(exp):
        pi_path = os.path.join(session_dir(sid), f"agents/{agent_name}/pi-session.jsonl")
        hits = find_hand_context(pi_path, hand_number)
        lines.append(f"\n--- {label} / {sid} ---")
        if not hits:
            lines.append("  (not found or session missing)")
        else:
            for lineno, excerpt in hits:
                lines.append(f"  L{lineno}: {excerpt}")
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Token-cost analysis (the fidelity-vs-COST frontier, metric #2 in research.md).
#
# Two LLM calls per agent are logged separately:
#   - decision calls   -> agents/<name>/pi-session.jsonl   (prompt header "Hand: N")
#   - post-hand update -> agents/<name>/update-session.jsonl (summary token "hand=N")
# Both files are concatenations of full Pi session transcripts, one sub-session
# per call, each delimited by a {"type":"session",...} boundary line. The prompt
# CONTEXT SIZE for a call is the first assistant turn's input+cacheRead+cacheWrite
# (cache reduces cost, not context size — the ballooning we want to chart is size).
# ---------------------------------------------------------------------------

HAND_DECISION_RE = re.compile(r"Hand:\s*(\d+)")
HAND_UPDATE_RE = re.compile(r"hand=(\d+)")
STREET_RE = re.compile(r"Street:\s*(\w+)")


def iter_subsessions(path):
    """Yield lists of message-objects, one list per Pi sub-session (split on
    {"type":"session"} boundary lines). Non-transcript records (e.g. compact
    fake_update_session entries) ride along harmlessly — they carry no usage."""
    cur = None
    try:
        with open(path) as f:
            for line in f:
                try:
                    obj = json.loads(line)
                except json.JSONDecodeError:
                    continue
                if obj.get("type") == "session":
                    if cur:
                        yield cur
                    cur = []
                    continue
                if cur is None:
                    cur = []
                cur.append(obj)
    except FileNotFoundError:
        return
    if cur:
        yield cur


def _first_user_text(sub):
    for obj in sub:
        m = obj.get("message", obj)
        if isinstance(m, dict) and m.get("role") == "user":
            c = m.get("content", "")
            if isinstance(c, list):
                return " ".join(b.get("text", "") for b in c if isinstance(b, dict))
            return str(c)
    return ""


def _assistant_usages(sub):
    out = []
    for obj in sub:
        m = obj.get("message", obj)
        if isinstance(m, dict) and m.get("role") == "assistant" and isinstance(m.get("usage"), dict):
            out.append(m["usage"])
    return out


def _prompt_context_tokens(usage):
    return usage.get("input", 0) + usage.get("cacheRead", 0) + usage.get("cacheWrite", 0)


def extract_calls(path, kind):
    """Return one row per LLM call in `path`. kind is 'decision' or 'update'."""
    rows = []
    for sub in iter_subsessions(path):
        usages = _assistant_usages(sub)
        if not usages:
            continue
        text = _first_user_text(sub)
        hand_re = HAND_DECISION_RE if kind == "decision" else HAND_UPDATE_RE
        hm = hand_re.search(text)
        sm = STREET_RE.search(text) if kind == "decision" else None
        rows.append({
            "kind": kind,
            "hand": int(hm.group(1)) if hm else None,
            "street": sm.group(1) if sm else "",
            "prompt_tokens": _prompt_context_tokens(usages[0]),
            "total_tokens": sum(u.get("totalTokens", 0) for u in usages),
            "cost": sum((u.get("cost") or {}).get("total", 0) for u in usages),
        })
    return rows


def agent_dirs(sid):
    base = os.path.join(session_dir(sid), "agents")
    if not os.path.isdir(base):
        return []
    return sorted(d for d in os.listdir(base) if os.path.isdir(os.path.join(base, d)))


def linregress_slope(points):
    """Least-squares slope of y on x for [(x, y), ...]; returns (slope, n)."""
    pts = [(x, y) for x, y in points if x is not None]
    n = len(pts)
    if n < 2:
        return None, n
    sx = sum(x for x, _ in pts)
    sy = sum(y for _, y in pts)
    sxx = sum(x * x for x, _ in pts)
    sxy = sum(x * y for x, y in pts)
    denom = n * sxx - sx * sx
    if denom == 0:
        return None, n
    return (n * sxy - sx * sy) / denom, n


def collect_token_rows(exp):
    """Walk every session and agent, returning a flat list of per-call rows
    tagged with group/session/agent. Aggregated by AGENT NAME downstream, so a
    seat-mirrored strategy's two appearances (focal in control, opponent in
    treatment) fold into one curve."""
    rows = []
    for label, sid in session_ids(exp):
        for agent in agent_dirs(sid):
            adir = os.path.join(session_dir(sid), "agents", agent)
            calls = (
                extract_calls(os.path.join(adir, "pi-session.jsonl"), "decision")
                + extract_calls(os.path.join(adir, "update-session.jsonl"), "update")
            )
            for c in calls:
                c.update({"group": label, "session": sid, "agent": agent})
                rows.append(c)
    return rows


def write_token_csv(experiment_id, rows):
    os.makedirs(RESULTS_DIR, exist_ok=True)
    path = os.path.join(RESULTS_DIR, f"{experiment_id}-tokens.csv")
    cols = ["group", "session", "agent", "kind", "hand", "street",
            "prompt_tokens", "total_tokens", "cost"]
    with open(path, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=cols)
        w.writeheader()
        for r in sorted(rows, key=lambda r: (r["agent"], r["session"], r["hand"] or 0,
                                             0 if r["kind"] == "decision" else 1)):
            w.writerow({k: r.get(k, "") for k in cols})
    return path


def _window_mean(decisions, lo_frac, hi_frac):
    """Mean prompt_tokens over a hand-number window [lo_frac, hi_frac] of the span."""
    hands = [d["hand"] for d in decisions if d["hand"] is not None]
    if not hands:
        return None
    lo_h, hi_h = min(hands), max(hands)
    span = hi_h - lo_h
    if span == 0:
        vals = [d["prompt_tokens"] for d in decisions if d["hand"] is not None]
        return sum(vals) / len(vals) if vals else None
    lo = lo_h + lo_frac * span
    hi = lo_h + hi_frac * span
    vals = [d["prompt_tokens"] for d in decisions
            if d["hand"] is not None and lo <= d["hand"] <= hi]
    return sum(vals) / len(vals) if vals else None


def token_summary(rows):
    """Per-agent token aggregates. Returns (agents_sorted, summary_by_agent)."""
    by_agent = {}
    for r in rows:
        by_agent.setdefault(r["agent"], []).append(r)

    summary = {}
    for agent, ars in by_agent.items():
        decisions = [r for r in ars if r["kind"] == "decision"]
        updates = [r for r in ars if r["kind"] == "update"]
        slope, n = linregress_slope([(d["hand"], d["prompt_tokens"]) for d in decisions])
        early = _window_mean(decisions, 0.0, 0.1)
        late = _window_mean(decisions, 0.9, 1.0)
        dec_prompt = [d["prompt_tokens"] for d in decisions]
        upd_prompt = [u["prompt_tokens"] for u in updates]
        summary[agent] = {
            "decisions": len(decisions),
            "updates": len(updates),
            "slope_tok_per_hand": slope,
            "slope_n": n,
            "dec_prompt_mean": (sum(dec_prompt) / len(dec_prompt)) if dec_prompt else 0,
            "early_prompt_mean": early,
            "late_prompt_mean": late,
            "upd_prompt_mean": (sum(upd_prompt) / len(upd_prompt)) if upd_prompt else 0,
            "total_tokens": sum(r["total_tokens"] for r in ars),
            "total_cost": sum(r["cost"] for r in ars),
        }
    return sorted(summary), summary


def format_token_table(rows, csv_path):
    agents, summary = token_summary(rows)
    lines = []
    lines.append("\nToken cost vs. hand count (decision prompt context = input+cacheRead+cacheWrite)")
    if not agents:
        lines.append("  (no token data found — pi-session.jsonl / update-session.jsonl missing)")
        return "\n".join(lines)
    hdr = (f"{'Agent':<22} {'decs':>5} {'upds':>5} {'slope tok/hand':>14} "
           f"{'early→late prompt':>20} {'upd prompt':>11} {'total tok':>12} {'total $':>9}")
    lines.append(hdr)
    lines.append("-" * len(hdr))
    for a in agents:
        s = summary[a]
        slope = f"{s['slope_tok_per_hand']:+.1f}" if s["slope_tok_per_hand"] is not None else "n/a"
        early = s["early_prompt_mean"]
        late = s["late_prompt_mean"]
        el = (f"{early:,.0f}→{late:,.0f}" if early is not None and late is not None else "n/a")
        lines.append(
            f"{a:<22} {s['decisions']:>5} {s['updates']:>5} {slope:>14} {el:>20} "
            f"{s['upd_prompt_mean']:>11,.0f} {s['total_tokens']:>12,} {s['total_cost']:>9.2f}"
        )
    lines.append("-" * len(hdr))
    lines.append("slope = least-squares growth of decision prompt context per hand "
                 "(≈flat ⇒ structured-memory cost win; steep ⇒ context ballooning).")
    lines.append(f"Per-call series written to: {csv_path}")
    return "\n".join(lines)


def write_report(experiment_id, exp, table_text, stdout_highlights, extra_sections):
    os.makedirs(RESULTS_DIR, exist_ok=True)
    path = os.path.join(RESULTS_DIR, f"{experiment_id}-analysis.md")
    today = date.today().isoformat()

    sections = [
        f"# Eval Analysis: {experiment_id}",
        f"",
        f"**Generated**: {today}",
        f"**Hypothesis**: {exp.get('hypothesis', '(none)')}",
        f"",
        f"## Summary",
        f"",
        stdout_highlights,
        f"",
        f"## Comparison Table",
        f"",
        f"```",
        table_text,
        f"```",
    ]
    for title, content in extra_sections:
        sections += [f"", f"## {title}", f"", f"```", content, f"```"]

    with open(path, "w") as f:
        f.write("\n".join(sections) + "\n")
    return path


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("experiment_id")
    parser.add_argument("--traces", nargs="?", const="pattern", metavar="KEYWORD",
                        help="Analyze assistant reasoning mentions for KEYWORD (default: 'pattern')")
    parser.add_argument("--hand", type=int, metavar="N",
                        help="Drill into hand number N across all sessions")
    parser.add_argument("--tokens", action="store_true",
                        help="Per-decision token-cost growth vs hand count (+CSV series)")
    args = parser.parse_args()

    global SCOPED_SESSIONS_DIR
    SCOPED_SESSIONS_DIR = resolve_scoped_sessions_dir(args.experiment_id)
    exp = load_experiment(args.experiment_id)
    rows, ctrl_mean, treat_mean = build_comparison_table(exp)
    table_text = format_table(rows, ctrl_mean, treat_mean)

    delta = treat_mean - ctrl_mean
    direction = exp.get("expected_direction", {}).get("chips_per_hand", "?")
    confirmed = (
        "YES" if (direction == "increase" and delta > 0) or (direction == "decrease" and delta < 0)
        else "NO"
    )
    highlights = textwrap.dedent(f"""\
        Experiment : {args.experiment_id}
        Model      : {exp.get('model', '?')}
        Expected   : chips/hand {direction}
        Control    : {ctrl_mean:+.2f} c/h (mean)
        Treatment  : {treat_mean:+.2f} c/h (mean)
        Delta      : {delta:+.2f} c/h
        Confirmed  : {confirmed}""")

    print(highlights)
    print()
    print(table_text)

    extra_sections = []

    if args.traces is not None:
        keyword = args.traces if args.traces != "pattern" else "pattern"
        traces_text = traces_table(exp, keyword)
        print(traces_text)
        extra_sections.append(("Pattern Reasoning Traces", traces_text))

    if args.hand is not None:
        drill_text = hand_drill(exp, args.hand)
        print(drill_text)
        extra_sections.append((f"Hand {args.hand} Context", drill_text))

    if args.tokens:
        token_rows = collect_token_rows(exp)
        csv_path = write_token_csv(args.experiment_id, token_rows)
        tokens_text = format_token_table(token_rows, csv_path)
        print(tokens_text)
        extra_sections.append(("Token Cost vs. Hand Count", tokens_text))

    report_path = write_report(args.experiment_id, exp, table_text, highlights, extra_sections)
    print(f"\nReport written to: {report_path}")


if __name__ == "__main__":
    main()
