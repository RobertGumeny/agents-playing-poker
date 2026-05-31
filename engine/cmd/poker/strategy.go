package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func runStrategy(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("expected subcommand (supported: ls, new)")
	}
	switch args[0] {
	case "ls":
		return runStrategyList(args[1:], stdout)
	case "new":
		return runStrategyNew(args[1:], stdout)
	default:
		return fmt.Errorf("unsupported strategy subcommand %q", args[0])
	}
}

func runStrategyList(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("poker strategy ls", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	repoDir, err := repoRoot()
	if err != nil {
		return err
	}

	entries, err := (&agentResolver{repoDir: repoDir}).loadRegistry()
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(stdout, "%-24s %-10s %-8s\n", "KEY", "TYPE", "BUILT")
	for _, e := range entries {
		built := "-"
		if e.Type == "pi-agent" {
			scriptPath := filepath.Join(repoDir, "pi-agents", e.Key, "dist", "main.js")
			if _, err := os.Stat(scriptPath); err == nil {
				built = "yes"
			} else {
				built = "no"
			}
		} else if e.Type == "go-agent" {
			built = "auto"
		}
		_, _ = fmt.Fprintf(stdout, "%-24s %-10s %-8s\n", e.Key, e.Type, built)
	}
	return nil
}

func runStrategyNew(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("poker strategy new", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: poker strategy new <key>")
	}
	key := fs.Arg(0)
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("strategy key must not be empty")
	}

	repoDir, err := repoRoot()
	if err != nil {
		return err
	}

	resolver := &agentResolver{repoDir: repoDir}
	entries, err := resolver.loadRegistry()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Key == key {
			return fmt.Errorf("strategy %q already exists in registry", key)
		}
	}

	piAgentsDir := filepath.Join(repoDir, "pi-agents")
	agentDir := filepath.Join(piAgentsDir, key)
	srcDir := filepath.Join(agentDir, "src")

	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", srcDir, err)
	}

	pkgJSON := fmt.Sprintf(`{
  "name": "@agent-poker/%s",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "bin": {
    "poker-agent-%s": "./dist/main.js"
  },
  "scripts": {
    "build": "tsc -p tsconfig.json",
    "start": "tsx src/main.ts",
    "typecheck": "tsc -p tsconfig.json --noEmit"
  },
  "dependencies": {
    "@agent-poker/pi-agent-shared": "file:../shared",
    "@earendil-works/pi-coding-agent": "latest"
  },
  "devDependencies": {
    "tsx": "^4.20.6"
  }
}
`, key, key)

	tsconfig := `{
  "extends": "../tsconfig.base.json",
  "compilerOptions": {
    "rootDir": "src",
    "outDir": "dist",
    "declaration": true,
    "emitDeclarationOnly": false
  },
  "include": ["src/**/*.ts"]
}
`

	mainTS := fmt.Sprintf(`#!/usr/bin/env node

import {
  createStandardDecisionEngine,
  parsePositiveInteger,
  runPokerAgent,
  type MemoryPolicy,
} from "@agent-poker/pi-agent-shared";

let serverMemoryDir: string | undefined;

// TODO: implement memory strategy — write to serverMemoryDir after each hand,
// read from it via beforeDecision to inject context into the prompt.
const memoryPolicy: MemoryPolicy = {
  async beforeDecision(context) {
    serverMemoryDir = context.state.session?.memoryDir;
    return { sections: [] };
  },
  async afterHandEnd() {
    // TODO: persist memory here
  },
};

await runPokerAgent({
  memoryPolicy,
  decisionEngine: createStandardDecisionEngine({ sessionScope: "decision", memoryDirProvider: () => serverMemoryDir }),
  agentVersion: "%s/0.1.0",
  maxDecisionAttempts: parsePositiveInteger(process.env.PI_POKER_MAX_DECISION_ATTEMPTS),
});
`, key)

	if err := os.WriteFile(filepath.Join(agentDir, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		return fmt.Errorf("write package.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "tsconfig.json"), []byte(tsconfig), 0o644); err != nil {
		return fmt.Errorf("write tsconfig.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "main.ts"), []byte(mainTS), 0o644); err != nil {
		return fmt.Errorf("write src/main.ts: %w", err)
	}

	// Add to pi-agents/package.json workspaces.
	wsPkgPath := filepath.Join(piAgentsDir, "package.json")
	wsPkgData, err := os.ReadFile(wsPkgPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", wsPkgPath, err)
	}
	var wsPkg map[string]any
	if err := json.Unmarshal(wsPkgData, &wsPkg); err != nil {
		return fmt.Errorf("parse %s: %w", wsPkgPath, err)
	}
	workspaces, _ := wsPkg["workspaces"].([]any)
	workspaces = append(workspaces, key)
	wsPkg["workspaces"] = workspaces
	updated, err := json.MarshalIndent(wsPkg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", wsPkgPath, err)
	}
	if err := os.WriteFile(wsPkgPath, append(updated, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", wsPkgPath, err)
	}

	// Append to registry.json.
	registryPath := filepath.Join(piAgentsDir, "registry.json")
	entries = append(entries, registryEntry{Key: key, Type: "pi-agent"})
	regData, err := json.MarshalIndent(registryFile{Strategies: entries}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}
	if err := os.WriteFile(registryPath, append(regData, '\n'), 0o644); err != nil {
		return fmt.Errorf("write registry: %w", err)
	}

	_, _ = fmt.Fprintf(stdout, "created strategy %s\n", key)
	_, _ = fmt.Fprintf(stdout, "  files:  pi-agents/%s/src/main.ts\n", key)
	_, _ = fmt.Fprintf(stdout, "\nnext steps:\n")
	_, _ = fmt.Fprintf(stdout, "  1. implement pi-agents/%s/src/main.ts\n", key)
	_, _ = fmt.Fprintf(stdout, "  2. build:  cd pi-agents && npm install && npm run build\n")
	_, _ = fmt.Fprintf(stdout, "  3. smoke:  poker match run --agent0 %s --agent1 heuristic --hands 25\n", key)
	_, _ = fmt.Fprintf(stdout, "  4. test:   poker experiment new my-%s-experiment\n", key)
	return nil
}
