# Archived run — phase2-fullhistory-vs-akg (2026-06-04)

This is the first run of `phase2-fullhistory-vs-akg` (sonnet-4-6, 80 hands × 4 seat-mirrored
sessions). It is **retained for reference only — do not treat its chip outcomes as a result.**
A clean re-run supersedes it.

## Why this run is invalid (chip outcomes)

Every fallback action in all four sessions is a `decision_timeout`, and **all of them land on the
`llm-akg-durable` seat** (none on `llm-fullhistory`):

| session | AKG seat | AKG voluntary turns | forced timeouts | forced % |
|---|---|---|---|---|
| akg-vs-fullhistory-1 | 0 | 219 | 109 | 50% |
| akg-vs-fullhistory-2 | 0 | 223 | 124 | 56% |
| fullhistory-vs-akg-1 | 1 | 209 | 109 | 52% |
| fullhistory-vs-akg-2 | 1 | 199 | 112 | 56% |

The server `DecisionDeadline` defaulted to **30s**. The agent-maintained AKG decision loop
(per-decision `akg_get_node` round-trips + reasoning) exceeded 30s about half the time, so the
engine auto-folded/auto-checked the AKG seat. The chip totals (AKG −214 / fullhistory +214 over
320 hands) are an artifact of forced folds, **not** a poker-skill signal.

## What is still usable

The **cost/fidelity curves** (in `analysis/`): AKG keeps per-decision context flat
(~1.6k→1.9k, +4.5 tok/hand) while fullhistory grows O(N) (792→8,048, +101 tok/hand). But honest
total cost inverted at this horizon — AKG $28.39 / 12.5M tok vs fullhistory $5.06 / 1.96M tok —
because the post-hand update step dominated (64% of AKG cost) **and** the update prompt itself
grew O(N) (+33.5 tok/hand) by dumping the full node list every hand.

## What changed before the re-run

Efficiency fixes to `llm-akg-durable` (this same commit/branch):

1. Update prompt = root body + hand summary only (no full node-list dump → update prompt flat).
2. `akg_apply` batched write tool (one turn instead of ~4 round-trips per update).
3. Prompt now records only significant hands (model judgement, no scripted skip).
4. Prompt prefers durable tendency nodes over one-node-per-hand.

Re-run with a **larger `-decision-deadline` (≥120–180s)** so the AKG loop completes, or after
confirming the efficiency fixes pulled per-decision latency under 30s.

See also: `analyze.py`'s `find_focal_seat` reports the treatment agent for every row, so the
generated table's control/treatment c/h both reflect `llm-fullhistory`'s perspective — read
per-seat chips from `eval.json` / `manifest.json` directly.
