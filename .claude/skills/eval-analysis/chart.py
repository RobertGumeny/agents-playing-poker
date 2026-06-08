#!/usr/bin/env python3
"""
Render the per-decision context-growth chart for a Phase 2 demo report.

Standalone so it can run under an ephemeral matplotlib environment (e.g.
`uv run --with matplotlib python3 chart.py ...`) while the main analyze.py stays
dependency-free. Reads the per-call token CSV that `analyze.py --tokens` writes
and plots decision prompt context (input+cacheRead+cacheWrite) vs hand number:
one series per agent, a solid least-squares fit over the measured range, and a
dashed linear projection past the measured horizon.

Slope is the hero metric. The projection is honest extrapolation of the measured
fit only; it is clearly dashed and labelled so nobody mistakes it for data.

Usage:
  chart.py --csv tokens.csv --out cost.png --akg llm-akg-durable \
           --naive llm-md-wiki [--project-to 300] [--title "..."]
"""

import argparse
import csv
import sys


def load_points(csv_path, agent):
    xs, ys = [], []
    with open(csv_path) as f:
        for r in csv.DictReader(f):
            if r["agent"] != agent or r["kind"] != "decision":
                continue
            if not r["hand"]:
                continue
            try:
                xs.append(int(r["hand"]))
                ys.append(int(r["prompt_tokens"]))
            except (ValueError, TypeError):
                continue
    return xs, ys


def fit(xs, ys):
    n = len(xs)
    if n < 2:
        return None, None
    sx, sy = sum(xs), sum(ys)
    sxx = sum(x * x for x in xs)
    sxy = sum(x * y for x, y in zip(xs, ys))
    denom = n * sxx - sx * sx
    if denom == 0:
        return None, None
    slope = (n * sxy - sx * sy) / denom
    intercept = (sy - slope * sx) / n
    return slope, intercept


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--csv", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--akg", required=True)
    ap.add_argument("--naive", required=True)
    ap.add_argument("--project-to", type=int, default=0,
                    help="Hand number to project fitted lines out to (0 = 2x measured horizon)")
    ap.add_argument("--title", default="Per-decision context growth")
    args = ap.parse_args()

    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt

    series = [
        (args.akg, "#1b9e77", "AKG (durable, structured)"),
        (args.naive, "#d95f02", args.naive.replace("llm-", "").replace("-", " ") + " (naive)"),
    ]

    fig, ax = plt.subplots(figsize=(9, 5.2))
    max_measured = 0
    fits = {}
    for agent, color, label in series:
        xs, ys = load_points(args.csv, agent)
        if not xs:
            continue
        max_measured = max(max_measured, max(xs))
        ax.scatter(xs, ys, s=10, alpha=0.28, color=color, edgecolors="none")
        slope, intercept = fit(xs, ys)
        fits[agent] = slope
        if slope is None:
            continue
        lo, hi = min(xs), max(xs)
        ax.plot([lo, hi], [slope * lo + intercept, slope * hi + intercept],
                color=color, lw=2.4,
                label=f"{label}  (+{slope:.0f} tok/hand)")

    project_to = args.project_to or max_measured * 2
    if project_to > max_measured:
        ax.axvline(max_measured, color="#999999", lw=0.8, ls=":", alpha=0.7)
        ax.text(max_measured, ax.get_ylim()[1] * 0.97, " measured →| projected",
                fontsize=8, color="#666666", va="top")
        for agent, color, _label in series:
            slope = fits.get(agent)
            if slope is None:
                continue
            xs, ys = load_points(args.csv, agent)
            _, intercept = fit(xs, ys)
            hi = max(xs)
            ax.plot([hi, project_to],
                    [slope * hi + intercept, slope * project_to + intercept],
                    color=color, lw=2.0, ls="--", alpha=0.85)

    ax.set_xlabel("Hand number")
    ax.set_ylabel("Decision prompt context (tokens)")
    ax.set_title(args.title)
    ax.legend(loc="upper left", fontsize=9, framealpha=0.9)
    ax.grid(True, alpha=0.18)
    ax.margins(x=0.02)
    fig.text(0.99, 0.01,
             "solid = least-squares fit over measured hands · dashed = linear projection (assumes slope holds)",
             ha="right", fontsize=7, color="#888888")
    fig.tight_layout()
    fig.savefig(args.out, dpi=130)
    print(f"wrote {args.out}", file=sys.stderr)


if __name__ == "__main__":
    main()
