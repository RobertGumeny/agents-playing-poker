# Adding a Memory Strategy

## Overview

A memory strategy is one agent package under `engine/pi-agents/`. It consists of two things:

1. A `MemoryPolicy` — decides what context to inject before each decision and what to persist after each hand.
2. A `main.ts` — wires the policy to the shared runtime.

The shared runtime (`pi-agents/shared/`) owns everything else: the JSONL protocol loop, prompt assembly, action validation, retry budgeting, and Pi session lifecycle. A new strategy touches none of that.

---

## Minimal strategy checklist

```
engine/pi-agents/
  llm-your-strategy/
    src/
      main.ts        ← entry point (~15 lines)
      memory.ts      ← MemoryPolicy implementation
    test/
      memory.test.ts ← optional but expected
    package.json
    tsconfig.json
```

Copy `llm-fullhistory/` as a starting point — it is the simplest stateful strategy.

---

## Step 1 — Implement `MemoryPolicy`

```typescript
// src/memory.ts
import type {
  CompletedHandContext,
  DecisionContext,
  MemoryPolicy,
  PromptAugmentation,
} from "@agent-poker/pi-agent-shared";

export class YourMemoryPolicy implements MemoryPolicy {
  // Expose memoryDir so main.ts can pass it to createStandardDecisionEngine.
  // The runner sets this via context.state.session?.memoryDir on the first call.
  private serverMemoryDir: string | undefined;

  get memoryDir(): string | undefined {
    return this.serverMemoryDir;
  }

  async beforeDecision(context: DecisionContext): Promise<PromptAugmentation> {
    this.serverMemoryDir = context.state.session?.memoryDir;

    // Return zero or more plain-text sections that will be injected into the
    // decision prompt before the current game state. Return { sections: [] }
    // for no augmentation (same as llm-stateless).
    return {
      sections: [
        "Prior hands: none yet.",
      ],
    };
  }

  async afterHandEnd(context: CompletedHandContext): Promise<void> {
    // Persist whatever you need for future beforeDecision calls.
    // context.heroHoleCards, context.board, context.actionHistory,
    // context.showdown, context.result are all available here.
    // Write to files under this.serverMemoryDir if you need durable storage.
  }
}
```

### What `CompletedHandContext` gives you

| Field | Type | Notes |
|---|---|---|
| `handNumber` | `number` | monotonically increasing |
| `heroSeat` | `number` | your seat number |
| `seats` | `Array<{seat, name}>` | all players |
| `heroHoleCards` | `string[]` | e.g. `["As", "Kd"]` |
| `board` | `string[]` | community cards dealt so far |
| `actionHistory` | `ActionHistoryEntry[]` | server-authoritative action log |
| `showdownReached` | `boolean` | |
| `showdown` | `Record<string, ShowdownEntry>` | only present when true |
| `result` | `Array<{seat, chips_delta}>` | final chip deltas |
| `dealerSeat` | `number` | dealer/small blind seat |

### What `DecisionContext` gives you

The same fields from the live `your_turn` message: `handNumber`, `street`, `board`, `pot`, `toCall`, `stacks`, `actionHistory`, `legalActions`. Plus `context.state.session?.memoryDir` for the server-scoped storage path.

---

## Step 2 — Write `main.ts`

```typescript
#!/usr/bin/env node

import {
  createStandardDecisionEngine,
  parsePositiveInteger,
  runPokerAgent,
} from "@agent-poker/pi-agent-shared";

import { YourMemoryPolicy } from "./memory.js";

const memoryPolicy = new YourMemoryPolicy();

await runPokerAgent({
  memoryPolicy,
  decisionEngine: createStandardDecisionEngine({
    sessionScope: "hand",           // "decision" = new Pi session per turn
    memoryDirProvider: () => memoryPolicy.memoryDir,
  }),
  agentVersion: "llm-your-strategy/0.1.0",
  maxDecisionAttempts: parsePositiveInteger(process.env.PI_POKER_MAX_DECISION_ATTEMPTS),
});
```

`createStandardDecisionEngine` reads the standard env vars automatically:

| Env var | Effect |
|---|---|
| `PI_POKER_MODEL` | Pi model spec (e.g. `anthropic:claude-sonnet-4-5`) |
| `PI_POKER_THINKING_LEVEL` | `off` / `minimal` / `low` / `medium` / `high` / `xhigh` |
| `PI_POKER_PI_SESSION_DIR` | explicit Pi session output dir (overrides memoryDir) |
| `PI_POKER_FAKE_DECISIONS_JSON` | scripted action list for tests |
| `PI_POKER_MAX_DECISION_ATTEMPTS` | retry cap (default: 3) |

