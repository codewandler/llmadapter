# Use-Case Matrix

This matrix answers a different question than `docs/PROVIDER_MATRIX.md`.

`PROVIDER_MATRIX.md` describes endpoint implementation evidence: what a provider endpoint can encode, decode, route, and smoke-test.

This document describes workload suitability: whether a specific model through a specific provider endpoint is suitable for a use case such as agentic coding.

The current implementation provides the compatibility vocabulary, evaluator, `adapterconfig` bridge, CLI inspection, library filtering helpers, and a live agentic-coding e2e matrix. The latest recorded live evidence is stored in `docs/compatibility/agentic_coding.json`.

The result table below is generated from that JSON artifact. Refresh it with:

```sh
go run ./cmd/llmadapter compatibility-record --use-case agentic_coding
```

## Use Cases

| Use case | Purpose |
| --- | --- |
| `agentic_coding` | Coding-agent runtime requiring tools, tool continuation, prompt caching, structured output, and usage accounting; reasoning is optional evidence for thinking-model filters. |
| `summarization` | Text generation or summarization where tools, reasoning, and prompt caching are optional. |

## Agentic Coding Requirements

| Feature | Requirement | Notes |
| --- | --- | --- |
| Streaming text | required | The client must receive incremental output. |
| Tools | required | The model/provider path must support tool calls. |
| Tool continuation | required | Tool results must be sendable back into the same API family. |
| Structured output | required | JSON mode/schema or tool schemas can carry structured data. |
| Reasoning | optional | Thinking/reasoning is recorded when observable and can be used by consumers that want reasoning-only model lists. |
| Prompt caching | required | llmadapter must be able to encode useful cache controls. |
| Usage | required | Usage events must be mapped when the provider reports usage. |
| Cache accounting | required | Provider-reported cache write/read counters are mandatory for agentic coding cost tracking. |
| Pricing | preferred | modeldb-backed pricing is preferred. |
| Gateway | optional | The mux/library path is enough for agentic coding; gateway coverage remains useful operator evidence. |

## Current CLI

Evaluate one model:

```sh
go run ./cmd/llmadapter compatibility --use-case agentic_coding --model anthropic/claude-haiku-4-5-20251001
```

Resolve and annotate candidates:

```sh
go run ./cmd/llmadapter resolve anthropic/claude-haiku-4-5-20251001 --use-case agentic_coding
```

Use JSON for consumers:

```sh
go run ./cmd/llmadapter compatibility --use-case agentic_coding --model anthropic/claude-haiku-4-5-20251001 --json
```

The CLI uses the same `adapterconfig` and modeldb-backed candidate resolution as `resolve`, `infer`, gateway, and mux construction. It does not perform a separate model lookup.

Strict approved-only selection is available through modeldb runtime views plus this live evidence artifact:

```sh
go run ./cmd/llmadapter resolve anthropic/claude-haiku-4-5-20251001 --use-case agentic_coding --approved-only
```

Library consumers can use `adapterconfig.SelectModelForUseCase` or `AutoResult.SelectModelForUseCase` with `LoadCompatibilityEvidence`. This fails closed unless a configured provider instance, API kind, and native model match an approved row.

The generated `Transport` column records the transport observed by the workload compatibility run. It is not a routing requirement unless the use case says so. Codex WebSocket continuation/cache behavior is tracked separately in `docs/PROVIDER_MATRIX.md` because it is a provider-internal optimization while the public Codex continuation contract remains replay.

`llmadapter conformance` now validates the approved `agentic_coding` rows as a strict contract. An approved row is only valid when all required workload checks are recorded as live evidence, cache accounting is live, and the artifact explicitly records consumer continuation, internal continuation, and transport. Reasoning remains recorded evidence, but non-thinking coding models can still be approved. This is the contract consumers such as agentsdk should trust when selecting models for coding-agent use.

## Initial Candidate Set

These rows are covered by the live agentic-coding compatibility smoke test:

