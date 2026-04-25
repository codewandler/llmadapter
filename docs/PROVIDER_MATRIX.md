# Provider Matrix

This matrix is the v1 supported provider endpoint surface. It describes what llmadapter routes and smoke-tests today; it is not a promise that every upstream provider-specific field is implemented.

Legend:

- `live`: covered by `TEST_INTEGRATION=1` smoke tests when credentials are available.
- `fixture`: covered by deterministic offline conformance fixtures.
- `mapped`: encoded/decoded by provider or endpoint mapping, but not asserted by a live provider accounting check.
- `n/a`: not part of that provider endpoint family.

## Endpoints

| Provider endpoint | API kind | Family | Credentials | Default smoke model |
| --- | --- | --- | --- | --- |
| `anthropic` | `anthropic.messages` | `anthropic.messages` | `ANTHROPIC_API_KEY` | `claude-haiku-4-5-20251001` |
| `claude` | `anthropic.messages` | `anthropic.messages` | `~/.claude/.credentials.json` or `CLAUDE_CONFIG_DIR` | `claude-haiku-4-5-20251001` |
| `openai_chat` | `openai.chat_completions` | `openai.chat_completions` | `OPENAI_API_KEY` or `OPENAI_KEY` | `gpt-4.1-mini` |
| `openai_responses` | `openai.responses` | `openai.responses` | `OPENAI_API_KEY` or `OPENAI_KEY` | `gpt-4.1-mini` |
| `codex_responses` | `codex.responses` | `openai.responses` | `CODEX_ACCESS_TOKEN`, `CODEX_CODE_OAUTH_TOKEN`, or `~/.codex/auth.json` | provider default |
| `openrouter_chat` | `openrouter.chat_completions` | `openai.chat_completions` | `OPENROUTER_API_KEY` or `OPENROUTER_KEY` | `openai/gpt-4.1-mini` |
| `openrouter_responses` | `openrouter.responses` | `openai.responses` | `OPENROUTER_API_KEY` or `OPENROUTER_KEY` | `openai/gpt-4.1-mini` |
| `openrouter_messages` | `openrouter.anthropic_messages` | `anthropic.messages` | `OPENROUTER_API_KEY` or `OPENROUTER_KEY` | `anthropic/claude-sonnet-4` |
| `minimax_chat` | `minimax.chat_completions` | `openai.chat_completions` | `MINIMAX_API_KEY` or `MINIMAX_KEY` | `MiniMax-M2.7` |
| `minimax_messages` | `minimax.anthropic_messages` | `anthropic.messages` | `MINIMAX_API_KEY` or `MINIMAX_KEY` | `MiniMax-M2.7` |

## Feature Coverage

| Provider endpoint | Text | Tools | Tool continuation | Parallel tools | Reasoning | Prompt cache accounting | Structured output | Vision | Usage | Pricing | Gateway |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `anthropic` | live | live | live | n/a | live | live | n/a | fixture | live | modeldb | live |
| `claude` | live | live | live | n/a | live | live | n/a | fixture | live | modeldb | live |
| `openai_chat` | live | live | live | live | n/a | n/a | fixture | fixture | live | modeldb | live |
| `openai_responses` | live | live | live | live | fixture | mapped | fixture | fixture | live | modeldb | live |
| `codex_responses` | live | live | live | live | live | live | fixture | fixture | live | modeldb | live |
| `openrouter_chat` | live | live | live | live | n/a | n/a | fixture | fixture | live | modeldb | live |
| `openrouter_responses` | live | live | live | live | fixture | mapped | fixture | fixture | live | modeldb | live |
| `openrouter_messages` | live | live | live | n/a | live | mapped | n/a | fixture | live | modeldb | live |
| `minimax_chat` | live | live | live | n/a | n/a | n/a | fixture | fixture | live | modeldb | live |
| `minimax_messages` | live | live | live | n/a | live | mapped | n/a | fixture | live | modeldb | live |

Prompt cache accounting means the live smoke test checks provider-reported cache write/read token counters. `mapped` means llmadapter maps the cache controls onto the provider wire shape, but the v1 smoke matrix does not assert provider-reported cache accounting for that endpoint.

## Live Smoke Commands

Full available matrix:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -count=1 -v
```

Focused slices:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeTextStream -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeToolUse -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeToolResultContinuation -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeReasoningStream -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokePromptCache -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestGatewaySmoke -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestAnthropicMessagesGatewaySmoke -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestResponsesGatewaySmoke -count=1 -v
```

The e2e package skips cleanly when `TEST_INTEGRATION` is unset. Individual provider subtests skip when their credential env vars or local OAuth files are unavailable, or when a feature is not advertised by that endpoint in the smoke matrix.

## Latest V1 Track Result

On 2026-04-25, the full live command above was run with local credentials available for all v1 provider endpoints. Text, tools, tool continuation, gateway routing, reasoning where advertised, parallel tools where advertised, Responses continuation, invalid credentials, and invalid model normalization passed. Prompt-cache accounting passed for `anthropic`, `claude`, and `codex_responses`; MiniMax Messages cache controls remain mapped but are not marked live accounting-verified because the provider response did not report cache write/read counters in this run.
