# `llm-akg-durable` Contract

`llm-akg-durable` is the durable structured-memory AKG strategy. It is the current thesis agent for testing whether graph-backed opponent memory can improve or preserve poker performance while keeping prompt growth bounded and inspectable.

## Strategy role

The memory-strategy lineup is documented in [`research.md`](research.md). In that lineup:

- `llm-stateless` provides the no-memory LLM baseline.
- `llm-fullhistory` provides the naive high-token prompt-history baseline.
- `llm-akg-recent` provides a shallow bounded-memory AKG control.
- `llm-akg-durable` provides cumulative structured opponent memory and active retrieval.

`llm-akg-durable` should be evaluated through checked-in experiment definitions and `poker experiment go <experiment-id>`, not through one-off procedural run notes.

## Runtime package

The agent lives under `pi-agents/llm-akg-durable/`.

Important files:

- `src/main.ts` — process entry point and protocol runner wiring
- `src/memory.ts` — durable memory policy and post-hand writes
- `src/tools.ts` — AKG read-tool definitions
- `src/runtime.ts` — Pi session creation and prompt contract
- `src/update.ts` — post-hand update session, write-tool wiring, and the graph-rot diagnostic
- `test/` — deterministic unit and subprocess coverage

The stable executable name is `poker-agent-llm-akg-durable`. The Go runner resolves the strategy alias `llm-akg-durable` for experiment sessions.

## Write model

The only strategic graph write point is after a completed hand, and the **model itself**
performs the writes via tools — there is no code-computed extraction. This is the
agent-maintained rework recorded in
[`research/llm-akg-durable-rework.md`](research/llm-akg-durable-rework.md): the previous
deterministic write path (a fixed feature extractor, hardcoded pattern slugs, and evidence
thresholds) is removed. The agent is now exposed to the same maintenance drift as the prose
agents (`llm-md-single`, `llm-md-wiki`), with the typed-graph representation as the variable
under test.

Behavior:

- After a completed hand, `afterHandEnd` runs a fresh one-shot Pi **update session** using the
  same model and thinking level as decisions (`PI_POKER_MODEL` / `PI_POKER_THINKING_LEVEL`).
- The update session is given the current root node body and a summary of the hand that just
  finished. The rest of the graph is **not inlined**; the model discovers it through the same
  read tools the decision session uses (`akg_list_nodes` / `akg_get_node`). This keeps the
  update prompt bounded instead of growing O(N) with the hand count, symmetric with the
  active-retrieval decision path.
- The update prompt steers the model to **only record significant hands** (a showdown, a
  multi-street line, a surprising decision) and to leave trivial hands — preflop folds, blind
  steals — unwritten, and to **prefer durable tendency nodes over one node per hand**. This is
  model judgement expressed through the prompt, not a code-side skip, so it stays symmetric with
  the prose agents (`llm-md-single`, `llm-md-wiki`), which also run a model update every hand.
- The model writes its changes with AKG write tools, batched into a single `akg_apply` call.
- The store is committed after the update session finishes. A failed update keeps the prior
  graph (it never corrupts or truncates it) and logs to `stderr.log`.
- The update transcript is written to a separate `update-session.jsonl` so update cost is
  measurable apart from decision cost.

**Open vocabulary.** The model authors its own graph: it invents and names its own node types,
ids, titles, bodies, meta keys, tags, and edge relations — the structured analogue of the wiki
agent freely creating pages and `[[links]]`. There is no fixed slot set. The one constraint is
that writes must be well-formed AKG (valid type/relation/tag names, existing edge endpoints);
the write tools return a structured error on malformed input rather than throwing.

**Counts are the model's job.** No tool computes tallies. The model keeps its own counts in
node bodies or meta (for example "folds to flop c-bet 4/7") and may under- or over-count,
contradict itself, or let evidence go stale. That drift is a measurement, not a bug to fix.

The agent does **not** write to AKG during a decision. Decision-time AKG access is read-only.

## Stored graph shape

