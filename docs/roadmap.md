# Post-Launch Roadmap — Ergonomics & Headline Features

**Status:** Active (week of 2026-06-10). **Owner:** Robert Gumeny.

This is the operating plan for the first post-launch iteration, after Agents Playing Poker
went public alongside the [AKG format](https://github.com/RobertGumeny/akg-format). It is a
*roadmap*, not a subsystem contract — when a piece here lands, its normative behavior moves
into the relevant focused doc (`eval-system.md`, `wire-protocol.md`, etc.) and this file
points at it.

For the experiment/science roadmap, see [`research.md`](research.md#experiment-roadmap). This
doc is about the **harness ergonomics and shareable features**, not the experiments.

## Goal

The thesis is proven out in the artifacts. This iteration lowers the activation energy for
visitors and harvests the narrative:

1. Make the harness **plug-and-play** — fewer steps, no leaked internals, one command.
2. Ship **one or two headline, shareable features**.
3. Mine the historical experiments into **writeups**.

Capacity this week is high (several days, multiple coding agents to parallelize across), so
aim for the *polished* version of each item, not the MVP.

## Track A — Ergonomics & artifact consolidation (current focus)

**Architecture principle — the spine of this whole track.** Earlier drafts framed A1 as a
*port checklist* ("port these functions, relocate those scripts"), which turned every script
into a "where does this go?" puzzle. The cleaner frame, and the one this track now follows:

> **Go computes → first-class artifacts.** Anything that derives a *metric* (token cost,
> growth slope, fidelity, comparison table) lives in Go and is emitted as a checked-in
> artifact, extending the existing `eval.json` pattern. Zero computation in Python.
>
> **A `poker analyze` CLI answers ad-hoc queries** (drill a hand, grep a reasoning trace) by
> streaming the session JSONL and returning just the slice — fast, zero-dep, and callable by
> **any** agent harness, not just Claude.
>
> **The `.claude` skill shrinks to a thin doc wrapper** over that CLI — a template other
> harnesses copy, never the capability itself.
>
> **`chart.py` is a *renderer*, not analysis** — optional presentation fed by Go's CSV.

This dissolves the old "relocate the analysis scripts out of `.claude/`" question: nothing
*computational* relocates, because Go owns computation and the skill stops computing.

### A1. One-command full report (Phase 1, ~days 1–2)

**Problem.** `poker experiment go` already runs the Go pipeline (`execAnalyze` → `eval.Compare`
→ `eval.RenderComparisonMarkdown`, `cmd/poker/experiment.go` ~379–398) and writes the
comparison report — but the **rich sections** (per-decision token cost + growth slope, memory
fidelity) only exist in the Python skill's `analyze.py --tokens` path, which a user runs *by
hand* and which reaches into `.claude/`. Worse, re-running `experiment go` *overwrites* the
report and drops the Python-appended block, so the rich sections silently vanish until someone
re-runs `--tokens`.

**Decision (resolved 2026-06-10).** Fold token + fidelity into the native-Go report so
`experiment go` is the single, complete, one-command path. Go **always writes the per-call CSV**
(zero-dep data artifact). **PNG rendering is out of the flagship path** — the cost chart's real
home is the Track B replay UI (`chart.js` over the same CSV), so the report just carries the CSV
plus a one-line pointer; no `uv`/`chart.py` shell-out, no Go-native charting dep. (`chart.py`
survives only as an optional local dev convenience.) See the resolved Q1/Q2 notes below for the
append mechanism and the retirement of `--tokens`.

**Scope.**
- Token-cost + growth-slope section → Go `eval` package; always write the per-call CSV.
- Fidelity section → Go `eval` package (Q4 resolved: ports cleanly; port `fidelity.py`'s
  deterministic logic; regression-test against a hermetic in-test fixture — never real
  `research/` sessions, which are gitignored).
- Go always writes the per-call CSV; no chart shell-out in the flagship path (PNG → Track B).
- README pass: drop the `python3 .claude/skills/...` side-step.

**Open questions:** none blocking. (Q1/Q2/Q4 resolved below; Q5 — chart schema coupling —
dropped, since the chart moves to the Track B replay UI.)

**Q1 resolved (2026-06-10) — single inline Go pass, no sentinel; decouple the gate.** Go is
now the only writer, so the `<!-- BEGIN/END eval-analysis -->` sentinel (which existed solely
to let two processes share the file idempotently) is dropped: emit base table + rich sections
in one render pass, fully regenerated each run. The manual-fidelity sidecar
(`<id>-fidelity-manual.md`, read at `analyze.py:801`) stays a separate file read+inlined at
render — it needs no sentinel. **Gate change:** `analyze.py:680` gates the entire rich block
on `llm-akg-durable`-vs-naive; the port decouples it — token-cost+slope emitted for *every*
experiment (it's the centerpiece of the phase-2 wiki/mdsingle brackets, none of which are
`llm-akg-durable`), fidelity emitted whenever ≥1 agent has machine-extractable claims
(akg/wiki). The AKG-demo *narrative prose* (`analyze.py` ~726–807) is the only thing that
stays AKG-gated — emit the data sections generically, keep the editorial paragraphs behind the
matchup check.

**Q2 resolved (2026-06-10) — retire `--tokens`.** Token+fidelity become native-Go-only; the
`analyze.py --tokens` path is deleted (the three functions — `collect_token_rows`,
`token_summary`, `_experiment_fidelity` — are self-contained, so this does not touch the
`--hand`/`--traces` paths). One true source of truth (Go). The only lost capability is
standalone regen of the rich sections, which costs nothing: re-running `poker experiment go`
on a completed experiment is a cheap local re-render (`collectMissing` skips sessions that
already have `eval.json`; `Compare`+render are pure file reads — no session replay, no API
spend). Implied cleanups: drop the `--tokens` section from `SKILL.md` (incl. moving the
chart-embed note into the Go path), and locate the `uv`/`chart.py` shell-out in
`experiment go` rather than the skill.

**Q4 resolved (2026-06-10) — fidelity ports cleanly; the risk is parity, not feasibility.**
`fidelity.py` is fully deterministic (no oracle, no LLM judge — its docstring and code both
confirm). Ground truth is just `hands.jsonl` (Go reads it); claims come from
`memory-export.json` (akg) and `agents/<name>/wiki/hands/*.md` (wiki), both Go-readable; the
rest is `verify`/`agg`/`drift_buckets` arithmetic. The "two source files" confusion resolves:
**`fidelity.py` is the engine to port**, `_experiment_fidelity` in `analyze.py` is just the
session/seat orchestration loop. Agent→extractor keying (`llm-akg-durable`→akg,
`llm-md-wiki`→wiki) maps exactly onto Q1's "fidelity where machine-extractable" gate. The only
real risk is **regex-heuristic parity** (Python `re` → Go RE2): preserve `re.MULTILINE`→`(?m)`,
the line-anchored `Board:` rule that avoids mis-attributing inline prose, the `[Vv]illain`
casing, and the hand-id `(\d+)` parse. **Acceptance test: a hermetic Go regression test** on a
small synthetic fixture (a hand-authored `hands.jsonl` + `memory-export.json` +
`wiki/hands/*.md`, constructed in-test the way the existing `eval` tests build artifacts in
`t.TempDir()`), exercising every classifier branch — OK / FABRICATION / BOARD ERROR /
no-such-hand / missing-claim / the inline-prose `Board:` trap. It must **not** read anything
under `research/`: those session artifacts are **gitignored, not committed**, so a test
depending on them would fail on a fresh clone and in CI. A parity check against the Python tool
on a real phase-2 session is fine as a **one-time, local, manual** migration-confidence step —
a dev action during the port, never a committed test.

**Keep extraction symmetric across substrates (validity guardrail).** Do *not* "fix" the
fragile prose regex by giving AKG structured `villain_cards`/`board` meta fields while wiki/prose
agents stay prose — for the prose agents the prose *is* the substrate under test, and privileging
AKG with trivially-parseable claims would hand it an unfair extraction advantage that biases the
headline fidelity comparison. Extraction must stay identical across substrates (regex-over-prose
for all); harden it with one robust, well-tested extractor, not per-agent structure.

**A1 parallelizes** into independent lanes: token port / fidelity port / README.

### A2. Harness-neutral analysis CLI (Phase 1, parallel to A1)

The introspection the skill does today — `--hand N` (drill one hand's context) and `--traces
<keyword>` (grep reasoning) — are *queries over session JSONL*, not metric computation. Move
them to first-class `poker analyze` subcommands:

- `poker analyze hand <N>` and `poker analyze traces <keyword>` — Go, stream the (multi-MB)
  session JSONL line-by-line, return only the relevant slice so an agent never slurps the whole
  file. This is the "tuned for efficient extraction" interface.
- `.claude/skills/eval-analysis/SKILL.md` shrinks to a thin wrapper documenting those commands
  — usable verbatim by doug, Codex, a bare shell, any harness. The capability is the CLI; the
  skill is just discoverability.
- `chart.py` is no longer part of any shipped path — it survives only as an optional local dev
  convenience (CSV → PNG). The real cost chart moves to the Track B replay UI (`chart.js` over
  the Go CSV), so there is no Python in any flagship flow.

### A3. Artifact consolidation (Phase 1, parallel)

The session bundle felt overwhelming because **filenames don't encode the three axes the
artifacts live on** — *owner* (server / agent / derived), *scope* (per-hand / session /
experiment), and, for agent traces, *phase* (decide / learn). Fix it as a consistent scheme,
not file-by-file:

```
sessions/<id>/
  manifest.json             server,  session    match metadata
  hands.jsonl               server,  per-hand   ground truth
  session-report.md         derived, session    standalone/demo only (see below)
  eval.json                 derived, session    machine summary + cheap-regen cache
  agents/<name>/
    memory.akg              agent               the durable store
    session-decisions.jsonl  agent, decide      decision-phase Pi trace  (was pi-session.jsonl)
    session-updates.jsonl    agent, learn       memory-update Pi trace   (was update-session.jsonl)
    memory-export.json      derived, session    human-readable graph snapshot (kept)
    stderr.log / stdout.log runtime             logs
```

- **Renames** (`pi-session.jsonl` → `session-decisions.jsonl`, `update-session.jsonl` →
  `session-updates.jsonl`): these are *our* files (TS `shared`/`update.ts` write them, Go
  `eval/load.go` reads them), but the names are also recorded inside every `eval.json`
  `source_artifacts` block and used by every archived session. Ship as **dual-read,
  single-write**: new sessions write the new names; the Go reader accepts either; archived
  sessions (incl. the committed phase-2 brackets) still reproduce. No backward break.
- **Per-session `report.md` → `session-report.md`, emitted only for standalone sessions.** In
  experiment mode the aggregate `reports/<id>.md` already carries the per-session diagnostic
  table, so the per-session files are redundant noise — suppress them. This also kills the
  `report.md` (session) vs `reports/<id>.md` (experiment) name collision.
- **Drop `diagnostics.jsonl`** — the best-effort `graph_rot` log written by `update.ts`, read
  by nothing in the toolchain.
- **Gitignore `*-export-*.jsonl`** — `update-session-export-NNNN.jsonl` is transient scratch
  (`exportUpdateLog` writes it → appends into `session-updates.jsonl` → `rm`s it); a survivor
  is only crash debris.
- **Keep `eval.json`** (a primary input to the generated report *and* the cheap-regen cache)
  and **`memory-export.json`** (zero-tooling human inspection of the headline memory graph on
  the now-public repo — cheap, worth it).
- **Make `hand_number` (and a `decision_index`) first-class trace fields**, not grep targets.
  The learn trace already stamps an explicit `{hand_number: N}` marker (`update.ts:107`); the
  decision trace only carries it *inside* the prompt payload, recovered today by
  substring-grepping `"hand_number": N` (`find_hand_context`). Formalize it in the
  `session-decisions.jsonl` schema so every grader — the deterministic Go eval *and* any future
  qualitative agent — ties a trace segment to a hand by reading a field, not parsing prose. This
  single field also unlocks clean per-hand token bucketing and the replay overlay's
  per-decision lookup.
- **Deterministic memory-use counts (DONE 2026-06-11).** New `eval.MemoryEngagement` metric:
  segments the decision trace on `user` messages (`toolResult` is its own role, so this is clean
  in both decision- and hand-scope traces) and computes **reads-per-decision**, **read coverage**
  (decisions with ≥1 read before acting — ordering is automatic since the action is the terminal
  assistant text), and `reads_by_tool`. Substrate-neutral: every decision-trace tool call is a
  read (write tools are update-session only), so no per-agent tool table. Lands in
  `seats[].memory_engagement` (eval.json, additive) and a "Memory Engagement" report section. The
  "retrieval agent made zero decision-time reads" tripwire is now a literal Go assertion
  (`EngagementAgentSummary.TripwireFail`, scoped per session so one clean session can't mask a
  broken one) that raises a `TRIPWIRE FAIL` warning; retrieval classification ported from
  analyze.py's `RETRIEVAL_STRATEGIES`. The Python tripwire stays until a later cleanup. (The fuzzy
  half — did the recall *inform* the action — stays with the parked qualitative layer.)
- **Gross pot size in `hands.jsonl`.** `eval.json` currently reports *swing*, not reconstructed
  pot size (`biggest_swing_hand` comment). Have the server stamp gross pot per hand so the
  deterministic eval reports true pots instead of a proxy. Small server-side add.
- **Promote `session-artifacts.md` to a complete artifact map (DONE 2026-06-11)** — added the
  full owner/scope/writer/consumer map, the `session-decisions.jsonl` schema (incl. the
  `hand_boundary` marker and `toolResult` role), and the `seats[].memory_engagement` contract.
  `decision_index` remains explicitly deferred there (per-decision attribution is already
  recoverable by segmenting on `user` messages; held until the Track B replay overlay needs a
  stable anchor).

### A4. Smaller ergonomic wins (opportunistic)

- No `CONTRIBUTING.md` / one-page quickstart beyond the README.
- Anything else surfaced while in A1–A3.

## Track B — Headline feature: poker-table HTML replay (Phase 2, ~days 3–5)

**The pitch.** This project is not about poker, it's about **memory**. The replay is a
*cinematic* one: a simple poker table is the stage, and when an agent stops to think — and
especially when it *recalls a stored fact and acts on it* — that reasoning surfaces inline, at
the moment of decision. The money shot: watch the AKG agent pull up *"villain check-folded to a
3bb bet on a similar turn"* and fire the bluff, while the stateless agent walks into the same
spot blind. Then a closing **summary screen** delivers the verdict (who won, why memory
mattered, the cost-vs-fidelity frontier).

This framing supersedes the earlier "side-by-side *what each agent knew* panel" idea: the
reasoning reveal is **inline on the table in time**, not a static two-column spreadsheet —
stronger as a GIF-able narrative. (Owner framing, 2026-06-11.)

**Decision (resolved 2026-06-10).** **Self-contained HTML page.** Go generates a standalone
`.html` from `hands.jsonl` + `session-decisions.jsonl` + `eval.json` (+ `memory-export.json`).
Opens in any browser, no server, hostable on GitHub Pages, linkable from the README, trivially
screenshot/GIF-able for a launch thread. Chosen over a terminal TUI for shareability.

**Three acts (one page):**
- *The table* (the stage): cards, board, one chip = bet / one chip = pot, hands dealt in
  sequence, transport controls (step / play / scrub across hands). Table stakes.
- *The thinking reveal* (the hook): two **orthogonal** per-decision signals, surfaced inline
  over the acting player. The trace supplies each independently, so the cross-product falls out
  without special-casing:
  - **Memory indicator** (binary, driven by `toolCall` presence) — *actively recalling* (a read
    is in flight; render the recalled fact that justified the action — the loud, money-shot
    case) vs. *deciding on available info* (no read this decision). The read count is exactly
    the Track A `seats[].memory_engagement` metric — the replay *renders* what the eval *counts*.
  - **Thought bubble** (present-or-absent, driven by `thinking` block presence) — surface the
    agent's reasoning when it deliberated; no bubble on a snap action.

  Honest by construction: "deciding on available info" claims nothing about *what* was available
  (injected opponent summary, just hole cards, or — for a memoryless agent — nothing), so it
  needs no caveat on the now-public repo. The stateless-vs-memory contrast emerges on its own —
  a memoryless agent simply never lights the *recalling* indicator while the AKG/wiki agents do;
  no decision needs to be labelled "blind." A bare snap action (no read, no thinking) is just
  the empty cell of the cross-product, not a special state.
- *The summary screen* (the verdict): who won, chips/hand, showdown rate, **memory engagement**,
  **fidelity**, and the **per-decision cost-vs-fidelity chart** — the centerpiece thesis result.
  This is the destination A1 cleared the runway for by dropping the standalone report PNG. All
  of it renders straight from `eval.json` (which inlines `token_calls` + `fidelity_rows` +
  `memory_export` shape + chips), so the chart needs no separate CSV in-page; `chart.js` over
  the inlined token rows.

**Architecture — the one league-aware seam (see [`project_poker_league_north_star`]).** Put a
clean Go **view-model boundary** between *gather the data* (read artifacts → a `ReplayModel`
Go type: hands, players, whose-turn, per-decision thinking/reads/action, summary metrics) and
*render the HTML* (template the view-model into the self-contained page). This is the **only**
concession to the long-term "Agent Poker League by AKG" vision (live hosted BYO-agent league,
fidelity + chip leaderboards) allowed into v0 scope: it costs ~nothing now and means going live
later is "feed the view-model from a websocket instead of a file." Everything else the league
needs — agent submission, untrusted-code sandboxing, auth, hosting, cross-match leaderboards —
stays **explicitly parked** (`AGENTS.md` scope discipline). Do not pre-build it.

**Generator home (proposed).** A new `poker replay <session-dir>` subcommand (`cmd/poker/`,
sibling to `analyze`/`experiment`), emitting the self-contained `.html`. Consistent with the
harness-neutral-CLI principle; not auto-welded into `experiment go`.

**Build order — vertical slice first.** Prove the whole loop on the smallest unit — one hand
dealt on the table, with one thinking reveal at one decision, both seats present — before adding
breadth (scrub across all hands, animation polish). The summary screen is comparatively cheap
(pure `eval.json` rendering) and slots in as act two, not act one.

**Kickoff eyeball — DONE (2026-06-11), data confirmed rich.** Traced a real AKG-vs-wiki session
(`phase2-wiki-vs-akg/.../akg-vs-wiki-1`). Findings that pin the design:
- Every layer the reveal needs is present and **substrate-symmetric**: game state + injected
  memory summary in the `user` prompt; reasoning in `assistant` `thinking` blocks (~326–330 per
  session); drill-in reads as `toolCall`→`toolResult` pairs (AKG `akg_get_node`/`akg_get_nodes`,
  wiki `md_read_page`); action in the terminal assistant `text`. The `toolResult` carries the
  full recalled fact (the stored hand body) — that's the money-shot content.
- **Legacy traces have no `hand_boundary` marker** — it's `Hand: N` prose. All archived
  phase-2 brackets (the replay's showcase material) predate Track A's marker, so the generator
  **must implement `Hand: N` prose fallback on day one** (the dual-read contract already allows
  this; just don't treat it as optional).
- `decision_index` stays **derived in Go at build time** (segment each hand-scope group on
  `user` messages → ordinal); no TS/wire-contract change. The `session-artifacts.md` deferral
  note explicitly permits this — promote to a first-class field only if the overlay later needs
  a stable cross-tool anchor, which the self-contained page does not.
- Many decisions are **bare** (`{"action":"call"}`, no thinking/reads) — expected; that absence
  *is* the stateless-vs-memory contrast, render it as such.

## Track C — Writeups (parked, angles locked)

Reports already exist to mine under `research/experiments/*/reports/`.

1. **Catching the confound** — the tautological-fidelity correction (code-computed vs
   model-maintained `llm-akg-durable`; see `research/llm-akg-durable-rework.md`). An
   intellectual-honesty piece.
2. **The fidelity-vs-cost frontier** — the centerpiece thesis result from the phase2 brackets
   (`phase2-wiki-vs-akg`, `phase2-mdsingle-vs-akg`, `phase2-fullhistory-vs-akg`): can linked
   prose match a typed graph, and at what per-decision cost?

## Sequencing

Track A (A1–A3) first, then replay (Track B) — confirmed. Track A is lower-risk, clears the
`.claude/` wart, and lands the artifact renames/consolidation *before* the replay is built on
top of those artifacts (so the replay reads `session-decisions.jsonl`, not a name we're about
to change). Writeups run in the background whenever there's a spare slot.

## Open questions

- **Qualitative analysis layer (new scope — parked for a future iteration).** The deterministic
  grader answers ground-truth questions (fidelity: are the stored cards *real*?). The
  *non*-ground-truth questions — did the agent's reasoning actually *use* what it recalled, was a
  bluff *justified* by the recalled stat, was an inference *sound* — want an LLM analysis agent
  reading `session-decisions` + `session-updates` + `hands.jsonl`. Keep it **separate** from
  fidelity, which stays deterministic for reproducibility and to avoid the tautological-grader
  confound (Track C #1). Not in this iteration; recorded so it isn't silently folded into A1.
  - **Candidate hypothesis (surfaced 2026-06-11 via the Track B replay): does reasoning
    *soundness* decay as context grows?** Spotted in `akg-vs-wiki-1` replay — the agent reasoned
    about a "flush draw on the turn" holding only 3 of a suit (impossible: one card to come). That
    is a *soundness* failure (an invalid live-hand inference), categorically distinct from
    *fidelity* (a misremembered stored fact) — no fidelity check would ever flag it. The question:
    does the rate of such errors *rise with the agent's context size*? Methodology notes:
    - **X-axis is already free** — per-decision context size is logged (`token_calls[].prompt_tokens`,
      grows ~×1.6–2.1 across a single 150-hand session even for the *bounded*-memory agents).
      Regress on measured prompt tokens, not hand index, to control for situation complexity.
    - **Y-axis is the blocker** — any single error class is far too sparse to trend (only ~2
      "flush draw" mentions in a whole 150-hand session). Needs either a deterministic multi-class
      live-claim detector (flush/straight-draw validity vs `hands.jsonl`, "top pair" vs board,
      pot-odds arithmetic — cheap, reproducible, but aggregate across brackets for N) or the
      LLM-judge soundness layer above (broad coverage, API cost, keep separate from fidelity).
    - **The headline comparison** — bounded-memory (`akg`, context plateaus) vs unbounded
      (`fullhistory`, context balloons): thesis prediction is `fullhistory` soundness degrades while
      `akg` stays flat. Falsifiable both ways.
    - **Requires a long horizon** — the divergence only opens once `fullhistory` climbs well past
      `akg` and crosses its context-ceiling/compaction threshold (the intended degradation signal).
      Wants a *few* long (500+ hand) `fullhistory-vs-akg` sessions, not one ultra-long n=1 run.
- A4 contents — what else earns a slot once A1–A3 are underway.
- (Resolved 2026-06-11) Replay design — the kickoff eyeball is done; design is the cinematic
  table + inline two-tier thinking reveal + summary screen, behind a Go view-model seam (see
  Track B above). Remaining detail-level open questions surface during the vertical slice.
- A1-Q4 (fidelity ports cleanly) and A1-Q5 (Go-CSV ⇄ `chart.py` schema) — tracked under A1.
- (Resolved/dropped) Where the relocated analysis scripts land — dissolved by Track A's
  "computation is Go, the skill stops computing" principle; nothing computational relocates.
