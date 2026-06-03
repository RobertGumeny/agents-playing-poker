# LLM AKG Durable Active Retrieval

`llm-akg-durable` is the active-retrieval AKG strategy. It is **agent-maintained**: after each
completed hand the model writes its own graph memory via tools, and during decisions it reads
that graph with read-only tools. (It was previously code-maintained — a deterministic feature
extractor wrote the graph — which made its fidelity result tautological; see
[`../research/llm-akg-durable-rework.md`](../research/llm-akg-durable-rework.md).)

## Current package and runtime shape

The durable agent lives in:

- `pi-agents/llm-akg-durable/src/main.ts`
- `pi-agents/llm-akg-durable/src/memory.ts` — policy: `beforeDecision` (root injection) + `afterHandEnd`
- `pi-agents/llm-akg-durable/src/graph.ts` — root-node seeding, rot counting, store helpers
- `pi-agents/llm-akg-durable/src/tools.ts` — generic read tools + open-vocabulary write tools
- `pi-agents/llm-akg-durable/src/update.ts` — post-hand update session (mirrors the wiki agent)
- `pi-agents/llm-akg-durable/src/runtime.ts`
- `pi-agents/llm-akg-durable/test/`

The shared runtime boundary is important: `pi-agents/shared/` owns protocol handling, state tracking, action validation, retries, safe fallback, and session lifecycle. The durable package owns memory writes, AKG tool definitions, and its decision-time prompt contract.

## Durable memory write model

`AkgDurableMemoryPolicy.afterHandEnd(...)` is the only durable strategic write point. It calls
`runDurableUpdate` (in `update.ts`), which mirrors the wiki agent's `runWikiUpdate`:

- resolves `memory_dir` from server-provided session state and opens `memory.akg`
- seeds the `opponent/villain` root node if absent
- builds an update prompt from the current node list, the root node body, and a
  `formatCompletedHand` summary of the just-finished hand
- runs a fresh one-shot Pi update session (same model/thinking level as decisions) with read +
  write tools, lets the **model** decide what to write, then commits
- exports the update transcript to a separate `update-session.jsonl`
- on failure, keeps the prior graph and logs to `stderr.log`

In fake-decision mode (no live model) it applies a deterministic scripted update (writes a
`hand/<n>` node, links it from the root, appends the summary to the root body) so the
read/inject/update loop is exercised in tests. No code computes opponent stats or patterns.

### File growth: WAL accumulation, not data

The store is committed each hand but never `compact()`ed, so the on-disk `.akg` grows faster
than the logical graph (log-structured WAL history). The 500-hand `mirror-match-500` measurement
(~8.2 MB on disk vs ~575 KB logical, ~93% WAL) predates the agent-maintained rework — under the
old code path the opponent and all pattern nodes were re-`putNode`d every hand, which inflated
WAL further; the model-maintained path writes only what the model touches, so growth depends on
model behavior. Either way this caused **zero operational trouble** and is a cost/efficiency
issue, not correctness: a single `compact()` collapses the file (see
`~/source/akg/docs/spec/06-compaction.md`). Periodic/end-of-session compaction belongs in the
planned `pi-akg-memory` package so every agent inherits bounded files.

## Stored graph shape

The shape is **authored by the model** (open vocabulary), not fixed by code. The runtime only
seeds and injects the `opponent/villain` root index node; everything else — node types, ids,
tendency/pattern names, tags, and edge relations — is the model's to invent and name, the
structured analogue of the wiki agent's free-form pages and `[[links]]`. The old fixed pattern
slugs (`folds-to-cbet`, `3bet-tendency`, …), hand-tag set, and `shows_pattern` / `supported_by`
relations are no longer part of the contract; they appear only if the model chooses them.

## Decision-time retrieval surface

The decision session registers two generic, read-only graph tools (the old schema-specific
five are gone, since the model now authors an open vocabulary):

- `akg_list_nodes` — list node summaries, optionally filtered by type or tag
- `akg_get_node` — read a node's title/body/meta/tags plus its outbound and inbound edges

The update session additionally registers `akg_put_node` and `akg_put_edge` (open vocabulary,
no delete). Builtin Pi tools are disabled. Tool results are JSON-serializable and deterministic
for empty/unknown lookups; write tools return structured `{written:false, error}` on bad input.

The model may inspect AKG before returning its final action JSON. It must still choose from server-advertised legal actions, and the shared runner still validates and falls back safely on malformed or illegal output.

After each update the runtime writes a count-only `diagnostics.jsonl` line (nodes, edges,
orphan nodes); it never repairs rot, symmetric with the wiki link-rot policy.

## Prompt and session contract

`runtime.ts` creates a hand-scoped decision Pi session, and `update.ts` creates a one-shot
update Pi session per completed hand. Both sessions are configured with:

- compaction disabled
- retry disabled inside Pi itself
- builtin tools disabled
- extensions, skills, themes, prompt templates, and context files disabled
- a strategy system prompt injected through `systemPromptOverride` (decision vs update prompts)

`beforeDecision(...)` injects the `opponent/villain` root node body as the index (or a "no
reads yet" reminder when empty) rather than preloading a fixed history block. This is the
active-retrieval distinction from `llm-akg-recent`.

## Operator integration

The agent is available as:

- strategy alias: `llm-akg-durable`
- executable: `poker-agent-llm-akg-durable`

Run it through experiment definitions and the root CLI:

```bash
go run ./cmd/poker experiment go <experiment-id>
```

## Verification surface

Automated coverage proves:

- `beforeDecision` seeds the root node and injects only the root index body
- read tools (`akg_list_nodes`, `akg_get_node`) return deterministic results, including
  not-found and empty-store cases, and surface a node's edges
- write tools (`akg_put_node`, `akg_put_edge`) create nodes/edges and report structured errors
  (missing edge endpoint, invalid type name) instead of throwing
- the update prompt includes the node list, current root body, and hand summary
- scripted (fake) update writes a hand node + root edge, updates the root body, and logs a
  separate `update-session.jsonl` transcript and a `diagnostics.jsonl` rot line
- the subprocess command speaks the protocol through `session_end`, and the second decision
  prompt injects the root body updated by the first hand

## Constraints to preserve

- Post-hand writes are the only durable graph writes, and the model performs them via tools.
- No code computes or rebuilds graph state.
- Decision-time retrieval stays read-only.
- `memory_dir` scopes each agent's graph and Pi logs.
- A failed update keeps the prior graph; inconsistencies are counted, never repaired.
- Strategy-specific retrieval should not require durable-specific forks of the shared protocol runner.

## Related references

- [`../llm-akg-durable-spec.md`](../llm-akg-durable-spec.md)
- [`llm-fullhistory-baseline.md`](llm-fullhistory-baseline.md)
- [`llm-stateless-pi-baseline.md`](llm-stateless-pi-baseline.md)
- [`repeatable-benchmark-reporting.md`](repeatable-benchmark-reporting.md)