### `sessionScope` guidance

- `"decision"` — fresh Pi context per turn. No cross-turn reasoning. Matches `llm-stateless`.
- `"hand"` — Pi session persists for the whole hand. Pi sees its own prior reasoning within the hand. All stateful strategies use this.

---

## Step 3 — Wire the package

**`package.json`** (copy from `llm-fullhistory/package.json`, update `name` and `bin`):
```json
{
  "name": "@agent-poker/llm-your-strategy",
  "version": "0.1.0",
  "type": "module",
  "bin": {
    "poker-agent-llm-your-strategy": "dist/main.js"
  },
  "dependencies": {
    "@agent-poker/pi-agent-shared": "*"
  },
  "devDependencies": {
    "typescript": "...",
    "vitest": "..."
  },
  "scripts": {
    "build": "tsc -p tsconfig.json",
    "test": "npm run build --workspace @agent-poker/pi-agent-shared && npm run build && vitest run"
  }
}
```

**`tsconfig.json`** — copy from any peer agent; no changes needed.

Add the package to the `workspaces` array in `engine/pi-agents/package.json`.

---

## Step 4 — Register as an experiment agent

In your experiment definition JSON (`research/experiments/<slug>/experiment.json`), add the strategy to the `agents` array:

```json
{
  "agent_tag": "your-strategy",
  "binary": "poker-agent-llm-your-strategy",
  "model": "anthropic:claude-sonnet-4-5",
  "thinking_level": "low"
}
```

Build first so the binary exists:
```sh
cd engine/pi-agents && npm run build
```

---

## Durable storage

If your strategy needs files that persist across Pi sessions (AKG graphs, markdown files, SQLite, etc.), use `context.state.session?.memoryDir` — the server creates and scopes this directory per session. It is available from the first `beforeDecision` call.

```typescript
import { join } from "node:path";
import { readFile, writeFile } from "node:fs/promises";

const filePath = join(this.serverMemoryDir, "notes.md");
```

The server guarantees this directory exists when provided. If it is `undefined`, the agent is running without a memory dir (e.g. in a scripted smoke test) — handle gracefully.

---

## Custom Pi tools (advanced)

If your strategy needs to inject custom tools into Pi (like `llm-akg-durable` does), skip `createStandardDecisionEngine` and build your own decision engine using `PiDecisionEngine` directly with its `sessionFactory` option:

```typescript
import { PiDecisionEngine, parseFakeDecisions, parsePiThinkingLevel } from "@agent-poker/pi-agent-shared";
import { createMySessionFactory } from "./session-factory.js";

const engine = new PiDecisionEngine({
  cwd: process.cwd(),
  sessionDirProvider: () => memoryPolicy.memoryDir,
  model: process.env.PI_POKER_MODEL,
  thinkingLevel: parsePiThinkingLevel(process.env.PI_POKER_THINKING_LEVEL),
  sessionScope: "hand",
  sessionFactory: createMySessionFactory(memoryPolicy),
});
```

See `llm-akg-durable/src/runtime.ts` (`createDurableSessionFactory`) for a complete example.

---

## Verification

1. Build: `cd engine/pi-agents && npm run build`
2. Smoke test with scripted decisions:
   ```sh
   PI_POKER_FAKE_DECISIONS_JSON='[{"action":"fold"}]' \
     node dist/main.js
   ```
   Feed it a valid `session_init` + `hand_start` + `your_turn` JSONL sequence on stdin.
3. Unit tests: `npm test` from the strategy package dir.
4. Integration: run a short match via `poker run` and inspect `hands.jsonl` and `eval.json`.

---

## Related references

- [`../wire-protocol.md`](../wire-protocol.md) — full server/agent JSONL protocol
- [`../session-artifacts.md`](../session-artifacts.md) — `memory-export.json` and `eval.json` schemas
- [`../llm-akg-durable-spec.md`](../llm-akg-durable-spec.md) — durable AKG agent contract (reference for tool-based strategies)
- [`llm-stateless-pi-baseline.md`](llm-stateless-pi-baseline.md) — simplest strategy (no memory)
- [`llm-fullhistory-baseline.md`](llm-fullhistory-baseline.md) — simplest stateful strategy (prompt injection)
- [`llm-akg-durable-active-retrieval.md`](llm-akg-durable-active-retrieval.md) — tool-based retrieval reference
