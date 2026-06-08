# `llm-md-wiki` Contract

`llm-md-wiki` keeps its notes as a small set of linked markdown pages — pages connected
by `[[wiki links]]`, a knowledge graph faked in prose. After each hand the model updates
the pages; at decision time the agent pulls up the pages it wants through read-only
tools, following links the way `llm-akg-durable` walks its graph. This is the top prose
rung and the **true head-to-head rival to AKG**: same ambition (addressable, linkable,
multi-page), but plain text instead of a typed, queryable store.

## Strategy role

In the ladder documented in [`research.md`](research.md):

- `llm-md-single` keeps one whole-file blob the model rewrites each hand.
- `llm-md-wiki` splits the notes into **linked pages the agent pulls up on demand** —
  better structure, still prose.
- `llm-akg-durable` is the typed, queryable graph — the same model-maintained counts, but stored as queryable fields instead of re-tallied prose.

The headline experiment of the whole project is **`llm-md-wiki` vs `llm-akg-durable`**:
if a linked pile of markdown can hold an accurate, cheap-to-look-up opponent model as
well as a typed graph, the typed graph is overhead. The thesis is that it can't.

The failure mode this rung exists to measure is **looking things up + keeping the links
honest**:

- **Looking things up.** Prose can't *compute* "folds to c-bet 39/77." To answer a
  numeric question the agent has to re-read pages and re-count in the prompt — slow,
  token-heavy, and error-prone — where AKG returns the number from one tool call.
- **Link integrity.** As the pages grow, `[[links]]` rot: links to pages that were never
  created, duplicate pages for the same idea, orphan pages nothing links to. Plain text
  loses the consistency a typed store keeps for free.

Evaluate it through checked-in experiment definitions and
`poker experiment go <experiment-id>`, not through one-off run notes.

## Two design choices that matter

1. **The model updates the pages, not code.** Same as `llm-md-single` — if code kept the
   pages current they'd never drift and the links would never rot, which would defeat the
   measurement. "The model updates the pages" is fixed.
2. **The agent pulls up pages on demand; it isn't handed the whole wiki.** This mirrors
   `llm-akg-durable` starting from the `opponent/villain` root node (via `akg_get_node`). The
   decision prompt includes only the root page as an index; the agent fetches deeper pages
   through tools. That keeps the
   matchup a fair test of *structure*, not of "who got more text pasted into the prompt,"
   and it's the step up from `llm-md-single`, which always preloads its whole file.

## Runtime package

The agent lives under `pi-agents/llm-md-wiki/`.

Important files:

- `src/main.ts` — process entry point and runner wiring
- `src/memory.ts` — `MarkdownWikiMemoryPolicy`: inject the root page before a decision,
  ask the model to update the pages after a hand
- `src/tools.ts` — read-only page-reading Pi tools
- `src/update.ts` — builds the update session and the prompt that asks the model to
  update the pages
- `src/runtime.ts` — decision engine and `sessionFactory` (the custom-tools path,
  mirroring `llm-akg-durable`)
- `test/` — unit coverage (page read/list tools, root injection, update-prompt assembly,
  link-extraction helper, fake-decision smoke)

The stable executable name is `poker-agent-llm-md-wiki`. The Go runner resolves the
strategy alias `llm-md-wiki`. Register the key in `pi-agents/registry.json` and add the
package to the `workspaces` array. Because it adds custom tools, it builds its own
decision engine with a `sessionFactory` (the way `llm-akg-durable` does in
`src/runtime.ts`) rather than using `createStandardDecisionEngine`.

## Stored memory

A directory of markdown pages under the server-provided `memory_dir`. All pages use YAML
frontmatter and are connected by `[[wiki links]]`.

### Page roles

**`memory_dir/wiki/villain.md`** — **pure index**. Allowed content only:

- YAML frontmatter: `slug`, `tags`, `hands_observed`, `last_updated`
- `## Stats` section: up to ~10 one-line stat bullets, each linking to its pattern page
  (e.g. `- Folds to c-bet: 5/10 → [[patterns/folds-to-cbet]]`)
- `## Pages` section: one line per page — `[[slug]] — one-sentence description`
- `## Notable Hands` section (optional): `[[hands/hand-N]]` links with one-line notes, **only for hands that changed the model** — a first observation of a new tendency, a behaviour that contradicts prior reads, or an unusual showdown. Routine evidence for already-established patterns goes only in the pattern page, not here.

