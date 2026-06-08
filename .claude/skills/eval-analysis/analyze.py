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
    """Return [(label, session_id), ...] for all groups."""
    results = []
    for i, group in enumerate(exp.get("groups", [])):
        label = f"group-{i}"
        for sid in group_session_ids(group):
            results.append((label, sid))
    return results


def load_eval(session_id):
    path = os.path.join(session_dir(session_id), "eval.json")
    with open(path) as f:
        return json.load(f)


def find_focal_seat(ev, exp, label=None):
    """Return the seat dict for seat0 of the relevant group."""
    groups = exp.get("groups", [])
    group_idx = 0
    if label and label.startswith("group-"):
        try:
            group_idx = int(label.split("-", 1)[1])
        except (ValueError, IndexError):
            pass
    agent_name = groups[group_idx].get("seat0") if group_idx < len(groups) else None
    if not agent_name and groups:
        agent_name = groups[0].get("seat0")
    for seat in ev.get("seats", []):
        if seat["name"] == agent_name:
            return seat
    # Fallback: first seat
    return ev["seats"][0] if ev["seats"] else {}


def build_comparison_table(exp):
    """Return rows of per-session stats. Chips/hand is a sanity check, not the headline."""
    rows = []

    for label, sid in session_ids(exp):
        try:
            ev = load_eval(sid)
        except FileNotFoundError:
            rows.append((label, sid, "MISSING", "", "", "", ""))
            continue

        hands = ev["session"]["hand_count"]
        seat = find_focal_seat(ev, exp, label)
        delta = seat.get("chips_delta", 0)
        cph = delta / hands if hands else 0
        sdr = ev["metrics"].get("showdown_rate", 0)
        fallbacks = ev["metrics"].get("fallback_action_count", 0)
        mem = seat.get("memory_export") or {}
        # eval.json uses snake_case; tolerate the legacy PascalCase from older artifacts.
        nodes = mem.get("nodes_by_type", mem.get("NodesByType", {}))
        edges = mem.get("edges_by_relation", mem.get("EdgesByRelation", {}))
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

    return rows


def format_table(rows):
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
    return "\n".join(lines)


# --- Pre-flight tripwire ------------------------------------------------------
# Guards the one failure that an expensive run can hide: a retrieval-memory agent
# that never opens its own memory (write-only graph), or an index that bloats into
# an O(hands) narrative instead of staying a slim pointer set. Both happened in the
# (now archived) phase2-wiki-vs-akg-150 run and went unnoticed for 3 hours.
#
# Runs by default (not behind a flag) — it is the safety net. read_calls == 0 is the
# load-bearing, hard-fail check; index_lines is a softer bloat warning.

# Decision-time read tools, per substrate. These are counted from eval.json
# seats[].tool_calls, which collect.go populates from the DECISION session only
# (pi-session.jsonl), not the update session — so a non-zero count means the agent
# actually drilled into its memory while deciding.
READ_TOOLS = {
    "akg_get_node", "akg_get_nodes", "akg_list_nodes",  # llm-akg-durable
    "md_read_page", "md_list_pages",                     # llm-md-wiki
}
# Agents that are SUPPOSED to retrieve. Non-retrieval strategies legitimately make
# zero read calls, so flagging them would be noise.
RETRIEVAL_STRATEGIES = {"llm-akg-durable", "llm-md-wiki"}
# Soft bloat threshold for the index body, in lines. The agents' prompts budget the
# index at ~25 lines of body; villain.md also carries YAML frontmatter + headers, so
# we warn only on clear narrative accumulation, not borderline.
INDEX_LINE_WARN = 40


def index_line_count(sid, seat_name):
    """Lines in the agent's root-index body. AKG: opponent/villain node body from
    memory-export.json. Wiki: wiki/villain.md. Returns None if unavailable."""
    base = os.path.join(session_dir(sid), "agents", seat_name)
    export = os.path.join(base, "memory-export.json")
    if os.path.exists(export):
        try:
            with open(export) as f:
                doc = json.load(f)
            for node in doc.get("nodes", []):
                if node.get("type") == "opponent" and node.get("id") == "villain":
                    return len((node.get("body") or "").splitlines())
        except (json.JSONDecodeError, OSError):
            return None
    villain = os.path.join(base, "wiki", "villain.md")
    if os.path.exists(villain):
        try:
            with open(villain) as f:
                return len(f.read().splitlines())
        except OSError:
            return None
    return None


