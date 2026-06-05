# Eval Analysis: phase2-fullhistory-vs-akg

**Generated**: 2026-06-04
**Hypothesis**: Phase 2 cost-ceiling bracket: raw full-history prompting (llm-fullhistory) versus a typed, queryable AKG graph (llm-akg-durable), heads-up and NON-mirror, seat-mirrored across 2 seeds (4 sessions) so positional and blind-rotation effects cancel. llm-fullhistory keeps no maintained memory - it stuffs every prior hand back into the prompt - so per-decision input tokens grow O(N) and the prompt balloons as the session runs (a prior 200-hand run was prohibitively expensive). Horizon is deliberately SHORT (80 hands) to capture the linear token-growth slope and the early curve without runaway cost; if it hits the context ceiling first, that hand count IS the result. OBSERVATIONAL (we are learning, not predicting a winner): we measure (1) profile fidelity - full-history is lossless in principle (the raw transcript is all there) but the model must re-derive everything in-context every decision, so we observe whether its stated reads stay accurate; against the AKG agent we cross-validate its self-maintained graph against engine ground truth in hands.jsonl. (2) Honest TOTAL token cost - for the AKG agent, decision-prompt tokens PLUS the post-hand update-session tokens; for full-history, the climbing decision-prompt tokens (it has no update step). Chip outcomes are DIAGNOSTIC ONLY. expected_direction is intentionally empty: reported as measured fidelity-vs-cost curves, not pass/fail predictions.

## Summary

Experiment : phase2-fullhistory-vs-akg
Model      : anthropic:claude-sonnet-4-6
Expected   : chips/hand ?
Control    : +1.12 c/h (mean)
Treatment  : +0.21 c/h (mean)
Delta      : -0.91 c/h
Confirmed  : NO

## Comparison Table

```
Group        Session                                       chips_delta  hands     c/h    sdr  fallbacks memory
------------------------------------------------------------------------------------------------------------------------
control      akg-vs-fullhistory-1                          +        90     80   +1.12  2.50%        109 n=0 e=0
control      akg-vs-fullhistory-2                          +        90     80   +1.12  1.25%        124 n=0 e=0

treatment    fullhistory-vs-akg-1                                 -64     80   -0.80  5.00%        109 n=0 e=0
treatment    fullhistory-vs-akg-2                          +        98     80   +1.23  2.50%        112 n=0 e=0
------------------------------------------------------------------------------------------------------------------------
                                                      control mean c/h    +1.12
                                                    treatment mean c/h    +0.21
                                           delta (treatment - control)    -0.91
```

## Token Cost vs. Hand Count

```

Token cost vs. hand count (decision prompt context = input+cacheRead+cacheWrite)
Agent                   decs  upds slope tok/hand    early→late prompt  upd prompt    total tok   total $
---------------------------------------------------------------------------------------------------------
llm-akg-durable          286   304           +4.5          1,605→1,938       3,960   12,528,458     28.39
llm-fullhistory          163     0         +101.0            792→8,048           0    1,960,096      5.06
---------------------------------------------------------------------------------------------------------
slope = least-squares growth of decision prompt context per hand (≈flat ⇒ structured-memory cost win; steep ⇒ context ballooning).
Per-call series written to: docs/research/results/phase2-fullhistory-vs-akg-tokens.csv
```
