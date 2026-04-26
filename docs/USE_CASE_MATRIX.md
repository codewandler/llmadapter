# Use-Case Matrix

This matrix answers a different question than `docs/PROVIDER_MATRIX.md`.

`PROVIDER_MATRIX.md` describes endpoint implementation evidence: what a provider endpoint can encode, decode, route, and smoke-test.

This document describes workload suitability: whether a specific model through a specific provider endpoint is suitable for a use case such as agentic coding.

The current implementation provides the compatibility vocabulary, evaluator, `adapterconfig` bridge, CLI inspection, library filtering helpers, and a live agentic-coding e2e matrix. The latest recorded live evidence is stored in `docs/compatibility/agentic_coding.json`.

## Use Cases

| Use case | Purpose |
| --- | --- |
| `agentic_coding` | Coding-agent runtime requiring tools, tool continuation, reasoning, prompt caching, structured output, and usage accounting. |
| `summarization` | Text generation or summarization where tools, reasoning, and prompt caching are optional. |

## Agentic Coding Requirements

| Feature | Requirement | Notes |
| --- | --- | --- |
| Streaming text | required | The client must receive incremental output. |
| Tools | required | The model/provider path must support tool calls. |
| Tool continuation | required | Tool results must be sendable back into the same API family. |
| Structured output | required | JSON mode/schema or tool schemas can carry structured data. |
| Reasoning | required | Thinking/reasoning must be requestable and observable where the provider exposes it. |
| Prompt caching | required | llmadapter must be able to encode useful cache controls. |
| Usage | required | Usage events must be mapped when the provider reports usage. |
| Cache accounting | preferred | Provider-reported cache write/read counters are preferred but not required for first-pass approval. |
| Pricing | preferred | modeldb-backed pricing is preferred. |
| Gateway | optional | The mux/library path is enough for agentic coding; gateway coverage remains useful operator evidence. |

## Current CLI

Evaluate one model:

```sh
go run ./cmd/llmadapter compatibility --use-case agentic_coding --model haiku
```

Resolve and annotate candidates:

```sh
go run ./cmd/llmadapter resolve haiku --use-case agentic_coding
```

Use JSON for consumers:

```sh
go run ./cmd/llmadapter compatibility --use-case agentic_coding --model haiku --json
```

The CLI uses the same `adapterconfig` and modeldb-backed candidate resolution as `resolve`, `infer`, gateway, and mux construction. It does not perform a separate model lookup.

## Initial Candidate Set

These rows are covered by the live agentic-coding compatibility smoke test:

| Public model | Provider endpoint candidates |
| --- | --- |
| `gpt-5.5` | `openai_responses`, `codex_responses`, `openrouter_responses` |
| `gpt-5.4` | `openai_responses`, `codex_responses`, `openrouter_responses` |
| `haiku` | `claude`, `anthropic`, `openrouter_messages` |
| `sonnet` | `claude`, `anthropic`, `openrouter_messages` |
| `opus` | `claude`, `anthropic`, `openrouter_messages` |
| `minimax-latest` | `minimax_messages` |

Bedrock is intentionally excluded until a Bedrock provider endpoint exists.

## Status Meaning

| Status | Meaning |
| --- | --- |
| `approved` | All required and preferred features have supporting evidence. |
| `degraded` | Required features pass, but at least one preferred feature is unavailable or untested. |
| `failed` | At least one required feature is unsupported. |
| `untested` | At least one required feature lacks evidence. |
| `unavailable` | The model/provider/API candidate cannot be resolved from the configured catalog/routes. |

## Current Limitation

The latest live run on 2026-04-26 passed all required agentic-coding checks for every row above:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestUseCaseAgenticCoding -count=1 -v
```

Cache accounting remains provider-dependent. Claude, Anthropic, and Codex rows reported live cache write/read evidence. OpenAI Responses, OpenRouter, and MiniMax rows passed prompt-cache control checks but did not report cache write/read counters in this test, so they are degraded rather than fully approved when `cache_accounting` is treated as preferred.

Sonnet and Opus rows use the current repository/modeldb aliases `claude-sonnet-4-6` and `claude-opus-4-6`.
