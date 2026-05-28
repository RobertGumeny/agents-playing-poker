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

The only strategic graph write point is after a completed hand.

Current behavior:

- The agent opens `memory.akg` under the server-provided `memory_dir`.
- Each completed hand writes or rewrites one `hand` node keyed by hand number.
- The opponent profile is rebuilt from stored hand nodes.
- Pattern nodes and evidence edges are rebuilt from stored hand nodes.
- The store is committed after the post-hand update.

This rebuild-from-hands model keeps the durable state logically idempotent when a hand is replayed or rewritten.

The agent does **not** write to AKG during a decision. Decision-time AKG access is read-only.

## Stored graph shape

### Opponent node

Identity: `opponent/villain`

The opponent profile tracks cumulative behavior, including:

- hands played
- VPIP and PFR
- fold-to-bet counts
- per-street aggression counts
- c-bet opportunities and c-bet folds
- 3-bet counts
- river bet and river fold counts
- showdown counts and villain showdown wins

The body is a short natural-language summary rebuilt from those counters.

### Hand nodes

Identity: `hand/<hand-number>`

Each completed hand stores compact metadata and a single-line summary. Tags include:

- `hand`
- `showdown`
- `big_pot`
- `hero_fold`
- `villain_fold`
- `3bet_hand`
- `aggressive_hand`

Tags are applied when their conditions are met; every hand node has `hand`.

### Pattern nodes

Identity: `pattern/<slug>`

Current pattern slugs:

- `folds-to-cbet`
- `3bet-tendency`
- `river-aggressor`
- `folds-to-river-bet`
- `calls-wide`

Pattern nodes are created or updated only when their evidence thresholds are met. Each pattern stores a human-readable body plus count/opportunity metadata.

### Edges

Current edge relations:

- `opponent/villain --[shows_pattern]--> pattern/<slug>`
- `pattern/<slug> --[supported_by]--> hand/<hand-number>`

Edges carry raw count/opportunity metadata where applicable. Calibrated strategic confidence scoring is not part of the current contract.

## Decision-time retrieval tools

The durable Pi session registers exactly these read-only AKG tools:

- `akg_get_opponent`
- `akg_list_patterns`
- `akg_get_pattern`
- `akg_list_hands`
- `akg_get_hand`

Builtin Pi tools are disabled for this agent. Tool results are JSON-serializable and deterministic for empty/unknown cases.

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

The durable system prompt tells the model:

- it is a heads-up no-limit Texas Hold'em decision engine
- AKG memory tools are available
- the opponent node is the starting index
- deeper pattern or hand tools may be used when relevant
- the final answer must be exactly one legal action encoded as JSON

`beforeDecision` injects only a lightweight reminder that AKG memory is available, rather than preloading a fixed history block. This is the active-retrieval distinction from `llm-akg-recent`.

## Artifacts

A durable-agent session can produce:

- `agents/<name>/memory.akg` — authoritative durable graph memory
- `agents/<name>/memory-export.json` — additive JSON export for offline analysis
- `agents/<name>/pi-session.jsonl` — Pi transcript / observability log
- `agents/<name>/stderr.log` — retry and fallback diagnostics

`memory.akg` is the primary memory store. `memory-export.json` is an analysis artifact and should not be treated as the source of truth when it disagrees with `memory.akg`.

## Constraints to preserve

- Post-hand writes are the only durable graph writes.
- Decision-time tools stay read-only.
- `memory_dir` comes from the server and scopes each session's graph and logs.
- Hand nodes remain the source of truth for rebuilding profile and pattern state.
- The shared Pi runner remains responsible for protocol handling, action validation, retries, and safe fallback behavior.
- Evaluation should use the experiment-first workflow in [`eval-system.md`](eval-system.md).

## Related implementation note

See [`kb/llm-akg-durable-active-retrieval.md`](kb/llm-akg-durable-active-retrieval.md) for package-level implementation details and test coverage notes.