def tripwire_rows(exp):
    """Per (session, retrieval-seat) read-usage + index-size diagnostics."""
    rows = []
    for label, sid in session_ids(exp):
        try:
            ev = load_eval(sid)
        except FileNotFoundError:
            rows.append({"label": label, "sid": sid, "seat": "-", "missing": True})
            continue
        for seat in ev.get("seats", []):
            name = seat.get("name", "")
            if name not in RETRIEVAL_STRATEGIES:
                continue
            tool_calls = seat.get("tool_calls") or {}
            reads = sum(c for t, c in tool_calls.items() if t in READ_TOOLS)
            idx = index_line_count(sid, name)
            flags = []
            if reads == 0:
                flags.append("ZERO-READS")
            if idx is not None and idx > INDEX_LINE_WARN:
                flags.append(f"INDEX>{INDEX_LINE_WARN}L")
            rows.append({
                "label": label, "sid": sid, "seat": name,
                "reads": reads, "index_lines": idx, "flags": flags,
            })
    return rows


def format_tripwire(rows):
    hard = any("ZERO-READS" in r.get("flags", []) for r in rows if not r.get("missing"))
    soft = any(f.startswith("INDEX>") for r in rows if not r.get("missing") for f in r.get("flags", []))
    checked = [r for r in rows if not r.get("missing") and r.get("seat") != "-"]
    if not checked:
        verdict = "NO RETRIEVAL-MEMORY SEATS FOUND (nothing to check)"
    elif hard:
        verdict = "FAIL — a retrieval agent made ZERO decision-time reads (memory is write-only)"
    elif soft:
        verdict = "WARN — index body over budget (accumulating narrative)"
    else:
        verdict = "PASS — all retrieval agents read their memory; indexes within budget"

    lines = [f"TRIPWIRE: {verdict}", ""]
    lines.append(f"{'Group':<10} {'Session':<40} {'Seat':<18} {'reads':>6} {'idx_lines':>9}  flags")
    lines.append("-" * 100)
    for r in rows:
        if r.get("missing"):
            lines.append(f"{r['label']:<10} {r['sid']:<40} {'(eval.json MISSING)'}")
            continue
        idx = "?" if r["index_lines"] is None else str(r["index_lines"])
        flags = ",".join(r["flags"]) if r["flags"] else "ok"
        lines.append(
            f"{r['label']:<10} {r['sid']:<40} {r['seat']:<18} {r['reads']:>6} {idx:>9}  {flags}"
        )
    lines.append("-" * 100)
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
            f"agents/{exp['groups'][0]['seat0']}/pi-session.jsonl"
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
    agent_name = exp["groups"][0]["seat0"]
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


# ---------------------------------------------------------------------------
# Demo report — the rich per-experiment report shown live. Observational,
# builder-to-builder voice. Slope is the hero; total-$ is the horizon-dependent
# consequence; fidelity is the "and it buys nothing" beat where measurable.
# Writes into research/experiments/<id>/reports/<id>.md (replaces the flat Go
# table at that path) plus a sibling <id>-cost.png chart.
# ---------------------------------------------------------------------------

import subprocess

DEMO_AKG_AGENT = "llm-akg-durable"

# The rich analysis is ADDITIVE: it augments the baseline report that
# `poker experiment go` writes rather than replacing it. The contribution lives
# inside these sentinels so re-running is idempotent (strip-then-append) and the
# Go-authored portion above is never touched.
SENTINEL_BEGIN = "<!-- BEGIN eval-analysis -->"
SENTINEL_END = "<!-- END eval-analysis -->"


def _strip_sentinel_block(text):
    pat = re.compile(re.escape(SENTINEL_BEGIN) + r".*?" + re.escape(SENTINEL_END), re.DOTALL)
    return pat.sub("", text).rstrip("\n") + "\n"


def _decision_horizon(token_rows, agent):
    hands = [r["hand"] for r in token_rows
             if r["agent"] == agent and r["kind"] == "decision" and r["hand"] is not None]
    return max(hands) if hands else 0


