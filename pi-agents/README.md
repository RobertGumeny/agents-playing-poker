# Pi Agents

TypeScript/Pi-based poker agents live here. These agents are external clients of the Go game server and should depend only on the documented JSONL wire protocol, not on Go `internal/...` packages.

Planned layout:

- `shared/`: protocol, state, prompt, action validation, Pi SDK, and memory-strategy infrastructure shared by all Pi agents.
- `llm-nomemory/`: first LLM baseline; current-hand prompt only, with no strategic memory exposed to the model.
- `llm-fullhistory/`: future naive memory baseline; injects hand history into prompt context.
- `llm-akg/`: future structured-memory agent; uses AKG retrieval/tools and compaction-aware Pi behavior.

Pi session logs are observability artifacts and should be stored durably, but they are separate from strategic memory. A no-memory agent may persist Pi logs while still ensuring previous hands are not visible to future decisions.
