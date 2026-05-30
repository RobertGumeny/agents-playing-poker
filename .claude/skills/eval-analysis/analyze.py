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
import argparse
import textwrap
from datetime import date

SESSIONS_DIR = "research/sessions"
EXPERIMENTS_DIR = "research/experiments"
RESULTS_DIR = "docs/research/results"


def load_experiment(experiment_id):
    path = os.path.join(EXPERIMENTS_DIR, f"{experiment_id}.json")
    with open(path) as f:
        return json.load(f)


def session_ids(exp):
    """Return [(label, session_id), ...] for all control and treatment sessions."""
    results = []
    ctrl = exp.get("control", {})
    for sid in ctrl.get("sessions", []):
        results.append(("control", sid))

    treat = exp.get("treatment", {})
    base = treat.get("session_base")
    count = treat.get("sessions_count", 0)
    if base and count:
        for i in range(1, count + 1):
            results.append(("treatment", f"{base}-{i}"))
    else:
        for sid in treat.get("sessions", []):
            results.append(("treatment", sid))

    return results


def load_eval(session_id):
    path = os.path.join(SESSIONS_DIR, session_id, "eval.json")
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
        mem_summary = (
            f"pat={nodes.get('pattern', '?')} "
            f"sup={edges.get('supported_by', '?')}"
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
            SESSIONS_DIR, sid,
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
        pi_path = os.path.join(SESSIONS_DIR, sid, f"agents/{agent_name}/pi-session.jsonl")
        hits = find_hand_context(pi_path, hand_number)
        lines.append(f"\n--- {label} / {sid} ---")
        if not hits:
            lines.append("  (not found or session missing)")
        else:
            for lineno, excerpt in hits:
                lines.append(f"  L{lineno}: {excerpt}")
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
    args = parser.parse_args()

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

    report_path = write_report(args.experiment_id, exp, table_text, highlights, extra_sections)
    print(f"\nReport written to: {report_path}")


if __name__ == "__main__":
    main()
