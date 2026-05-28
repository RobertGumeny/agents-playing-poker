# Experiment: test-2b-retrieval-throttle

**Hypothesis:** Throttling durable retrieval should reduce tool use and session time without hurting chips per hand.

## Summary

| Metric | Control (n=5) | Treatment (n=5) | Δ (T-C) | Direction |
|---|---:|---:|---:|---|
| chips/hand | 5.12 | 0.25 | -4.87 | ❌ decrease (expected increase) |
| session duration (s) | 465 | 656 | +191 | ❌ increase (expected decrease) |
| preflop-only rate | 46.4% | 60.0% | +13.6pp | ❌ increase (expected decrease) |
| showdown rate | 8.8% | 12.8% | +4.0pp | - |
| fallback actions/session | 0.00 | 1.00 | +1.00 | - |
| decision prompts/session | 62.60 | 77.00 | +14.40 | - |
| akg_get_opponent/session | 33.40 | 48.40 | +15.00 | ❌ increase (expected decrease) |

## Tool Use

| Metric | Control (n=5) | Treatment (n=5) | Δ (T-C) | Direction |
|---|---:|---:|---:|---|
| akg_get_opponent/hand | 1.34 | 1.94 | +0.60 | - |
| akg_get_opponent/session | 33.40 | 48.40 | +15.00 | ❌ increase (expected decrease) |
| akg_get_pattern/hand | 0.02 | 0.06 | +0.05 | - |
| akg_get_pattern/session | 0.40 | 1.60 | +1.20 | - |
| akg_list_hands/hand | 0.00 | 0.06 | +0.06 | - |
| akg_list_hands/session | 0.00 | 1.60 | +1.60 | - |
| akg_list_patterns/hand | 0.15 | 0.80 | +0.65 | - |
| akg_list_patterns/session | 3.80 | 20.00 | +16.20 | - |

## Per-Session Results

| Group | Session | Seed | Agent | Opponent | Chips Δ | Chips/Hand | Duration (s) | Preflop-only | Showdown |
|---|---|---:|---|---|---:|---:|---:|---:|---:|
| control | akg-durable-vs-stateless-test-1 | 1 | llm-akg-durable [llm-akg-durable/0.1.0] | llm-stateless [llm-stateless/0.1.0] | +143 | 5.72 | 488 | 44.0% | 8.0% |
| control | akg-durable-vs-stateless-test-2 | 2 | llm-akg-durable [llm-akg-durable/0.1.0] | llm-stateless [llm-stateless/0.1.0] | +152 | 6.08 | 554 | 44.0% | 12.0% |
| control | akg-durable-vs-stateless-test-3 | 3 | llm-akg-durable [llm-akg-durable/0.1.0] | llm-stateless [llm-stateless/0.1.0] | +75 | 3.00 | 428 | 48.0% | 8.0% |
| control | akg-durable-vs-stateless-test-4 | 4 | llm-akg-durable [llm-akg-durable/0.1.0] | llm-stateless [llm-stateless/0.1.0] | +108 | 4.32 | 466 | 48.0% | 16.0% |
| control | akg-durable-vs-stateless-test-5 | 5 | llm-akg-durable [llm-akg-durable/0.1.0] | llm-stateless [llm-stateless/0.1.0] | +162 | 6.48 | 390 | 48.0% | 0.0% |
| treatment | akg-durable-retrieval-test-1 | 1 | llm-akg-durable [llm-akg-durable@exp-0.1.3-throttle] | llm-stateless [llm-stateless/0.1.0] | +11 | 0.44 | 725 | 60.0% | 32.0% |
| treatment | akg-durable-retrieval-test-2 | 2 | llm-akg-durable [llm-akg-durable@exp-0.1.3-throttle] | llm-stateless [llm-stateless/0.1.0] | +13 | 0.52 | 769 | 60.0% | 8.0% |
| treatment | akg-durable-retrieval-test-3 | 3 | llm-akg-durable [llm-akg-durable@exp-0.1.3-throttle] | llm-stateless [llm-stateless/0.1.0] | +11 | 0.44 | 549 | 64.0% | 8.0% |
| treatment | akg-durable-retrieval-test-4 | 4 | llm-akg-durable [llm-akg-durable@exp-0.1.3-throttle] | llm-stateless [llm-stateless/0.1.0] | -4 | -0.16 | 598 | 64.0% | 4.0% |
| treatment | akg-durable-retrieval-test-5 | 5 | llm-akg-durable [llm-akg-durable@exp-0.1.3-throttle] | llm-stateless [llm-stateless/0.1.0] | 0 | 0.00 | 641 | 52.0% | 12.0% |

