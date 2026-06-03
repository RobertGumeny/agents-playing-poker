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
- The update session is given the current node list, the current root node body, and a summary
  of the hand that just finished, then writes its changes with AKG write tools.
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
index to start from, mirroring the wiki agent's seeded `villain.md`. The model is told to keep
this node's body as a current opponent summary and to connect deeper nodes to it with edges.
`beforeDecision` injects this node's body as the decision-time index.

### Model-authored nodes and edges

Beyond the root, the model creates whatever nodes and edges it finds useful — for example a
node per observed tendency, an optional node for a notable hand, and edges (relations of its
own choosing) connecting them. It is free to name patterns it actually observes
(`limps-then-jams`, `tilts-after-a-big-loss`, …) rather than pick from a menu, and to add
relations like `supported_by`, `contradicts`, or `superseded_by` as it sees fit. The previous
fixed pattern slugs, hand-tag set, and `shows_pattern` / `supported_by` relations are no longer
part of the contract; they may appear only if the model chooses them.

## Decision-time retrieval tools

The durable decision session registers two generic, read-only graph tools (the schema-specific
`akg_get_opponent` / `akg_list_patterns` / … tools are gone, because the model now authors an
open vocabulary they could not describe):

- `akg_list_nodes` — list node summaries (`type`, `id`, `title`, `tags`), optionally filtered
  by type or tag.
- `akg_get_node` — return a node's `title`, `body`, `meta`, `tags`, plus its outbound and
  inbound edges; the model follows an edge by reading its other endpoint.

Builtin Pi tools are disabled for this agent. Tool results are JSON-serializable and
deterministic for empty/unknown cases (missing nodes return `found:false`).

## Post-hand update tools and inconsistency diagnostics

The update session registers the two read tools above plus two write tools:

- `akg_put_node` — create or replace a node with a model-chosen `type`, `id`, `title`, `body`,
  `meta`, and `tags`.
- `akg_put_edge` — create or replace a directed edge with a model-chosen `relation` between two
  existing nodes.

There is **no delete tool**, symmetric with the wiki agent's `md_write_page` ("does not delete
pages"): the model retires stale state by overwriting a node body or authoring a superseding
edge. Both write tools return a structured `{written:false, error}` result on malformed input
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
- `opponent/villain` is the starting index; follow its edges by reading connected nodes
- `akg_list_nodes` and `akg_get_node` may be used as needed before the final answer
- the final answer must be exactly one legal action encoded as JSON

`beforeDecision` injects the root node body as the index (or a "no reads yet" reminder when
the graph is empty), rather than preloading a fixed history block. This is the active-retrieval
distinction from `llm-akg-recent`.

The update session uses its own system prompt instructing the model to fold the just-finished
hand into the graph: keep the `opponent/villain` summary current, author nodes for observed
tendencies with its own types/relations, keep its own counts, avoid duplicate nodes, reconcile
contradictions toward newest evidence, and not delete.

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
