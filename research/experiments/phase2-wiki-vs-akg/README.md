# phase2-wiki-vs-akg

The **headline Phase 2 matchup**: `llm-md-wiki` vs `llm-akg-durable`, heads-up,
non-mirror, seat-mirrored across five seeds. See the `hypothesis` field in
[`phase2-wiki-vs-akg.json`](phase2-wiki-vs-akg.json) for the full framing.

## ⚠️ Not yet runnable

This definition is **teed up ahead of its agents**. `llm-md-wiki` is **not built**
(see [`docs/llm-md-wiki-spec.md`](../../../docs/llm-md-wiki-spec.md)). Do not run until:

1. `poker strategy new llm-md-wiki` is implemented to the spec and registered in
   `engine/pi-agents/registry.json`.
2. `cd engine/pi-agents && npm run build` produces `poker-agent-llm-md-wiki`.

Then: `poker experiment go phase2-wiki-vs-akg`.

## Reporting

Report on the **fidelity-vs-cost frontier** (`docs/research.md` → Metrics), not chips.
The standardized profile-fidelity cross-validation (stated stats vs `hands.jsonl`) and
tokens-per-decision extraction are Phase 2 prerequisites still pending in the eval
tooling — the working logic is preserved in `HANDOFF.md`. `expected_direction` carries
forward-looking metric keys (`tokens_per_decision`, `profile_fidelity_error`) that
existing compare tooling will treat as informational until those metrics are wired in.

## The rest of the ladder

The experiment-definition contract is strictly two-group (control/treatment), so the
full ladder benchmark is a set of **pairwise** definitions, not one file. Sibling rungs,
each the same seat-mirrored shape, run to shorter horizons on purpose:

- [`phase2-fullhistory-vs-akg`](../phase2-fullhistory-vs-akg/) — cost-ceiling bracket,
  80 hands. **Runnable now** (both agents built).
- [`phase2-mdsingle-vs-akg`](../phase2-mdsingle-vs-akg/) — single-file
  fidelity/rewrite-cost bracket, 150 hands. Awaits the `llm-md-single` agent.
- optionally `llm-md-single` vs `llm-md-wiki` — isolates the value of addressability
  within the prose family (not yet defined).
