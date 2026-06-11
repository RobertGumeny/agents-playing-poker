---
name: "eval-analysis"
description: "Analyze experiment results across sessions. Reads eval.json for each session in an experiment, builds a comparison table (chips/hand, showdown rate, memory graph shape), and writes a markdown report. Optional --traces flag analyzes pattern-reasoning mentions in pi-session.jsonl; optional --hand N drills into a specific hand across all sessions. Token-cost growth and memory fidelity are produced natively by `poker experiment go`, not this skill."
---

# Eval Analysis Workflow

Use this skill when asked to analyze, review, or summarize the results of a named experiment (e.g. `test-2d-showdown-pattern`, `test-2e-pattern-confidence`).

## Step 1: Resolve the experiment ID

The user will typically name the experiment. If ambiguous, list available experiments:

```bash
ls research/experiments/
```

## Step 2: Run the analysis script

Run from the repository root. The script reads the experiment definition JSON, loads `eval.json` for every session (control and treatment), and writes a report.

**Basic comparison table:**

```bash
python3 .claude/skills/eval-analysis/analyze.py <experiment-id>
```

**With pattern-reasoning trace analysis** (scans pi-session.jsonl for assistant turns mentioning KEYWORD):

```bash
python3 .claude/skills/eval-analysis/analyze.py <experiment-id> --traces <keyword>
```

`keyword` defaults to `"pattern"` if omitted. Use the specific pattern slug (e.g. `strong-showdown-caller`, `folds-to-cbet`) for targeted analysis.

**With hand drill-down** (extracts context around hand N from each session's pi-session.jsonl):

```bash
python3 .claude/skills/eval-analysis/analyze.py <experiment-id> --hand <N>
```

**Token-cost growth and memory fidelity** (the fidelity-vs-**cost** frontier, metric #2 in `research.md`) are now produced by **native Go**: `poker experiment go` folds a Token Cost section, a Fidelity section, and a per-call `reports/<id>-tokens.csv` directly into the comparison report — no Python step. The former `--tokens` flag is retired.

Flags are combinable.

## Step 3: Interpret stdout highlights

The script prints a summary block first:

```
Experiment : test-2e-pattern-confidence
Model      : anthropic:claude-sonnet-4-6
Expected   : chips/hand increase
Control    : +3.21 c/h (mean)
Treatment  : +4.87 c/h (mean)
Delta      : +1.66 c/h
Confirmed  : YES
```

**Confirmed = YES** means the treatment mean moved in the expected direction. It does not imply statistical significance — note sample sizes and variance across sessions.

## Step 4: Read the comparison table

Columns:
- `chips_delta` — net chips for the focal agent across the full session
- `hands` — actual hand count (may differ from `hands_per_session`)
- `c/h` — chips per hand (the primary comparable metric across groups)
- `sdr` — showdown rate
- `fallbacks` — fallback action count (high values indicate agent parsing failures)
- `memory` — `n=TOTAL[type:count,…] e=TOTAL[relation:count,…]`: total node/edge counts plus the top three node types and edge relations by count. Agents author an open vocabulary (their own node types and relations), so this reflects whatever the agent actually created rather than fixed `pattern`/`supported_by` keys.

## Step 5: Check the written report

The script writes `docs/research/results/<experiment-id>-analysis.md`. Surface the path to the user and confirm whether they want additional sections (e.g. --traces or --hand drill).

## Notes on pi-session.jsonl size

Sessions can be 2–5 MB with very long lines (each line = one RPC message). The `--traces` mode reads line-by-line in Python — do not try to `cat` or `grep` these files directly for content analysis; use the script. For a quick presence check:

```bash
wc -l research/sessions/<id>/agents/<agent>/pi-session.jsonl
```
