# Pi Agents

TypeScript/Pi-based poker agents live here. These agents are external clients of the Go game server and should depend only on the documented JSONL wire protocol, not on Go `internal/...` packages.

## Workspace

This directory is a deliberately narrow npm workspace for Pi agents only. It does not introduce repo-wide JavaScript tooling.

Packages:

- `shared/`: shared protocol, state, prompt, action validation, runner, and Pi-session seams.
- `llm-stateless/`: first LLM baseline; current-hand prompt only, with no strategic memory exposed to the model.
- `llm-fullhistory/`: naive memory baseline; uses a fresh Pi session per hand and injects compact prior-hand summaries into the prompt.
- `llm-md-single/`: single-file prose memory; the model rewrites one markdown notes file after each hand.
- `llm-md-wiki/`: linked-markdown "wiki" memory; the model maintains linked pages and retrieves them on demand through read-only tools.
- `llm-akg-recent/`: shallow AKG-backed recent-memory baseline.
- `llm-akg-durable/`: durable AKG retrieval agent with read-only graph-query tools.

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
- `llm-md-single`: the model rewrites one whole markdown notes file after each hand; the entire file is injected into every decision (no tools).
- `llm-md-wiki`: the model maintains a set of linked markdown pages; only the root index page is injected and deeper pages are pulled on demand through read-only tools.
- `llm-akg-recent`: long-lived shallow structured-memory baseline with AKG-backed recent-hand retrieval.
- `llm-akg-durable`: durable AKG retrieval agent; uses one Pi session per hand, writes richer post-hand graph state, and exposes three read-only AKG query tools during decisions (`akg_list_nodes`, `akg_get_node`, `akg_get_nodes`).

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

## `llm-akg-durable` install/run

Build the workspace, then use the package bin as the stable agent command:

```bash
cd pi-agents
npm install
npm run build
npm exec --workspace @agent-poker/llm-akg-durable poker-agent-llm-akg-durable
```

The same executable is suitable for `poker-server -agent*-cmd`. The agent persists its AKG memory file under the server-provided `memory_dir` and exposes only its read-only AKG query tools to Pi.

## `llm-md-single` install/run

Build the workspace, then use the package bin as the stable agent command:

```bash
cd pi-agents
npm install
npm run build
npm exec --workspace @agent-poker/llm-md-single poker-agent-llm-md-single
```

The same executable is suitable for `poker-server -agent*-cmd`. The agent keeps one `notes.md` file under the server-provided `memory_dir`, rewriting it after each hand and injecting the whole file at decision time.

## `llm-md-wiki` install/run

Build the workspace, then use the package bin as the stable agent command:

```bash
cd pi-agents
npm install
npm run build
npm exec --workspace @agent-poker/llm-md-wiki poker-agent-llm-md-wiki
```

The same executable is suitable for `poker-server -agent*-cmd`. The agent keeps a `wiki/` directory of linked markdown pages under the server-provided `memory_dir`, injecting only the root index page at decision time and exposing read-only page-reading tools to Pi.

## Pi-agent runtime knobs

`llm-stateless`, `llm-fullhistory`, `llm-md-single`, `llm-md-wiki`, `llm-akg-recent`, and `llm-akg-durable` currently read these optional environment variables:

- `PI_POKER_MODEL`: Pi model selector (`provider:model-id` or `provider/model-id`)
- `PI_POKER_THINKING_LEVEL`: Pi thinking level (`off`, `minimal`, `low`, `medium`, `high`, `xhigh`)
- `PI_POKER_MAX_DECISION_ATTEMPTS`: shared runner retry budget before safe fallback
- `PI_POKER_PI_SESSION_DIR`: override directory for the canonical `pi-session.jsonl` audit log (defaults to `session_init.memory_dir` when the server provides one)