| Public model | Provider endpoint candidates |
| --- | --- |
| `gpt-5.5` | `openai_responses`, `codex_responses`, `openrouter_responses` |
| `gpt-5.4` | `openai_responses`, `codex_responses`, `openrouter_responses` |
| `kimi-k2.6` | `openrouter_responses` |
| `glm-4.6` | `openrouter_responses` |
| `glm-4.7` | `openrouter_responses` |
| `qwen3-coder` | `openrouter_responses` |
| `qwen3-coder-next` | `openrouter_responses` |
| `deepseek-v3.2` | `openrouter_responses` |
| `haiku` | `claude`, `anthropic`, `openrouter_messages` |
| `sonnet` | `claude`, `anthropic`, `openrouter_messages` |
| `opus` | `claude`, `anthropic`, `openrouter_messages` |
| `bedrock-haiku` | `bedrock_converse` |
| `bedrock-sonnet-4-6` | `bedrock_converse` |
| `bedrock-opus-4-6` | `bedrock_converse` |
| `bedrock-opus-4-7` | `bedrock_converse` |
| `minimax-latest` | `minimax_messages` |

Short names in this generated evidence artifact are modeldb/catalog or test-harness public model names, not llmadapter-owned built-in aliases. Runtime docs prefer service-qualified names or explicit operator aliases.

## Latest Agentic-Coding Result

<!-- agentic-coding-result:start -->

