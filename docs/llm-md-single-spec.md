# `llm-md-single` Contract

`llm-md-single` keeps one markdown file of notes about the opponent. Before each
decision it pastes the whole file into the prompt; after each hand it asks the model to
rewrite the file with what it just learned. It is the lower of the two prose-memory
rungs and the bracket below `llm-md-wiki`.

## Strategy role

The memory-strategy lineup is documented in [`research.md`](research.md). In that
ladder this rung sits between `llm-fullhistory` and `llm-md-wiki`:

- `llm-fullhistory` dumps the raw transcript — nothing is summarized, tokens grow O(N).
- `llm-md-single` keeps **one running summary the model rewrites itself**.
- `llm-md-wiki` splits that summary across linked, addressable pages.
- `llm-akg-durable` is the typed, queryable control.

The failure mode this rung exists to measure is **how accurate the notes stay, and what
the whole-file rewrite costs**: the file has no addressable parts, so every update means
re-reading and re-writing the entire thing. Facts get silently dropped, contradicted, or
bloated, and the rewrite gets more expensive and more lossy as the file grows. Evaluate
it through checked-in experiment definitions and `poker experiment go <experiment-id>`,
not through one-off run notes.

## The one design choice that matters: the model updates the file, not code

This is the defining choice for `llm-md-single`: **the model updates the file, not code.**
The durable agent was *once* code-maintained (`rebuildOpponent` / `rebuildPatterns` recomputed
exact counts from stored hands, which is why its stated profile matched ground truth to the
digit), but it has since been reworked to be model-maintained too — see
[`llm-akg-durable-spec.md`](llm-akg-durable-spec.md). So across all contestants the model now
keeps its own memory; the axis under test is *representation*, not extraction method.
`llm-md-single` keeps that memory as one freeform file:

> After each hand, the model is given the current notes plus a summary of the hand that
> just ended, and asked to return the full updated notes.

If code did the updating, the notes would never drift and the experiment would prove
nothing. The point of the rung is to see what happens when prose is kept current by the
model instead of computed — so "the model rewrites the file" is fixed, not an
implementation shortcut.

## Runtime package

The agent lives under `pi-agents/llm-md-single/`.

Important files:

- `src/main.ts` — process entry point and runner wiring
- `src/memory.ts` — `MarkdownSingleMemoryPolicy`: read the file before a decision, ask
  the model to rewrite it after a hand
- `src/update.ts` — builds the update session and the prompt that asks the model to
  rewrite the notes
- `test/` — unit coverage (file read/inject, update-prompt assembly, fake-decision smoke)

The stable executable name is `poker-agent-llm-md-single`. The Go runner resolves the
strategy alias `llm-md-single` for experiment sessions. Register the key in
`pi-agents/registry.json` and add the package to the `workspaces` array.

## Stored memory

A single file under the server-provided `memory_dir`:

- `memory_dir/notes.md` — one freeform markdown file the model fully owns and rewrites.

No structure is imposed. The model is told what's useful to keep (opponent tendencies,
stat tallies, exploit notes, recent hands) but chooses and maintains the layout itself —
including letting it decay. This file is the **only** thing the agent remembers between
hands; nothing about the opponent lives anywhere else.

Before the first hand completes the file doesn't exist yet; in that case the agent
injects an explicit "no notes yet" section, the same way `llm-fullhistory` handles an
empty history.

## How the file gets written

The only write happens after a completed hand, in `MemoryPolicy.afterHandEnd`:

1. Read the current `notes.md` (empty string if absent).
2. Build a short, plain summary of the completed hand from `CompletedHandContext` — hole
   cards, board, per-street actions, showdown, hero net. Reuse the formatting already in
   `llm-fullhistory/src/history.ts`.
3. Ask the model to rewrite the notes (current notes + hand summary in, full updated file
   out), using the same model and thinking level as decisions.
4. Overwrite `notes.md` with what the model returns.

Rules for the update:

- **It runs in a fresh session that sees only the file and the new hand.** Decisions also
  start a fresh session each hand, so nothing carries across hands except `notes.md` —
  the file is the only memory by construction, not something the agent has to police.
- **Same model and thinking level as decisions** (`PI_POKER_MODEL` /
  `PI_POKER_THINKING_LEVEL`) — the lineup holds the model constant.