def _experiment_fidelity(experiment_id):
    """Per-agent fidelity aggregates for agents fidelity.py can parse (akg, wiki).
    Returns {} if fidelity tooling/data is unavailable. Naive prose/no-memory
    agents have no extractor — their absence is itself the auditability point."""
    try:
        sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
        import fidelity
        d, sessions = fidelity.load_layout(experiment_id)
    except Exception:
        return {}
    sess_root = os.path.join(fidelity.REPO, "research", "experiments", experiment_id, "sessions")
    extractors = {"llm-akg-durable": fidelity.extract_akg_claims,
                  "llm-md-wiki": fidelity.extract_wiki_claims}
    per_agent_rows = {}
    for s in sessions:
        sdir = os.path.join(sess_root, s["id"])
        if not os.path.isdir(sdir):
            continue
        try:
            gt = fidelity.load_ground_truth(sdir)
        except Exception:
            continue
        for seat, name in ((0, s["seat0"]), (1, s["seat1"])):
            ext = extractors.get(name)
            if not ext:
                continue
            try:
                rows = fidelity.verify(ext(os.path.join(sdir, "agents", name)), gt, 1 - seat)
            except Exception:
                continue
            per_agent_rows.setdefault(name, []).extend(rows)
    out = {}
    for name, rows in per_agent_rows.items():
        a = fidelity.agg(rows)
        a["buckets"] = fidelity.drift_buckets(rows)
        out[name] = a
    return out


def _render_chart(experiment_id, csv_path, out_png, akg, naive, title):
    skill_dir = os.path.dirname(os.path.abspath(__file__))
    chart_py = os.path.join(skill_dir, "chart.py")
    uv = os.path.expanduser("~/.local/bin/uv")
    runner = ([uv, "run", "--with", "matplotlib", "python3"]
              if os.path.exists(uv) else [sys.executable])
    cmd = runner + [chart_py, "--csv", csv_path, "--out", out_png,
                    "--akg", akg, "--naive", naive, "--title", title]
    try:
        subprocess.run(cmd, check=True, capture_output=True, timeout=300)
        return os.path.exists(out_png)
    except Exception:
        return False


def _pretty(agent):
    return agent.replace("llm-", "").replace("-", " ")


