# phase2-fullhistory-vs-akg

Phase 2 **cost-ceiling bracket**: `llm-fullhistory` vs `llm-akg-durable`, heads-up,
non-mirror, seat-mirrored across five seeds. See the `hypothesis` in
[`phase2-fullhistory-vs-akg.json`](phase2-fullhistory-vs-akg.json).

## ✅ Runnable now

Both agents are built. `poker experiment go phase2-fullhistory-vs-akg`.

## Why only 80 hands

`llm-fullhistory` re-injects every prior hand into the prompt, so per-decision input
tokens grow **O(N)** and cost balloons late in a session — a prior 200-hand run was
prohibitively expensive. The short horizon captures the linear token-growth slope and
the early curve cheaply. If the context ceiling is hit before hand 80, that hand count
is itself the result (`docs/research.md` → "strategies run to different hand counts on
purpose").

## Reporting

Report on the **fidelity-vs-cost frontier** (`docs/research.md` → Metrics), not chips.
The contrast to watch: AKG's precomputed-profile reads stay roughly flat per decision
while full-history climbs linearly. The standardized fidelity + tokens-per-decision
analyses are still pending in the eval tooling (working logic preserved in `HANDOFF.md`);
`expected_direction` carries forward-looking metric keys treated as informational until
those metrics are wired in.

Sibling rungs: [`phase2-mdsingle-vs-akg`](../phase2-mdsingle-vs-akg/) and
[`phase2-wiki-vs-akg`](../phase2-wiki-vs-akg/) (both await their agents being built).
