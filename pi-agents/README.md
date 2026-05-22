# Pi Agents

TypeScript/Pi-based poker agents live here. These agents are external clients of the Go game server and should depend only on the documented JSONL wire protocol, not on Go `internal/...` packages.

## Workspace

This directory is a deliberately narrow npm workspace for Pi agents only. It does not introduce repo-wide JavaScript tooling.

Packages:

- `shared/`: shared protocol, state, prompt, action validation, runner, and Pi-session seams.
- `llm-nomemory/`: first LLM baseline; current-hand prompt only, with no strategic memory exposed to the model.
- `llm-fullhistory/`: future naive memory baseline; injects hand history into prompt context.
- `llm-akg/`: future structured-memory agent; uses AKG retrieval/tools and compaction-aware Pi behavior.

## Commands

From `pi-agents/`:

- `npm install`
- `npm run typecheck`
- `npm test`

Target only the shared runtime package when needed:

- `npm run typecheck --workspace @agent-poker/pi-agent-shared`
- `npm run test --workspace @agent-poker/pi-agent-shared`

The shared runtime tests cover protocol helpers, state updates, prompt construction, action validation/fallback, and the stdio runner loop with fake decision clients.

Pi session logs are observability artifacts and should be stored durably, but they are separate from strategic memory. A no-memory agent may persist Pi logs while still ensuring previous hands are not visible to future decisions.