Latest command:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestUseCaseAgenticCoding -count=1 -v
```

Total duration: 95.062 seconds.

| Candidate | Provider endpoint | Native model | Continuation | Transport | Required checks | Cache accounting | Status | Duration |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `anthropic_haiku` | `anthropic` | `claude-haiku-4-5-20251001` | replay | http_sse | pass | live | approved | 5.29s |
| `anthropic_opus` | `anthropic` | `claude-opus-4-6` | replay | http_sse | pass | live | approved | 10.26s |
| `anthropic_opus_4_7` | `anthropic` | `claude-opus-4-7` | replay | http_sse | pass | live | approved | 9.12s |
| `anthropic_sonnet` | `anthropic` | `claude-sonnet-4-6` | replay | http_sse | pass | live | approved | 7.98s |
| `bedrock_converse_haiku` | `bedrock_converse` | `anthropic.claude-haiku-4-5-20251001-v1:0` | replay | http_sse | pass | live | approved | 7.20s |
| `bedrock_converse_opus_4_6` | `bedrock_converse` | `anthropic.claude-opus-4-6-v1` | replay | http_sse | pass | live | approved | 12.97s |
| `bedrock_converse_opus_4_7` | `bedrock_converse` | `anthropic.claude-opus-4-7` | replay | http_sse | pass | live | approved | 9.08s |
| `bedrock_converse_sonnet_4_6` | `bedrock_converse` | `anthropic.claude-sonnet-4-6` | replay | http_sse | pass | live | approved | 8.14s |
| `claude_haiku` | `claude` | `claude-haiku-4-5-20251001` | replay | http_sse | pass | live | approved | 5.78s |
| `claude_opus` | `claude` | `claude-opus-4-6` | replay | http_sse | pass | live | approved | 14.60s |
| `claude_opus_4_7` | `claude` | `claude-opus-4-7` | replay | http_sse | pass | live | approved | 8.46s |
| `claude_sonnet` | `claude` | `claude-sonnet-4-6` | replay | http_sse | pass | live | approved | 16.83s |
| `codex_gpt_5_4` | `codex_responses` | `gpt-5.4` | replay | http_sse | pass | live | approved | 9.00s |
| `codex_gpt_5_5` | `codex_responses` | `gpt-5.5` | replay | http_sse | pass | live | approved | 13.29s |
| `minimax_latest` | `minimax_messages` | `MiniMax-M2.7` | replay | http_sse | pass | live | approved | 27.49s |
| `openai_gpt_5_4` | `openai_responses` | `gpt-5.4` | previous_response_id | http_sse | pass | live | approved | 8.49s |
| `openai_gpt_5_5` | `openai_responses` | `gpt-5.5` | previous_response_id | http_sse | pass | live | approved | 12.19s |
| `openrouter_deepseek_v3_2` | `openrouter_responses` | `deepseek/deepseek-v3.2` | replay | http_sse | pass | live | approved | 37.37s |
| `openrouter_glm_4_6` | `openrouter_responses` | `z-ai/glm-4.6` | replay | http_sse | pass | live | approved | 71.69s |
| `openrouter_glm_4_7` | `openrouter_responses` | `z-ai/glm-4.7` | replay | http_sse | pass | live | approved | 52.58s |
| `openrouter_gpt_5_4` | `openrouter_responses` | `openai/gpt-5.4` | replay | http_sse | pass | live | approved | 8.45s |
| `openrouter_gpt_5_5` | `openrouter_responses` | `openai/gpt-5.5` | replay | http_sse | pass | live | approved | 17.68s |
| `openrouter_haiku` | `openrouter_messages` | `anthropic/claude-haiku-4.5` | replay | http_sse | pass | live | approved | 6.48s |
| `openrouter_kimi_k2_6` | `openrouter_responses` | `moonshotai/kimi-k2.6` | replay | http_sse | pass | live | approved | 95.06s |
| `openrouter_opus` | `openrouter_messages` | `anthropic/claude-opus-4.6` | replay | http_sse | pass | live | approved | 13.01s |
| `openrouter_opus_4_7` | `openrouter_messages` | `anthropic/claude-opus-4.7` | replay | http_sse | pass | live | approved | 7.75s |
| `openrouter_qwen3_coder` | `openrouter_responses` | `qwen/qwen3-coder` | replay | http_sse | pass | live | approved | 6.09s |
| `openrouter_qwen3_coder_next` | `openrouter_responses` | `qwen/qwen3-coder-next` | replay | http_sse | pass | live | approved | 6.59s |
| `openrouter_sonnet` | `openrouter_messages` | `anthropic/claude-sonnet-4.6` | replay | http_sse | pass | live | approved | 29.28s |

<!-- agentic-coding-result:end -->

## Status Meaning

| Status | Meaning |
| --- | --- |
| `approved` | All required and preferred features have supporting evidence. |
| `degraded` | Required features pass, but at least one preferred feature is unavailable or untested. |
| `failed` | At least one required feature is unsupported. |
| `untested` | At least one required feature lacks evidence. |
| `unavailable` | The model/provider/API candidate cannot be resolved from the configured catalog/routes. |

## Current Result

The latest live run on 2026-05-03 passed all required agentic-coding checks for every row above:

```sh
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestUseCaseAgenticCoding -count=1 -v
```

Cache accounting is mandatory for agentic coding. Every approved row reported provider cache write or cache read counters in this run.

Reasoning is optional for agentic coding. Consumers that need a thinking-model-only list should filter the evidence table for `reasoning=live`.

The same artifact passes `llmadapter conformance`: every approved row is also a valid approved row, and no approved row is missing required feature, continuation, or transport evidence.

OpenRouter documentation says prompt caching can report `cached_tokens` and `cache_write_tokens` in detailed usage. The adapter now decodes both Responses-style `input_tokens_details` and Chat/Completions-style `prompt_tokens_details`, which is required because OpenRouter can expose the latter shape on Responses-compatible streams.

Kimi uses OpenRouter model `moonshotai/kimi-k2.6`. GLM, Qwen, and DeepSeek rows use OpenRouter Responses models `z-ai/glm-4.6`, `z-ai/glm-4.7`, `qwen/qwen3-coder`, `qwen/qwen3-coder-next`, and `deepseek/deepseek-v3.2`. Sonnet and Opus rows use catalog/test-harness public model names that resolve to `claude-sonnet-4-6` and `claude-opus-4-6`; these are not llmadapter-owned built-in aliases.
