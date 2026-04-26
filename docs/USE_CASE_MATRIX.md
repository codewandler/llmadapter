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
| Cache accounting | required | Provider-reported cache write/read counters are mandatory for agentic coding cost tracking. |
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
| `kimi-k2.6` | `openrouter_responses` |
| `haiku` | `claude`, `anthropic`, `openrouter_messages` |
| `sonnet` | `claude`, `anthropic`, `openrouter_messages` |
| `opus` | `claude`, `anthropic`, `openrouter_messages` |
| `minimax-latest` | `minimax_messages` |

Bedrock is intentionally excluded until a Bedrock provider endpoint exists.

## Latest Agentic-Coding Result

Latest command:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestUseCaseAgenticCoding -count=1 -v
```

Total duration: 232.313 seconds.

| Candidate | Provider endpoint | Native model | Required checks | Cache accounting | Status | Duration |
| --- | --- | --- | --- | --- | --- | --- |
| `openai_gpt_5_5` | `openai_responses` | `gpt-5.5` | pass | live | approved | 10.42s |
| `codex_gpt_5_5` | `codex_responses` | `gpt-5.5` | pass | live | approved | 9.49s |
| `openrouter_gpt_5_5` | `openrouter_responses` | `openai/gpt-5.5` | pass | live | approved | 10.85s |
| `openai_gpt_5_4` | `openai_responses` | `gpt-5.4` | pass | live | approved | 8.13s |
| `codex_gpt_5_4` | `codex_responses` | `gpt-5.4` | pass | live | approved | 7.96s |
| `openrouter_gpt_5_4` | `openrouter_responses` | `openai/gpt-5.4` | pass | live | approved | 7.22s |
| `openrouter_kimi_k2_6` | `openrouter_responses` | `moonshotai/kimi-k2.6` | pass | live | approved | 58.22s |
| `claude_haiku` | `claude` | `claude-haiku-4-5-20251001` | pass | live | approved | 8.35s |
| `anthropic_haiku` | `anthropic` | `claude-haiku-4-5-20251001` | pass | live | approved | 4.42s |
| `openrouter_haiku` | `openrouter_messages` | `anthropic/claude-haiku-4.5` | pass | live | approved | 7.74s |
| `claude_sonnet` | `claude` | `claude-sonnet-4-6` | pass | live | approved | 8.29s |
| `anthropic_sonnet` | `anthropic` | `claude-sonnet-4-6` | pass | live | approved | 7.16s |
| `openrouter_sonnet` | `openrouter_messages` | `anthropic/claude-sonnet-4.6` | pass | live | approved | 10.31s |
| `claude_opus` | `claude` | `claude-opus-4-6` | pass | live | approved | 12.47s |
| `anthropic_opus` | `anthropic` | `claude-opus-4-6` | pass | live | approved | 16.55s |
| `openrouter_opus` | `openrouter_messages` | `anthropic/claude-opus-4.6` | pass | live | approved | 13.53s |
| `minimax_latest` | `minimax_messages` | `MiniMax-M2.7` | pass | live | approved | 31.17s |

## Status Meaning

| Status | Meaning |
| --- | --- |
| `approved` | All required and preferred features have supporting evidence. |
| `degraded` | Required features pass, but at least one preferred feature is unavailable or untested. |
| `failed` | At least one required feature is unsupported. |
| `untested` | At least one required feature lacks evidence. |
| `unavailable` | The model/provider/API candidate cannot be resolved from the configured catalog/routes. |

## Current Result

The latest live run on 2026-04-26 passed all required agentic-coding checks for every row above:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestUseCaseAgenticCoding -count=1 -v
```

Cache accounting is mandatory for agentic coding. Every approved row reported provider cache write or cache read counters in this run.

OpenRouter documentation says prompt caching can report `cached_tokens` and `cache_write_tokens` in detailed usage. The adapter now decodes both Responses-style `input_tokens_details` and Chat/Completions-style `prompt_tokens_details`, which is required because OpenRouter can expose the latter shape on Responses-compatible streams.

Kimi uses OpenRouter model `moonshotai/kimi-k2.6`. Sonnet and Opus rows use the current repository/modeldb aliases `claude-sonnet-4-6` and `claude-opus-4-6`.
