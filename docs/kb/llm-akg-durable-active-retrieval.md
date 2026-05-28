# LLM AKG Durable Active Retrieval

`llm-akg-durable` is the active-retrieval AKG strategy. It stores durable graph memory after completed hands and gives the model read-only AKG tools during decisions.

## Current package and runtime shape

The durable agent lives in:

- `pi-agents/llm-akg-durable/src/main.ts`
- `pi-agents/llm-akg-durable/src/memory.ts`
- `pi-agents/llm-akg-durable/src/tools.ts`
- `pi-agents/llm-akg-durable/src/runtime.ts`
- `pi-agents/llm-akg-durable/test/`

The shared runtime boundary is important: `pi-agents/shared/` owns protocol handling, state tracking, action validation, retries, safe fallback, and session lifecycle. The durable package owns memory writes, AKG tool definitions, and its decision-time prompt contract.

## Durable memory write model

`AkgDurableMemoryPolicy.afterHandEnd(...)` is the only durable strategic write point.

Current behavior:

- resolves `memory_dir` from server-provided session state
- opens `memory.akg`
- writes or rewrites the completed hand node
- rebuilds the opponent profile from hand nodes
- rebuilds pattern nodes and evidence edges from hand nodes
- commits the store

Rebuilding from hand nodes keeps profile and pattern state logically idempotent. Reprocessing the same hand number rewrites that hand and recomputes derived state instead of blindly incrementing counters.

## Stored graph shape

The graph stores:

- `opponent/villain`
- one `hand` node per completed hand
- pattern nodes for repeated tendencies
- `shows_pattern` edges from villain to patterns
- `supported_by` edges from patterns to evidence hands

Current patterns:

- `folds-to-cbet`
- `3bet-tendency`
- `river-aggressor`
- `folds-to-river-bet`
- `calls-wide`

Current hand tags include:

- `hand`
- `showdown`
- `big_pot`
- `hero_fold`
- `villain_fold`
- `3bet_hand`
- `aggressive_hand`

## Decision-time retrieval surface

The durable package registers exactly five custom Pi tools:

- `akg_get_opponent`
- `akg_list_patterns`
- `akg_get_pattern`
- `akg_list_hands`
- `akg_get_hand`

Builtin Pi tools are disabled. Tool results are JSON-serializable and deterministic for empty or unknown lookups.

The model may inspect AKG before returning its final action JSON. It must still choose from server-advertised legal actions, and the shared runner still validates and falls back safely on malformed or illegal output.

## Prompt and session contract

`runtime.ts` creates a hand-scoped Pi session with:

- compaction disabled
- retry disabled inside Pi itself
- builtin tools disabled
- extensions, skills, themes, prompt templates, and context files disabled
- durable system prompt injected through `systemPromptOverride`

`beforeDecision(...)` injects a lightweight reminder that AKG memory is available rather than preloading a fixed history block. This is the active-retrieval distinction from `llm-akg-recent`.

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

- completed-hand persistence works even if the hero never received a turn in that hand
- opponent stat accumulation matches intended heuristics
- hand tagging covers showdown, big pots, folds, 3-bets, and aggression thresholds
- pattern thresholds hold below and at creation boundaries
- pattern growth updates counts, opportunities, and support edges
- evidence-edge bookkeeping remains idempotent
- query tools handle empty and unknown lookups deterministically
- durable session wiring registers the AKG tool set and disables builtin tools
- the subprocess command speaks the protocol through `session_end`
- `pi-session.jsonl` is produced under the normal session artifact layout

## Constraints to preserve

- Post-hand writes are the only durable graph writes.
- Decision-time retrieval stays read-only.
- `memory_dir` scopes each agent's graph and Pi logs.
- Hand nodes remain the source of truth for rebuilding opponent and pattern state.
- Strategy-specific retrieval should not require durable-specific forks of the shared protocol runner.

## Related references

- [`../llm-akg-durable-spec.md`](../llm-akg-durable-spec.md)
- [`llm-fullhistory-baseline.md`](llm-fullhistory-baseline.md)
- [`llm-stateless-pi-baseline.md`](llm-stateless-pi-baseline.md)
- [`repeatable-benchmark-reporting.md`](repeatable-benchmark-reporting.md)
