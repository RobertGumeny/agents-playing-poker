**Manual fidelity pass.** `llm-md-single` keeps its memory as one free-form prose file
(`notes.md`), which is not machine-parseable the way AKG's typed records are — so this
was a hand audit. Every opponent-holding claim the agent wrote across all four sessions
was read out and cross-checked against engine ground truth (`hands.jsonl`): does the hand
the claim is attached to actually reach showdown, and do the stated cards match the cards
dealt?

| Session | Showdowns | Holdings recorded | Fabrications | Rank errors | Suit errors |
|---|---:|---:|---:|---:|---:|
| mdsingle-vs-akg-1 | 7 | 7 | 0 | 0 | 0 |
| mdsingle-vs-akg-2 | 4 | 4 | 0 | 0 | 0 |
| akg-vs-mdsingle-1 | 6 | 6 | 0 | 0 | 0 |
| akg-vs-mdsingle-2 | 5 | 3 | 0 | 0 | 1 |

- **Zero fabrications.** The agent never asserts a specific opponent holding on a hand that
  did not reach showdown — it reliably records "No showdown" where there was none. In
  showdown-only information mode, that is the sharpest possible fidelity-drift signal, and
  it never tripped.
- **Zero rank errors.** Every showdown holding it chose to record matches the cards dealt
  (e.g. H19 `5h 5d`, H33 `6h Qc`, H36 `Ts Kd`, H47 `8h Qd`, H55 `Kd 9c`).
- **One blemish:** in `akg-vs-mdsingle-2` hand 28 the agent logged the opponent's hand as
  "89s" (suited) when it was actually `8s 9c` (offsuit) — correct ranks, wrong suitedness.
  The only inaccuracy in the entire pass.

**Reading.** At 60 hands the single-file agent did **not** lose fidelity — it stayed
essentially perfectly accurate. So this bracket tells the same story as the wiki bracket:
at this horizon **fidelity is not the differentiator — cost is** (2.05× here). The
single-file rewrite approach is expected to drift at longer horizons; that is directly
testable by raising `hands_per_session` and re-running (see *Reproduce* below). We are
reporting what we measured, not what we expected.
