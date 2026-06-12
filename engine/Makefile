.PHONY: install setup agents test build

install:
	go install ./cmd/poker

# One-shot setup for a fresh checkout: install the poker CLI and build the
# Pi (LLM) agents. The agents also build automatically on first use, so this
# is optional — it just pre-warms the build and surfaces toolchain problems
# early. Requires Go and, for the agents, Node/npm.
setup: install agents
	@echo "Setup complete. Set ANTHROPIC_API_KEY, then try: poker demo"

agents:
	cd pi-agents && npm ci && npm run build

build:
	go build -o .tmp/bin/poker ./cmd/poker

test:
	go test ./...
