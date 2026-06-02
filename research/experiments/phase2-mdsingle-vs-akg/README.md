# phase2-mdsingle-vs-akg

Phase 2 **single-file bracket**: `llm-md-single` vs `llm-akg-durable`, heads-up,
non-mirror, seat-mirrored across five seeds. See the `hypothesis` in
[`phase2-mdsingle-vs-akg.json`](phase2-mdsingle-vs-akg.json).

## ⚠️ Not yet runnable

`llm-md-single` is **not built** (see [`docs/llm-md-single-spec.md`](../../../docs/llm-md-single-spec.md)).
Do not run until:

1. `poker strategy new llm-md-single` is implemented to the spec and registered in
   `engine/pi-agents/registry.json`.
2. `cd engine/pi-agents && npm run build` produces `poker-agent-llm-md-single`.

Then: `poker experiment go phase2-mdsingle-vs-akg`.

## Why 150 hands

Lower than the wiki headline (200): the single file is expected to decay sooner (no
addressable parts — the model rewrites the whole file each hand), and 150 is enough to
let that drift surface. Higher than the full-history bracket (80), which is capped by raw
O(N) token cost rather than note accuracy.

## Reporting

Report on the **fidelity-vs-cost frontier** (`docs/research.md` → Metrics), not chips.
Cross-validate the notes' stated stats against `hands.jsonl`; sum the decision-read and
`update-session.jsonl` rewrite tokens for the cost curve. Standardized fidelity +
tokens-per-decision analyses are still pending in the eval tooling (working logic in
`HANDOFF.md`).

Sibling rungs: [`phase2-wiki-vs-akg`](../phase2-wiki-vs-akg/) and
[`phase2-fullhistory-vs-akg`](../phase2-fullhistory-vs-akg/) (the latter is runnable now).