- **Full rewrite, not a diff.** The model returns the entire file. That whole-file cost is
  the failure mode under test; don't optimize it into an append or a patch.
- If the update fails or comes back empty, keep the previous `notes.md` and log to
  `stderr.log`. A failed update must never corrupt or truncate the file.

## Decision-time reads

No tools. `beforeDecision` reads `notes.md` and injects the **whole file** as one prompt
section ahead of the current game state. The model just reads it, the way
`llm-fullhistory` reads its transcript.

That's the contrast with `llm-akg-durable` (which queries a precomputed profile through
tools) and with `llm-md-wiki` (which pulls up pages on demand). Because the whole file
goes into every decision prompt, this agent's read cost grows with the file on every
single decision, not only when it's rewritten.

## Prompts

The decision prompt tells the model:

- it is a heads-up no-limit Texas Hold'em decision engine
- the injected notes are its own running memory of this opponent
- the final answer must be exactly one legal action from the server-provided
  `legal_actions`, as JSON: `{"action": string, "amount"?: number}`
- no commentary, markdown, or extra keys in the final JSON

The shared parser (`shared/src/action.ts`) already recovers the action even if the model
writes some reasoning before the JSON, so a leak costs nothing — keep the prompt strict
anyway.

The update prompt (in `src/update.ts`) asks the model to: keep the durable facts,
reconcile contradictions in favor of the newest evidence, keep the stat tallies, and
return the complete file.

## Artifacts

A `llm-md-single` session can produce:

- `agents/<name>/notes.md` — the memory file (the source of truth for what it remembers)
- `agents/<name>/update-session.jsonl` — transcript of the after-hand updates, kept
  **separate** from the decision transcript so update cost is measurable on its own
- `agents/<name>/pi-session.jsonl` — decision transcript / observability log
- `agents/<name>/stderr.log` — retry, fallback, and update-failure diagnostics

The two transcripts together give the full per-hand token cost (decision read + rewrite).

## What this rung measures

Both curves of the fidelity-vs-cost frontier (see [`research.md`](research.md)) apply,
and both are expected to bend the wrong way as hands pile up:

- **Fidelity vs. hand count.** Cross-check the stats the notes *claim* against engine
  ground truth in `hands.jsonl`. Unlike `llm-akg-durable` (exact), this agent is expected
  to drift — dropped facts, stale contradictions, rounded or invented numbers — and the
  run's job is to find where that drift starts.
- **Tokens per decision vs. hand count.** Two cost pieces, both growing with file size:
  the decision read (whole file every turn) and the rewrite (whole file in and out every
  hand). Sum both transcripts.

Per the roadmap, this rung may run to a **shorter** horizon than the durable agent if the
notes fall apart or the rewrite gets too expensive — "broke at hand ~N" is a result.

## Two cost axes

The frontier plots **runtime** cost. `llm-md-single` is also cheap to *build*: read a
file, summarize a hand, ask the model to rewrite — no SDK, no schema, no tools. That low
build cost is markdown's whole appeal, so the comparison should report **both** axes, not
just runtime, to show the trade AKG makes (more to build once, less to run forever,
higher fidelity).

## Constraints to preserve

- The model, not code, rewrites the notes.
- The rewrite is whole-file; the update runs in a fresh session seeing only the file and
  the new hand.
- The decision prompt injects the entire `notes.md`; decisions use no tools.
- The model is held constant across decisions and updates.
- `memory_dir` comes from the server and scopes each session's file and logs.
- A failed update keeps the previous file rather than corrupting it.
- The shared Pi runner still owns protocol handling, action validation, retries, and safe
  fallback.
- Evaluation uses the experiment-first workflow in [`eval-system.md`](eval-system.md).

## Related references

- [`llm-md-wiki-spec.md`](llm-md-wiki-spec.md) — the linked-pages rung above this one
- [`llm-akg-durable-spec.md`](llm-akg-durable-spec.md) — the typed/computed control to
  contrast against
- [`kb/adding-a-memory-strategy.md`](kb/adding-a-memory-strategy.md) — package how-to
- [`kb/llm-fullhistory-baseline.md`](kb/llm-fullhistory-baseline.md) — the hand-summary
  formatting to reuse