The graph shape is **authored by the model**, not fixed by code. The node types, ids,
pattern names, and edge relations below are conventions seeded and encouraged by the prompt,
not a schema the runtime enforces or fills in.

### Root index node

Identity: `opponent/villain`

The runtime seeds this node (with a `(no reads yet)` body) so the model always has a stable
index to start from, mirroring the wiki agent's seeded `villain.md`. `beforeDecision` injects
this node's body — and only this node's body — as the decision-time index.

This node is a **pure index**, exactly as `villain.md` is in `llm-md-wiki-spec.md`; the two
agents follow the same discipline and differ only in substrate. Allowed content only:

- a short **stats** summary (body lines or `meta`): up to ~10 one-line tendencies, each naming
  the tendency node it points to (the AKG analogue of the wiki's `## Stats` bullets with
  `[[links]]`);
- a **directory** of the nodes that exist, one line each with a one-sentence description;
- optional **notable-hand** pointers (edges to hand nodes) only for hands that changed the
  model — a first observation, a contradiction, or an unusual showdown.

**Forbidden in the index node**: per-hand narratives, action logs, hand histories, routine
evidence entries. If the index body exceeds ~25 lines the model is accumulating narrative and
must move detail into tendency nodes. This bound is what keeps the injected decision-time
payload flat instead of growing O(hands); detail lives in the tendency nodes and is reached by
drilling in (see *Decision-time retrieval tools*).

### Model-authored nodes and edges

Beyond the root, the model creates whatever nodes and edges it finds useful — for example a
node per observed tendency, an optional node for a notable hand, and edges (relations of its
own choosing) connecting them. It is free to name patterns it actually observes
(`limps-then-jams`, `tilts-after-a-big-loss`, …) rather than pick from a menu, and to add
relations like `supported_by`, `contradicts`, or `superseded_by` as it sees fit. The previous
fixed pattern slugs, hand-tag set, and `shows_pattern` / `supported_by` relations are no longer
part of the contract; they may appear only if the model chooses them.

## Decision-time retrieval tools

The durable decision session registers three generic, read-only graph tools (the schema-specific
`akg_get_opponent` / `akg_list_patterns` / … tools are gone, because the model now authors an
open vocabulary they could not describe):

- `akg_list_nodes` — list node summaries (`type`, `id`, `title`, `tags`), optionally filtered
  by type or tag.
- `akg_get_node` — return a node's `title`, `body`, `meta`, `tags`, plus its outbound and
  inbound edges; the model follows an edge by reading its other endpoint.
- `akg_get_nodes` — read several nodes in one call (an array of `{type, id}` refs), returning
  the same per-node shape as `akg_get_node`, with `found:false` for any missing ref. Collapses
  a multi-node drill-down into a single turn instead of sequential `akg_get_node` round-trips.

Builtin Pi tools are disabled for this agent. Tool results are JSON-serializable and
deterministic for empty/unknown cases (missing nodes return `found:false`).

The decision system prompt requires **active retrieval**, mirroring the wiki agent's mandatory
link-following: the inline index is an index, not the answer. Before deciding, the model follows
the index's edges and reads the relevant tendency nodes with `akg_get_node` / `akg_get_nodes`
(using `akg_list_nodes` to discover what else is recorded), rather than relying on the inline
index summary alone. The root body is already injected, so the model does not re-fetch it. This
keeps the index slim and the per-decision cost bounded, since detail is pulled on demand only
for spots that need it — the graph analogue of `md_read_page` in `llm-md-wiki-spec.md`.

## Post-hand update tools and inconsistency diagnostics

The update session registers the two read tools above plus three write tools:

- `akg_apply` — apply a whole update in one call: an optional `nodes` array (upserted first) and
  an optional `edges` array (applied after, so endpoints created in the same call resolve). This
  is the preferred write path and keeps the update session to a single turn instead of one
  round-trip per write. Returns per-item `{written, …}` results under `nodes` and `edges`.
- `akg_put_node` — create or replace a single node with a model-chosen `type`, `id`, `title`,
  `body`, `meta`, and `tags`. Retained for one-off writes; `akg_apply` is preferred.
