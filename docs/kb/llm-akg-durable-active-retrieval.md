# LLM AKG Durable Active Retrieval

EPIC-10 delivered the first AKG strategy intended to test the project thesis directly: `llm-akg-durable`.

## Epic delivery summary

The archived EPIC-10 task log splits the work into eight slices:
- `EPIC-10-001`: scaffolded the `llm-akg-durable` workspace package and published bin
- `EPIC-10-002`: implemented completed-hand persistence plus opponent/hand writes
- `EPIC-10-003`: implemented pattern derivation, thresholding, and idempotent evidence edges
- `EPIC-10-004`: added the five read-only AKG query tools
- `EPIC-10-005`: wired a hand-scoped custom Pi session with AKG-only tools and the durable prompt contract
- `EPIC-10-006`: added deterministic unit coverage for heuristics, thresholds, and tool behavior
- `EPIC-10-007`: added seam-level and subprocess integration coverage
- `EPIC-10-008`: verified first-class workspace, `poker-run`, and operator-doc surfaces

## Role in the strategy ladder

`llm-akg-durable` replaces the passive memory dump used by `llm-akg-recent` with active retrieval. The model is told that AKG memory exists, then chooses which graph tools to call before returning its final poker action JSON.

This is the current step-2 comparison agent:
- `llm-akg-recent` vs `llm-stateless` established the shallow-memory baseline
- `llm-akg-durable` vs `llm-akg-recent` is the first direct test of active graph retrieval beating passive prompt stuffing
- later work should compare durable against `llm-stateless` and `llm-fullhistory` using the same reporting surface described in [`repeatable-benchmark-reporting.md`](repeatable-benchmark-reporting.md)

## Current package and runtime shape

The durable agent lives in:
- `pi-agents/llm-akg-durable/src/main.ts`
- `pi-agents/llm-akg-durable/src/memory.ts`
- `pi-agents/llm-akg-durable/src/tools.ts`
- `pi-agents/llm-akg-durable/src/runtime.ts`
- `pi-agents/llm-akg-durable/test/memory.test.ts`
- `pi-agents/llm-akg-durable/test/main.test.ts`

The important boundary to preserve is that `pi-agents/shared/` did not need durable-specific refactors. `llm-akg-durable` plugs into the existing shared runner through:
- a strategy-owned `MemoryPolicy`
- a strategy-owned decision engine setup
- a custom `sessionFactory` passed into `PiDecisionEngine`

That keeps the server-authoritative protocol flow, legality checks, retry behavior, and session lifecycle in the shared runtime while letting the durable package own AKG writes and tool registration.

## Durable memory write model

`AkgDurableMemoryPolicy.afterHandEnd(...)` is the strategic write point. It resolves `memory_dir` from the current hand or a previously seen session value, opens `memory.akg`, writes durable state, and commits.

Current persistence behavior:
- every completed hand writes or rewrites one `hand` node keyed by `hand_number`
- hand metadata stores the derived booleans and counts needed to rebuild higher-level summaries deterministically
- the opponent profile is rebuilt by sweeping all stored hand nodes, not by incrementally mutating counters in place
- pattern nodes and edges are also rebuilt from the stored hand-node evidence on each completed hand

That rebuild-from-hands approach is why pattern updates remain logically idempotent. Reprocessing the same hand number rewrites the hand record and then recomputes opponent/pattern state from the durable hand set instead of blindly incrementing counters.

## What gets stored

### Opponent node

The agent maintains `opponent/villain` with:
- `hands_played`, `vpip`, `pfr`, `fold_to_bet`
- per-street aggression counts
- c-bet opportunity/fold counts
- `three_bet_count`, `river_bet_count`, `river_bet_folds`
- showdown counts and villain showdown wins

The node body is rebuilt as a short natural-language summary each hand. Current phrasing includes style classification from VPIP/PFR, c-bet-fold behavior, river aggression frequency, and showdown summary.

### Hand nodes

Each completed hand stores:
- `hand_number`
- hero position
- hero net result
- last street reached
- showdown flag
- all derived durable-feature booleans used by opponent and pattern rebuilds

Current hand tags include:
- `hand`
- `showdown`
- `big_pot`
- `hero_fold`
- `villain_fold`
- `3bet_hand`
- `aggressive_hand`

