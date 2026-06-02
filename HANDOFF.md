# HANDOFF — implement `llm-md-single` and `llm-md-wiki`

**Task:** build the two prose-memory agents to their specs, then run the Phase 2
brackets. The specs are committed and authoritative — this file is the build plan and
the context that isn't yet in the committed docs.

Read first:
- [`docs/llm-md-single-spec.md`](docs/llm-md-single-spec.md) — single-file contract
- [`docs/llm-md-wiki-spec.md`](docs/llm-md-wiki-spec.md) — linked-pages contract
- [`docs/kb/adding-a-memory-strategy.md`](docs/kb/adding-a-memory-strategy.md) — how-to
- All `go`/`npm` commands run from `engine/`. Build agents from `engine/pi-agents/`.

---

## The one design choice that matters

The model updates the notes after each hand — not code. AKG's stated profile matches
ground truth to the digit *because* `rebuildOpponent`/`rebuildPatterns` compute it in
code. The prose rungs deliberately have the model keep its own notes current, because
that drift is the failure mode under test. If you find yourself computing stats in code
and writing them to the file, stop — that defeats the experiment.

How it works (same shape for both agents): every interaction is a fresh session whose
only context is the file(s). Decide = load the file(s), make the move. Update = load the
file(s) + the hand that just ended, write the file(s) back. Nothing carries across hands
except what's on disk, so the file is the only memory by construction — you don't have to
enforce it.

The update step:
- Lives in `MemoryPolicy.afterHandEnd` (the shared runner calls it with the completed
  hand — see `engine/pi-agents/shared/src/runner.ts`, `hand_end` case).
- Runs the same model + thinking level as decisions (`PI_POKER_MODEL` /
  `PI_POKER_THINKING_LEVEL`) — the lineup holds the model constant.
- Writes its transcript to a **separate `update-session.jsonl`** so its token cost is
  measurable apart from decision cost.
- On failure / empty output: keep the prior file(s) unchanged, log to `stderr.log`. A
  failed update must never corrupt or truncate memory.

The hand summary fed to the update step should reuse the formatting already in
`engine/pi-agents/llm-fullhistory/src/history.ts` (`formatCompletedHand`).

**The update step is the only new mechanic — no existing agent makes an LLM call in
`afterHandEnd`, so there's no copy-paste exemplar.** Build it from the existing Pi
primitives: a one-shot session is just `createPiSession`-style setup
(`shared/src/pi-session.ts`, or the deps pattern in `llm-akg-durable/src/runtime.ts`)
with its own prompt and **no poker/AKG tools registered**; then `session.prompt(...)` →
read `session.getLastAssistantText()` → write the file(s) → `session.dispose()`. Read the
model/thinking level from the same env vars as decisions, and write the transcript to
`update-session.jsonl` via the session's `exportToJsonl`, the way
`PiDecisionEngine.persistAndDispose` writes `pi-session.jsonl`.

---

## Recommended order: `llm-md-single` first, then `llm-md-wiki`

`md-single` establishes the update step with no custom tools; `md-wiki` reuses the same
update pattern and adds read tools + a session factory. If the update wiring looks
genuinely shared after the first agent, consider lifting a small helper into
`pi-agents/shared/` — but don't pre-abstract; duplicate first, extract only if it earns it.

### Step 1 — `llm-md-single`

1. `poker strategy new llm-md-single` (scaffolds the package + registry entry).
2. Start from `llm-fullhistory/` — it's the simplest stateful strategy and its prompt
   injection and hand formatting are directly reusable.
3. `beforeDecision`: read `memory_dir/notes.md` (empty "no notes yet" section if absent)
   and inject the **whole file** as one prompt section. No tools.
4. `afterHandEnd`: ask the model to rewrite the notes (current notes + this hand's summary
   → full updated file), overwrite `notes.md`. Full rewrite, not a diff.
5. Decision prompt: strict JSON-only final action. The shared parser
   (`shared/src/action.ts` `parseActionResponse`) already tolerates prose/code-fence-
   wrapped JSON, so a reasoning leak no longer costs a retry; keep the prompt strict
   regardless.
6. Artifacts: `notes.md`, `update-session.jsonl`, `pi-session.jsonl`, `stderr.log`.

### Step 2 — `llm-md-wiki`

1. `poker strategy new llm-md-wiki`.
2. Mirror `llm-akg-durable/src/runtime.ts` (`createDurableSessionFactory`) for the
   custom-tools path — this agent does **not** use `createStandardDecisionEngine`.
3. Read-only Pi tools: `md_list_pages`, `md_read_page` (deterministic not-found result).
   The agent follows `[[links]]` by calling `md_read_page` on targets.
4. `beforeDecision`: inject **only** the `wiki/villain.md` root page as the index, plus a
   reminder the tools exist. Active retrieval — do not dump the whole wiki.
5. `afterHandEnd`: ask the model to update the pages (root summary, pattern pages,
   optional hand pages) and keep the `[[links]]`. The runtime **counts but does not
   auto-repair** broken/duplicate/orphan links — that rot is the failure mode under test.
6. Artifacts: `wiki/` dir, `update-session.jsonl`, `pi-session.jsonl`, `stderr.log`.

### Verify (both)

- `cd engine/pi-agents && npm run build` produces `poker-agent-llm-md-single` /
  `poker-agent-llm-md-wiki`.
- Unit tests in `test/` (file read/inject, update-prompt assembly, tools, fake-decision
  smoke). LLM behavior isn't unit-tested — validate by running a short match and
  inspecting artifacts:
  `poker match run --agent0 llm-md-single --agent1 heuristic --hands 10`.