- `akg_put_edge` — create or replace a single directed edge with a model-chosen `relation`
  between two existing nodes. Retained for one-off writes; `akg_apply` is preferred.

There is **no delete tool**, symmetric with the wiki agent's `md_write_page` ("does not delete
pages"): the model retires stale state by overwriting a node body or authoring a superseding
edge. All write tools return structured `{written:false, error}` results on malformed input
or a missing edge endpoint, rather than throwing.

After each update the runtime records a best-effort **graph-rot diagnostic** to
`diagnostics.jsonl` (node count, edge count, and orphan-node count — non-root nodes with no
edges). This only *counts* structural rot; it never repairs it, symmetric with the wiki
agent's link-rot policy. (Because the SDK refuses edges to missing endpoints, truly dangling
edges cannot form.)

The model may call tools before returning its poker action. The final response must still be JSON only:

```json
{"action":"call"}
```

or, for sized actions:

```json
{"action":"raise","amount":12}
```

The server-provided `legal_actions` remain authoritative. The shared runtime validates model output and falls back safely on malformed or illegal responses.

## Prompt contract

The durable decision system prompt tells the model:

- it is a heads-up no-limit Texas Hold'em decision engine
- a knowledge graph about the opponent is available through AKG tools
- the `opponent/villain` summary is **already injected inline** in the prompt; it should be read
  there directly and **not** re-fetched with a tool call
- the read tools (`akg_list_nodes`, `akg_get_node`, `akg_get_nodes`) are for drilling into
  detail the inline summary does not cover — most decisions need no tool calls
- the final answer must be exactly one legal action encoded as JSON

`beforeDecision` injects the root node body as the index (or a "no reads yet" reminder when
the graph is empty), rather than preloading a fixed history block. This is the active-retrieval
distinction from `llm-akg-recent`. Because the summary is supplied inline, the prompt steers the
model away from a reflexive per-decision re-read of `opponent/villain` (a redundant round-trip),
which keeps routine decisions to a single turn while preserving tool-driven retrieval for the
spots that need deeper graph detail.

The update session uses its own system prompt instructing the model to fold the just-finished
hand into the graph: discover existing nodes with the read tools (the graph is not inlined),
keep the `opponent/villain` summary current, author nodes for observed tendencies with its own
types/relations, keep its own counts, avoid duplicate nodes, reconcile contradictions toward
newest evidence, and not delete. It is told to **record only significant hands** (leaving
trivial preflop folds and blind steals unwritten), to **prefer durable tendency nodes over a
node per hand**, and to make all its writes in a **single `akg_apply` call**.

## Artifacts

A durable-agent session can produce:

- `agents/<name>/memory.akg` — authoritative durable graph memory
- `agents/<name>/memory-export.json` — additive JSON export for offline analysis
- `agents/<name>/pi-session.jsonl` — decision Pi transcript / observability log
- `agents/<name>/update-session.jsonl` — post-hand update transcript (separate from decisions)
- `agents/<name>/diagnostics.jsonl` — per-hand graph-rot counts (count-only, no repair)
- `agents/<name>/stderr.log` — update-failure and fallback diagnostics

`memory.akg` is the primary memory store. `memory-export.json` is an analysis artifact and should not be treated as the source of truth when it disagrees with `memory.akg`.

## Constraints to preserve

- Post-hand writes are the only durable graph writes.
- The model performs those writes via tools; no code computes or rebuilds graph state.
- Decision-time tools stay read-only.
- `memory_dir` comes from the server and scopes each session's graph and logs.
- A failed update keeps the prior graph; the runtime counts inconsistencies but never repairs them.
- The shared Pi runner remains responsible for protocol handling, action validation, retries, and safe fallback behavior.
- Evaluation should use the experiment-first workflow in [`eval-system.md`](eval-system.md).

## Related implementation note

See [`kb/llm-akg-durable-active-retrieval.md`](kb/llm-akg-durable-active-retrieval.md) for package-level implementation details and test coverage notes.
