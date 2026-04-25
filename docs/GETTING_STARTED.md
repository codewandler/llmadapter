# Getting Started

This guide gets you from a fresh checkout to a working CLI inference call, config inspection, gateway, and Docker run.

## 1. Verify The Repo

```sh
env GOCACHE=/tmp/go-cache go test ./...
env GOCACHE=/tmp/go-cache go vet ./...
env GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go build ./...
```

These checks do not require provider credentials.

## 2. Add Credentials

Set at least one provider credential:

```sh
export ANTHROPIC_API_KEY=...
export OPENAI_API_KEY=...
export OPENROUTER_API_KEY=...
export MINIMAX_API_KEY=...
```

Aliases also supported:

```sh
export OPENAI_KEY=...
export OPENROUTER_KEY=...
export MINIMAX_KEY=...
```

Local OAuth providers:

- Claude Code-compatible access uses `~/.claude/.credentials.json`; set `CLAUDE_CONFIG_DIR` to override the directory.
- Codex/ChatGPT-compatible access uses `~/.codex/auth.json`; set `CODEX_AUTH_PATH` to override the file.

Inspect what llmadapter sees:

```sh
go run ./cmd/llmadapter providers --auto
```

The output reports provider endpoint status without printing secrets.

## 3. Run CLI Inference

Use the auto-detected mux client:

```sh
go run ./cmd/llmadapter infer -m haiku "reply with one short sentence"
```

The command prints route/model resolution first, streams reasoning/text when available, then prints usage and cost data when providers report it.

Resolve without calling a provider:

```sh
go run ./cmd/llmadapter resolve haiku
go run ./cmd/llmadapter resolve openai/gpt-5.5
```

## 4. Inspect The Example Config

The repository ships a load-tested config:

```sh
go run ./cmd/llmadapter serve --config examples/llmadapter.example.json --inspect-config
```

This validates routing, provider endpoint metadata, modeldb overlays, aliases, capability provenance, and pricing availability without constructing live provider clients.

Useful inspection commands:

```sh
go run ./cmd/llmadapter routes --config examples/llmadapter.example.json
go run ./cmd/llmadapter models --config examples/llmadapter.example.json
go run ./cmd/llmadapter resolve --config examples/llmadapter.example.json fast
```

## 5. Run The Gateway

Start the gateway:

```sh
go run ./cmd/llmadapter serve --config examples/llmadapter.example.json
```

Call the OpenAI Responses-compatible endpoint:

```sh
curl -sS http://localhost:8080/v1/responses \
  -H 'content-type: application/json' \
  -d '{
    "model": "fast",
    "input": "Reply with exactly: llmadapter ok"
  }'
```

The same gateway also exposes:

- `/v1/chat/completions`
- `/v1/responses`
- `/v1/messages`

## 6. Run In Docker

Build:

```sh
docker build -t llmadapter:local .
```

Run with the example config:

```sh
docker run --rm -p 8080:8080 -w /app \
  -v "$PWD/examples:/app/examples:ro" \
  -e ANTHROPIC_API_KEY -e OPENAI_API_KEY -e OPENROUTER_API_KEY \
  llmadapter:local serve --config examples/llmadapter.example.json
```

Run with your own config:

```sh
docker run --rm -p 8080:8080 \
  -v "$PWD/llmadapter.json:/etc/llmadapter/config.json:ro" \
  -e LLMADAPTER_CONFIG=/etc/llmadapter/config.json \
  -e ANTHROPIC_API_KEY \
  llmadapter:local
```

## 7. Run Live Smoke Tests

Run the full live matrix:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -count=1 -v
```

Run focused slices:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeTextStream -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeToolUse -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestGatewaySmoke -count=1 -v
```

Tests skip cleanly when credentials are missing or a provider endpoint does not advertise a feature.

## Next Steps

- Read [CLI.md](CLI.md) for command details.
- Read [CONFIGURATION.md](CONFIGURATION.md) for routing and modeldb config.
- Read [LIBRARY_USAGE.md](LIBRARY_USAGE.md) for Go client patterns.
- Read [PROVIDER_MATRIX.md](PROVIDER_MATRIX.md) before relying on provider-specific features.