The hand body is a single-line human-readable summary with hero position, hero hole cards, final board, street-by-street action summary, terminal street, and hero net.

### Pattern nodes and edges

Current durable patterns are:
- `folds-to-cbet`
- `3bet-tendency`
- `river-aggressor`
- `folds-to-river-bet`
- `calls-wide`

Pattern creation rules match the durable spec thresholds. `rebuildPatterns(...)` writes:
- `opponent/villain --[shows_pattern]--> pattern/<slug>` with `count` and `opportunities` metadata
- `pattern/<slug> --[supported_by]--> hand/<id>` for each supporting hand

Before writing support edges, the implementation deletes stale `supported_by` edges that are no longer part of the desired evidence set. That is the main protection against duplicated evidence when the same hand is replayed or rewritten.

## Decision-time retrieval surface

The durable package registers only five custom Pi tools:
- `akg_get_opponent`
- `akg_list_patterns`
- `akg_get_pattern`
- `akg_list_hands`
- `akg_get_hand`

Important current behavior:
- builtin Pi tools are disabled via `noTools: "builtin"`
- tool results are JSON-serializable and returned both as text content and structured `details`
- empty/unknown cases are deterministic: null opponent body, empty pattern list, null unknown pattern, null unknown hand
- `akg_list_hands` sorts most recent first and supports optional `tag` plus `limit`
- `akg_get_pattern` returns supporting hand IDs from `supported_by` edges

## Hand-scoped Pi session contract

`runtime.ts` creates the durable Pi session with:
- session scope `hand`
- compaction disabled
- retry disabled inside Pi itself
- extensions, skills, themes, prompt templates, context files, and builtin tools disabled
- the durable system prompt injected through `systemPromptOverride`

The durable prompt contract is strict:
- the model may use AKG tools before answering
- the final answer must still be JSON only
- the final action must come from the server-advertised legal action set

`beforeDecision(...)` intentionally injects only a lightweight reminder:
- when memory exists: `AKG memory is available. Call akg_get_opponent to read the opponent index.`
- when no `memory_dir` exists: an explicit no-memory fallback line

That keeps retrieval active rather than preloading a fixed summary block.

## Operator and integration surfaces

EPIC-10 also made the agent first-class outside the package itself:
- `pi-agents/package.json` includes `llm-akg-durable` in workspace build/typecheck/test flows
- `cmd/poker-run` resolves the `llm-akg-durable` alias with the same model and thinking env forwarding used by the other Pi agents
- `README.md` and `pi-agents/README.md` document the agent as the durable AKG retrieval strategy
- the stable executable is `poker-agent-llm-akg-durable`

## Verification surface delivered

Current automated coverage proves:
- completed-hand persistence works even if the hero never received `your_turn` in that hand
- opponent stat accumulation matches the intended heuristics across multiple hands
- hand tagging covers showdown, big pots, folds, 3-bets, and aggressive-action thresholds
- pattern thresholds hold below and at the creation boundary
- pattern growth updates counts, opportunities, and support edges beyond the threshold
- evidence-edge bookkeeping stays idempotent at the logical level
- query tools handle empty and unknown lookups deterministically
- durable session wiring registers exactly the AKG tool set and disables builtin tools
- the published subprocess command speaks the protocol cleanly from `session_init` through `session_end`
- the canonical `pi-session.jsonl` artifact is still produced under the hand-scoped lifecycle

## Constraints to preserve in follow-on work

When extending this agent, preserve these current assumptions unless the spec changes first:
- post-hand writes are the only durable graph write point
- `memory_dir` comes from the server and is reused as the AKG home plus default Pi session-log home
- hand nodes are the source of truth for rebuilding opponent and pattern state
- durable retrieval should stay read-only during decisions
- shared runtime changes are not required for strategy-specific retrieval behavior

## Related references

- [`../llm-akg-durable-spec.md`](../llm-akg-durable-spec.md)
- [`../spec.md`](../spec.md)
- [`llm-fullhistory-baseline.md`](llm-fullhistory-baseline.md)
- [`llm-stateless-pi-baseline.md`](llm-stateless-pi-baseline.md)
- [`repeatable-benchmark-reporting.md`](repeatable-benchmark-reporting.md)