**Forbidden in villain.md**: per-hand narratives, positional notes, action logs, hand
histories. If villain.md body exceeds ~25 lines, detail is accumulating in the wrong
place — it belongs in pattern or hand pages. The villain.md root is the AKG
`opponent/villain` equivalent and must stay comparably compact.

**`memory_dir/wiki/patterns/<slug>.md`** — one page per opponent tendency, equivalent to
AKG pattern nodes. Frontmatter: `slug`, `tags`, `sample_size`, `last_updated`. Body:
bold count line (`**Count: N/M**`), concise evidence notes, links to supporting hand
pages, cross-links to related patterns.

**`memory_dir/wiki/hands/hand-<n>.md`** — optional per-notable-hand page. Frontmatter:
`slug`, `tags: [hand]`, `hand: N`. Body: full action summary and what it revealed, links
back to the pattern pages it supports.

Page names and links are the **model's** to create and keep tidy — including letting them
rot. Only the root page name (`villain.md`) is fixed, as the guaranteed entry point;
everything reachable from it is prose the model wrote and is responsible for. The wiki
directory is the only thing the agent remembers between hands.

Before the first hand completes, only `villain.md` exists (seeded empty / "no reads yet"),
and the page-list tool reports it alone.

## How the pages get written

The only write happens after a completed hand, in `MemoryPolicy.afterHandEnd`:

