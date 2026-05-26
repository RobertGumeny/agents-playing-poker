# Pi Agents

TypeScript/Pi-based poker agents live here. These agents are external clients of the Go game server and should depend only on the documented JSONL wire protocol, not on Go `internal/...` packages.

## Workspace

This directory is a deliberately narrow npm workspace for Pi agents only. It does not introduce repo-wide JavaScript tooling.

Packages:

- `shared/`: shared protocol, state, prompt, action validation, runner, and Pi-session seams.
- `llm-stateless/`: first LLM baseline; current-hand prompt only, with no strategic memory exposed to the model.
- `llm-fullhistory/`: naive memory baseline; uses a fresh Pi session per hand and injects compact prior-hand summaries into the prompt.
- `llm-akg-recent/`: shallow AKG-backed recent-memory baseline.

## Commands

From `pi-agents/`:

- `npm install`
- `npm run build`
- `npm run typecheck`
- `npm test`

Target only the shared runtime package when needed:

- `npm run typecheck --workspace @agent-poker/pi-agent-shared`
- `npm run test --workspace @agent-poker/pi-agent-shared`

The shared runtime tests cover protocol helpers, state updates, prompt construction, action validation/fallback, the stdio runner loop, and both per-decision and per-hand Pi session lifecycle seams.

Pi session logs are observability artifacts and should be stored durably, but they are separate from strategic memory. A stateless agent may persist Pi logs while still ensuring previous hands are not visible to future decisions. When the server provides `session_init.memory_dir`, `llm-stateless` uses that session bundle agent directory as the default home for the canonical `pi-session.jsonl` artifact.

Memory-strategy boundaries:
- `llm-stateless`: fresh Pi session per decision; no prior-hand prompt context.
- `llm-fullhistory`: fresh Pi session per hand; prior hands injected explicitly as compact human-readable summaries derived from server-visible history. Hand history grows throughout match.
- `llm-akg-recent`: long-lived structured-memory baseline with AKG-backed recent-hand retrieval.

## `llm-stateless` install/run

Build the workspace, then use the package bin as the stable agent command:

```bash
cd pi-agents
npm install
npm run build
npm exec --workspace @agent-poker/llm-stateless poker-agent-llm-stateless
```

That same executable is suitable for `poker-server -agent*-cmd`, with each additional server-side `-agent*-arg` passed as a separate process argument in the normal `exec.Command` style.

## `llm-fullhistory` install/run

Build the workspace, then use the package bin as the stable agent command:

```bash
cd pi-agents
npm install
npm run build
npm exec --workspace @agent-poker/llm-fullhistory poker-agent-llm-fullhistory
```

The same executable is suitable for `poker-server -agent*-cmd`. Each hand uses a fresh Pi session; prior-hand summaries are injected into the prompt at the start of each decision.

## `llm-akg-recent` install/run

Build the workspace, then use the package bin as the stable agent command:

```bash
cd pi-agents
npm install
npm run build
npm exec --workspace @agent-poker/llm-akg-recent poker-agent-llm-akg-recent
```

The same executable is suitable for `poker-server -agent*-cmd`. The agent persists its AKG memory file under the server-provided `memory_dir`.

## Pi-agent runtime knobs

`llm-stateless` and `llm-fullhistory` currently read these optional environment variables:

- `PI_POKER_MODEL`: Pi model selector (`provider:model-id` or `provider/model-id`)
- `PI_POKER_THINKING_LEVEL`: Pi thinking level (`off`, `minimal`, `low`, `medium`, `high`, `xhigh`)
- `PI_POKER_MAX_DECISION_ATTEMPTS`: shared runner retry budget before safe fallback
- `PI_POKER_PI_SESSION_DIR`: override directory for the canonical `pi-session.jsonl` audit log (defaults to `session_init.memory_dir` when the server provides one)