def render_demo_report(experiment_id, exp, token_rows, comparison_rows):
    agents, summary = token_summary(token_rows)
    if DEMO_AKG_AGENT not in summary:
        return None  # not an AKG matchup — leave the standard report alone
    naive = next((a for a in agents if a != DEMO_AKG_AGENT), None)
    if naive is None:
        return None
    a, nv = summary[DEMO_AKG_AGENT], summary[naive]
    if not a["slope_tok_per_hand"] or not nv["slope_tok_per_hand"]:
        return None

    horizon = max(_decision_horizon(token_rows, DEMO_AKG_AGENT),
                  _decision_horizon(token_rows, naive))
    slope_mult = nv["slope_tok_per_hand"] / a["slope_tok_per_hand"]
    cost_mult = (nv["total_cost"] / a["total_cost"]) if a["total_cost"] else float("nan")
    near_tied = cost_mult < 1.3

    out_dir = os.path.join(EXPERIMENTS_DIR, experiment_id, "reports")
    os.makedirs(out_dir, exist_ok=True)
    png_name = f"{experiment_id}-cost.png"
    csv_path = os.path.join(RESULTS_DIR, f"{experiment_id}-tokens.csv")
    chart_ok = _render_chart(
        experiment_id, csv_path, os.path.join(out_dir, png_name),
        DEMO_AKG_AGENT, naive,
        f"{experiment_id} — per-decision context growth ({horizon} hands)")

    def el(s):
        e, l = s["early_prompt_mean"], s["late_prompt_mean"]
        return f"{e:,.0f} → {l:,.0f}" if e is not None and l is not None else "n/a"

    L = []
    L.append(SENTINEL_BEGIN)
    L.append("")
    L.append("## Token cost & fidelity")
    L.append("")
    L.append(f"*Observational. Generated {date.today().isoformat()} from the live "
             f"`{experiment_id}` sessions. Model: {exp.get('model', '?')}.*")
    L.append("")
    # 1. one-line observation
    L.append(f"> Across **{horizon} identical hands**, the two memory strategies "
             f"produced agents whose per-decision context **diverged**: the naive "
             f"agent's prompt grew **{nv['slope_tok_per_hand']:+.0f} tok/hand** versus "
             f"AKG's **{a['slope_tok_per_hand']:+.0f}** — **{slope_mult:.1f}× faster**. "
             f"Total spend was **{cost_mult:.2f}×** "
             f"(${nv['total_cost']:.2f} vs ${a['total_cost']:.2f}).")
    L.append("")
    L.append("**Framing.** Both agents run identical rules, identical tools, identical "
             "model — the *only* difference is the memory substrate. AKG keeps a typed, "
             "queryable graph it edits in small deltas; the naive agent rewrites free-form "
             "notes / re-injects history. Both SDKs are deliberately minimal and leave "
             "strategy to the implementer; this is a measurement, not a pitch.")
    L.append("")

    # 2. slope — the hero
    L.append("## Per-decision context growth (the headline)")
    L.append("")
    L.append("Decision prompt context = `input + cacheRead + cacheWrite` of each "
             "decision call. Slope is least-squares growth per hand.")
    L.append("")
    L.append("| Agent | Decisions | Slope (tok/hand) | Early → late prompt |")
    L.append("|---|---:|---:|---:|")
    L.append(f"| {_pretty(DEMO_AKG_AGENT)} | {a['decisions']} | "
             f"**{a['slope_tok_per_hand']:+.1f}** | {el(a)} |")
    L.append(f"| {_pretty(naive)} (naive) | {nv['decisions']} | "
             f"**{nv['slope_tok_per_hand']:+.1f}** | {el(nv)} |")
    L.append("")
    L.append(f"The naive agent's per-decision context grows **{slope_mult:.1f}× faster**. "
             "Structured deltas keep most of the prompt stable across turns, which also "
             "happens to be cache-friendly; rewriting notes or re-injecting history does not.")
    L.append("")

    # 3. total $ — honest consequence at this horizon
    L.append("## Total cost at this horizon")
    L.append("")
    L.append("| Agent | Total tokens | Total $ |")
    L.append("|---|---:|---:|")
    L.append(f"| {_pretty(DEMO_AKG_AGENT)} | {a['total_tokens']:,} | ${a['total_cost']:.2f} |")
    L.append(f"| {_pretty(naive)} (naive) | {nv['total_tokens']:,} | ${nv['total_cost']:.2f} |")
    L.append("")
    if near_tied:
        L.append(f"At **{horizon} hands the total cost is ≈tied** ({cost_mult:.2f}×) — and "
                 "that is the trap. AKG front-loads a fixed per-hand memory-maintenance "
                 "cost that roughly cancels the naive agent's prompt bloat at short "
                 "horizons. The slopes above show the gap only widens: the longer the "
                 "agent runs, the worse the naive curve gets. See the projection in the chart.")
    else:
        L.append(f"At **{horizon} hands** the naive strategy costs **{cost_mult:.2f}×** as "
                 "much for the same work. This is the slope above, compounded over the "
                 "session — and the slope shows no sign of flattening.")
    L.append("")

    # 4. fidelity beat
    L.append("## Fidelity")
    L.append("")
    fid = _experiment_fidelity(experiment_id)
    if DEMO_AKG_AGENT in fid and naive in fid:
        L.append("Each agent's stated opponent reads were cross-validated against engine "
                 "ground truth (`hands.jsonl`). **Fabrication** = a specific villain holding "
                 "claimed on a hand that never reached showdown; **card/board error** = a "
                 "claim that contradicts the cards actually shown.")
        L.append("")
        L.append("| Agent | Records | Card claims | Fabrications | Card errors | Board errors |")
        L.append("|---|---:|---:|---:|---:|---:|")
        for name in (DEMO_AKG_AGENT, naive):
            f = fid[name]
            L.append(f"| {_pretty(name)} | {f['records']} | {f['card_claims']} | "
                     f"{f['fabrications']} | {f['card_errors']} | {f['board_errors']} |")
        L.append("")
        both_clean = all(fid[n]["fabrications"] == 0 and fid[n]["card_errors"] == 0
                         and fid[n]["board_errors"] == 0 for n in (DEMO_AKG_AGENT, naive))
        if both_clean:
            L.append("Both agents recalled their opponent with **zero hard-fact errors**. "
                     "The extra spend buys no fidelity — same accuracy, more tokens.")
        else:
            L.append("Read the fabrication and error columns against cost: accuracy is the "
                     "*other* axis of the frontier.")
    else:
        L.append(f"Only `{DEMO_AKG_AGENT}` keeps structured, machine-checkable per-hand "
                 "records, so it is the only agent this tool can cross-validate against "
                 "ground truth. The naive agent's memory is free-form prose / raw history — "
                 "**not structured-auditable**: in production you could not verify what it "
                 "\"remembers\" either. A manual spot-check of the naive agent's notes is "
                 "below where available.")
        L.append("")
        manual = os.path.join(out_dir, f"{experiment_id}-fidelity-manual.md")
        if os.path.exists(manual):
            with open(manual) as f:
                L.append(f.read().strip())
        else:
            L.append("> _Manual fidelity pass pending — see "
                     f"`{experiment_id}-fidelity-manual.md`._")
    L.append("")

    # 5. chart
    L.append("## Cost trajectory")
    L.append("")
    if chart_ok:
        L.append(f"![Per-decision context growth]({png_name})")
        L.append("")
        L.append("Solid = least-squares fit over measured hands. Dashed = linear projection "
                 "past the measured horizon (assumes the slope holds). Don't trust the "
                 "projection? Run it yourself — see *Reproduce* below.")
    else:
        L.append("_(chart unavailable — install matplotlib, or run "
                 f"`uv run --with matplotlib python3 .claude/skills/eval-analysis/chart.py "
                 f"--csv {csv_path} --out {os.path.join(out_dir, png_name)} "
                 f"--akg {DEMO_AKG_AGENT} --naive {naive}`)_")
    L.append("")

    # 6. limitations
    L.append("## What this does and does not show")
    L.append("")
    L.append(f"- **Scope:** {len(comparison_rows)} sessions, single model "
             f"({exp.get('model', '?')}), seat-mirrored across {exp.get('hands_per_session','?')} "
             "planned hands. Not a claim about all workloads or models.")
    L.append("- **Chips are diagnostic only.** Heads-up variance over this few hands is "
             "large; the per-session table above is a sanity check, not a result.")
    L.append("- **Fidelity is measured only where structured records exist.** Prose / "
             "history-stuffing agents are not machine-auditable here (that is itself a finding).")
    L.append("- **The projection is extrapolation,** not data — clearly dashed, and "
             "reproducible at any horizon (below).")
    L.append("")

    # 7. reproduce footer
    L.append("## Reproduce / extend this")
    L.append("")
    L.append("Everything here regenerates from checked-in artifacts. Public repo, no hidden config.")
    L.append("")
    L.append("```bash")
    L.append(f"# re-run the experiment (any horizon: edit hands_per_session first)")
    L.append(f"# research/experiments/{experiment_id}/{experiment_id}.json")
    L.append(f"poker experiment go {experiment_id}")
    L.append("")
    L.append(f"# regenerate this report + chart")
    L.append(f"python3 .claude/skills/eval-analysis/analyze.py {experiment_id} --tokens")
    L.append("```")
    L.append("")

    L.append(SENTINEL_END)
    block = "\n".join(L)

    # Additive: augment the baseline report `poker experiment go` wrote rather than
    # replacing it. Strip any prior eval-analysis block (idempotent) then append.
    out_path = os.path.join(out_dir, f"{experiment_id}.md")
    if os.path.exists(out_path):
        with open(out_path) as f:
            base = _strip_sentinel_block(f.read())
    else:
        base = (f"# {experiment_id}\n\n_Per-session diagnostic table not present yet — "
                f"run `poker experiment go {experiment_id}` to add it above this analysis._\n")
    with open(out_path, "w") as f:
        f.write(base.rstrip("\n") + "\n\n" + block + "\n")
    return out_path


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
    rows = build_comparison_table(exp)
    table_text = format_table(rows)

    groups = exp.get("groups", [])
    strategies = " vs ".join(g.get("seat0", "?") for g in groups[:2])
    highlights = textwrap.dedent(f"""\
        Experiment : {args.experiment_id}
        Model      : {exp.get('model', '?')}
        Strategies : {strategies}
        Hands/session: {exp.get('hands_per_session', '?')}""")

    tripwire_text = format_tripwire(tripwire_rows(exp))

    print(highlights)
    print()
    print(tripwire_text)
    print()
    print(table_text)

    extra_sections = [("Tripwire (read-usage + index size)", tripwire_text)]

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

        demo_path = render_demo_report(args.experiment_id, exp, token_rows, rows)
        if demo_path:
            print(f"\nDemo report written to: {demo_path}")

    report_path = write_report(args.experiment_id, exp, table_text, highlights, extra_sections)
    print(f"\nReport written to: {report_path}")


if __name__ == "__main__":
    main()