- Confirm `update-session.jsonl` is written and decision vs update tokens are
  separable.

---

## After the agents build — run Phase 2

Definitions are committed and ready (seat-mirrored, non-mirror, reported on the
fidelity-vs-cost frontier, chips diagnostic only):

| Experiment | Hands × seeds | Status |
|---|---|---|
| `phase2-fullhistory-vs-akg` | 80 × 5 | **runnable now** (both agents built) |
| `phase2-mdsingle-vs-akg` | 150 × 5 | after `llm-md-single` builds |
| `phase2-wiki-vs-akg` | 200 × 5 | after `llm-md-wiki` builds (headline) |

Run with `poker experiment go <id>`. The remaining Phase 2 prerequisite is standardizing
the profile-fidelity cross-validation + tokens-per-decision as first-class eval analyses
(see Follow-up work); the `eval-analysis` skill's `analyze.py` also needs fixing for the
nested `research/experiments/<id>/sessions/` layout.

---

## Git state

- Branch **`docs/memory-ladder-overhaul`**, ahead of `main`, **not merged, not pushed**.
- Uncommitted: the two specs, three `phase2-*` experiment defs + READMEs, doc-index edits
  in `AGENTS.md`/`docs/research.md`, and this file.
- `research/experiments/mirror-match-500/` is untracked; heavy artifacts (`memory.akg`,
  `pi-session.jsonl`, `*.log`) are gitignored — only lightweight records get `git add`ed.

---

## Follow-up work (after the agents are built and the experiments run)

Once the two agents exist and the Phase 2 brackets have run, **the sole focus becomes
making session data easier to visualize and interpret** — turning the raw artifacts
(`hands.jsonl`, `memory-export.json` / `notes.md` / `wiki/`, `pi-session.jsonl`,
`update-session.jsonl`) into the fidelity-vs-cost curves and per-decision views that make
the thesis legible at a glance, rather than something reconstructed by hand each time.

Concrete pieces already identified:

- **Standardize the profile-fidelity cross-validation** (stated opponent model vs engine
  ground truth) as a first-class eval analysis. The ad-hoc prototype is preserved below;
  it's parameterized by session dir, and its c-bet heuristic is intentionally fuzzy —
  exact-match the unambiguous stats (3-bets, river aggression), treat c-bet as directional.
- **Tokens-per-decision extraction** from `pi-session.jsonl` (+ `update-session.jsonl`
  for the markdown agents' update cost) — the cost axis of the frontier.
- **Fix the `eval-analysis` skill's `analyze.py`** for the nested
  `research/experiments/<id>/sessions/` layout (it currently expects flat
  `research/sessions/<id>/eval.json`).

Preserved prototype — the fidelity cross-validation instrument:

```python
#!/usr/bin/env python3
# Cross-validate each agent's stated opponent model vs engine ground truth.
# Usage: python3 fidelity.py <session_dir>   (dir with hands.jsonl + agents/*/memory-export.json)
import json, sys, glob, os

session = sys.argv[1]
hands = [json.loads(l) for l in open(f"{session}/hands.jsonl")]

# --- ground truth per seat, from the full-info engine log ---
gt = {0: {}, 1: {}}
for s in (0, 1):
    gt[s] = dict(threebet=0, river_aggr=0, cbet_opp=0, cbet_fold=0, river_face=0, river_fold=0)
for h in hands:
    by_street = {}
    for a in h["actions"]:
        by_street.setdefault(a["street"], []).append(a)
    pf = [a for a in by_street.get("preflop", []) if a["action"] != "post_blind"]
    seen_raise = False; pfa = None
    for a in pf:
        if a["action"] == "raise":
            if seen_raise: gt[a["seat"]]["threebet"] += 1
            seen_raise = True; pfa = a["seat"]
    flop = by_street.get("flop", [])
    if pfa is not None and flop:
        for a in flop:
            if a["action"] == "bet":
                if a["seat"] == pfa:
                    v = 1 - pfa; gt[v]["cbet_opp"] += 1
                    resp = [x for x in flop[flop.index(a)+1:] if x["seat"] == v]
                    if resp and resp[0]["action"] == "fold": gt[v]["cbet_fold"] += 1
                break
    river = by_street.get("river", [])
    for a in river:
        if a["action"] in ("bet", "raise"): gt[a["seat"]]["river_aggr"] += 1
    for i, a in enumerate(river):
        if a["action"] == "bet":
            o = 1 - a["seat"]; resp = [x for x in river[i+1:] if x["seat"] == o]
            if resp:
                gt[o]["river_face"] += 1
                if resp[0]["action"] == "fold": gt[o]["river_fold"] += 1

# --- stated model per agent (villain = the OTHER seat) ---
for path in sorted(glob.glob(f"{session}/agents/*/memory-export.json")):
    g = json.load(open(path))
    opp = next((n for n in g["nodes"] if n["type"] == "opponent"), None)
    pats = {n["id"]: n for n in g["nodes"] if n["type"] == "pattern"}
    print(f"\n{os.path.basename(os.path.dirname(path))}: opponent meta={opp.get('meta') if opp else None}")
    print("  patterns:", {k: p.get("body","")[:60] for k, p in pats.items()})
# Compare opp.meta / pattern counts against gt[villain_seat].
print("\nGROUND TRUTH:", gt)
```

For the prose agents this generalizes: parse the *stated* stats out of `notes.md` /
`wiki/villain.md` instead of `memory-export.json`, and the error-vs-hand-count curve is
the fidelity axis. Tokens-per-decision is extractable from prompts in
`agents/*/pi-session.jsonl` (+ `update-session.jsonl` for update cost).
