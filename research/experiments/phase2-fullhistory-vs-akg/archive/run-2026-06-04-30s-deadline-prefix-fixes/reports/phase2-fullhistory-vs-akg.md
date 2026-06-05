# Experiment: phase2-fullhistory-vs-akg

**Hypothesis:** Phase 2 cost-ceiling bracket: raw full-history prompting (llm-fullhistory) versus a typed, queryable AKG graph (llm-akg-durable), heads-up and NON-mirror, seat-mirrored across 2 seeds (4 sessions) so positional and blind-rotation effects cancel. llm-fullhistory keeps no maintained memory - it stuffs every prior hand back into the prompt - so per-decision input tokens grow O(N) and the prompt balloons as the session runs (a prior 200-hand run was prohibitively expensive). Horizon is deliberately SHORT (80 hands) to capture the linear token-growth slope and the early curve without runaway cost; if it hits the context ceiling first, that hand count IS the result. OBSERVATIONAL (we are learning, not predicting a winner): we measure (1) profile fidelity - full-history is lossless in principle (the raw transcript is all there) but the model must re-derive everything in-context every decision, so we observe whether its stated reads stay accurate; against the AKG agent we cross-validate its self-maintained graph against engine ground truth in hands.jsonl. (2) Honest TOTAL token cost - for the AKG agent, decision-prompt tokens PLUS the post-hand update-session tokens; for full-history, the climbing decision-prompt tokens (it has no update step). Chip outcomes are DIAGNOSTIC ONLY. expected_direction is intentionally empty: reported as measured fidelity-vs-cost curves, not pass/fail predictions.

## Summary

| Metric | Control (n=2) | Treatment (n=2) | Δ (T-C) | Direction |
|---|---:|---:|---:|---|
| chips/hand | -1.12 | 0.21 | +1.34 | - |
| session duration (s) | 4088 | 3644 | -444 | - |
| preflop-only rate | 63.1% | 71.2% | +8.1pp | - |
| showdown rate | 1.9% | 3.8% | +1.9pp | - |
| fallback actions/session | 116.50 | 110.50 | -6.00 | - |
| decision prompts/session | 128.00 | 70.50 | -57.50 | - |

## Tool Use

| Metric | Control (n=2) | Treatment (n=2) | Δ (T-C) | Direction |
|---|---:|---:|---:|---|
| akg_get_node/hand | 0.84 | 0.00 | -0.84 | - |
| akg_get_node/session | 67.00 | 0.00 | -67.00 | - |
| akg_list_nodes/hand | 0.03 | 0.00 | -0.03 | - |
| akg_list_nodes/session | 2.00 | 0.00 | -2.00 | - |

## Per-Session Results

| Group | Session | Seed | Agent | Opponent | Chips Δ | Chips/Hand | Duration (s) | Preflop-only | Showdown |
|---|---|---:|---|---|---:|---:|---:|---:|---:|
| control | akg-vs-fullhistory-1 | 1 | llm-akg-durable [llm-akg-durable/0.1.0] | llm-fullhistory [llm-fullhistory/0.1.0] | -90 | -1.12 | 3954 | 63.7% | 2.5% |
| control | akg-vs-fullhistory-2 | 2 | llm-akg-durable [llm-akg-durable/0.1.0] | llm-fullhistory [llm-fullhistory/0.1.0] | -90 | -1.12 | 4222 | 62.5% | 1.2% |
| treatment | fullhistory-vs-akg-1 | 1 | llm-fullhistory [llm-fullhistory/0.1.0] | llm-akg-durable [llm-akg-durable/0.1.0] | -64 | -0.80 | 3708 | 70.0% | 5.0% |
| treatment | fullhistory-vs-akg-2 | 2 | llm-fullhistory [llm-fullhistory/0.1.0] | llm-akg-durable [llm-akg-durable/0.1.0] | +98 | 1.23 | 3581 | 72.5% | 2.5% |