1. Build a short, plain summary of the completed hand from `CompletedHandContext` (reuse
   `llm-fullhistory`'s hand formatting).
2. Start a fresh session and give it the current root page and the hand summary, plus the same
   page-reading tools the decision agent uses (`md_list_pages` / `md_read_page`), so it can
   discover and pull up the specific pages it needs to change. The update prompt injects only the
   root page and the hand summary — not an eager page list; the model discovers pages on demand,
   symmetric with the active-retrieval decision path and with `llm-akg-durable-spec.md`.
3. The model returns a set of page writes (create/replace by page name): update the root
   summary, update or create the relevant pattern pages, optionally add a hand page, and
   keep the `[[links]]` between them.
4. Apply the writes to the `wiki/` directory.

Rules for the update:

- **It runs in a fresh session that sees only the wiki and the new hand.** Decisions also
  start a fresh session each hand, so nothing carries across hands except the wiki — it's
  the only memory by construction, not something the agent has to police.
- **Same model and thinking level as decisions** (`PI_POKER_MODEL` /
  `PI_POKER_THINKING_LEVEL`) — the lineup holds the model constant.
- **The model keeps the links honest; the code does not auto-repair them.** Broken,
  duplicate, and orphan links are the failure mode under test — the runtime counts them
  (below) but must not silently fix them.
- **Counts are tallied in prose, never computed in code.** The code does not compute stats
  and hand them over; if a page is to say "folds to c-bet 39/77," the model has to get
  there by reading and counting. That's the look-things-up failure mode by construction.
- If the update fails or comes back empty, keep the previous wiki and log to `stderr.log`.
  A failed update must never delete or truncate existing pages.

## Decision-time reading tools

The decision session registers these read-only tools (built-in Pi tools disabled, as in
`llm-akg-durable`):

- `md_list_pages` — list the page names under `wiki/`.
- `md_read_page` — return the raw markdown of a named page (with a clear "not found"
  result for a missing/broken target).

The agent follows a `[[link]]` by reading its target with `md_read_page`. Tool results
are JSON-serializable and deterministic for empty/unknown cases. The model may call tools
before returning its action; the final response is still JSON only.

`beforeDecision` injects only a small section: the `villain.md` root page as the index,
plus a note that `md_read_page` / `md_list_pages` are available to follow links. The
prompt does not preload the whole wiki — the same on-demand pattern as
`llm-akg-durable`, which doesn't preload a fixed history block either.

## Prompts

The decision prompt tells the model:

- it is a heads-up no-limit Texas Hold'em decision engine
- `villain.md` is an index only — real pattern data lives in linked pages
- it **must** read relevant pattern pages via `md_read_page` before deciding; the
  villain.md one-liners are not sufficient on their own
- the final answer must be exactly one legal action, as JSON: `{"action": string, "amount"?: number}`
- no commentary, markdown, or extra keys in the final JSON

The shared parser (`shared/src/action.ts`) already recovers the action even if the model
writes some reasoning before the JSON, so a leak costs nothing — keep the prompt strict
anyway.

The update prompt (in `src/update.ts`) enforces strict page roles: villain.md is a pure
index (stats + page directory, no narratives); pattern pages hold counts and evidence;
hand pages hold per-hand detail. The model follows this workflow each hand: read pages to
update, update pattern counts, create hand pages as needed, refresh villain.md index
entries only, write all changed pages.

## Artifacts

A `llm-md-wiki` session can produce:

- `agents/<name>/wiki/` — the linked-page memory (the source of truth for what it
  remembers)
- `agents/<name>/update-session.jsonl` — transcript of the after-hand updates, kept
  separate from decisions so update cost is measurable on its own
- `agents/<name>/pi-session.jsonl` — decision transcript / observability log
- `agents/<name>/stderr.log` — retry, fallback, update-failure, and link diagnostics

The two transcripts together give per-hand token cost (decision reads + updates).

## What this rung measures

The headline comparison reports the fidelity-vs-cost frontier (see [`research.md`](research.md)),
plus two checks specific to its failure mode:

- **Fidelity vs. hand count.** Cross-check the stats the wiki *states* against engine
  ground truth in `hands.jsonl`, just like `llm-akg-durable` — but here the numbers were
  counted by hand in prose, so watch where they diverge.
- **Tokens per decision vs. hand count.** On-demand reading means decision cost depends on
  how many pages the agent pulls up and how big they are. Compare the slope against AKG's
  near-flat profile reads. Update cost is the update transcript.
- **Look-up accuracy.** Test numeric claims directly ("folds to c-bet N/M"): does the wiki
  hold a correct number, or stale/approximate prose?
- **Link integrity over hand count.** Count broken, duplicate, and orphan links as the
  wiki grows — a decay curve with no equivalent in the typed store.

Per the roadmap, the wiki and the durable agent may run to **different** horizons; the
hand count at which the notes or the links break is itself the result.

## Two cost axes

On the **runtime** axis the wiki is expected to climb (reads + prose re-counting +
updates). On the one-time **build** axis it sits in the middle — more than
`llm-md-single` because of the page/link logic and the read tools, less than
`llm-akg-durable`'s SDK + schema + computed writes. Report **both** axes: AKG's bet is
that paying the higher build cost once buys flat runtime cost and exact fidelity forever,
which is exactly the trade this matchup is meant to expose.

## Constraints to preserve

- The model, not code, updates the pages and links.
- The update runs in a fresh session seeing only the wiki and the new hand.
- **villain.md is a pure index** — per-hand narratives, positional notes, and action logs
  are forbidden there; they belong in `hands/` pages linked from villain.md.
- All pages use YAML frontmatter (`slug`, `tags`, and page-type-specific fields).
- Reading is on-demand and tool-driven; only the root page is injected, deeper pages are
  fetched through read-only tools. The decision prompt must instruct the model to follow
  links — not treat the villain.md summary as sufficient.
- Counts are tallied in prose in pattern pages, never computed in code and injected.
- The runtime counts but does not auto-repair broken/duplicate/orphan links.
- The model is held constant across decisions and updates.
- `memory_dir` comes from the server and scopes the wiki and logs.
- A failed update keeps the existing wiki rather than corrupting it.
- The shared Pi runner still owns protocol handling, action validation, retries, and safe
  fallback.
- Evaluation uses the experiment-first workflow in [`eval-system.md`](eval-system.md).

## Related references

- [`llm-md-single-spec.md`](llm-md-single-spec.md) — the single-file rung below this one
- [`llm-akg-durable-spec.md`](llm-akg-durable-spec.md) — the typed/queryable control this
  rung is the headline rival to; mirror its tool-injection runtime pattern
- [`kb/adding-a-memory-strategy.md`](kb/adding-a-memory-strategy.md) — package how-to,
  including the custom-tools (`sessionFactory`) path
- [`kb/llm-akg-durable-active-retrieval.md`](kb/llm-akg-durable-active-retrieval.md) —
  on-demand-reading implementation reference
