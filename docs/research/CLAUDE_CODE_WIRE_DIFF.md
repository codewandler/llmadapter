# Claude Code Wire Diff

This document records the observed wire-level differences between the current llmadapter Claude-compatible provider path and Claude Code traffic.

Scope:

- Claude Code `2.1.112`, observed through `llmadapter proxy --analyze claude`.
- Anthropic Messages API over `https://api.anthropic.com/v1/messages?beta=true`.
- Low-noise Claude CLI runs using `--bare`, `--tools ""`, `--system-prompt ""`, `--no-session-persistence`, `--disable-slash-commands`, `--strict-mcp-config`, and an empty MCP config.
- Current Anthropic model availability from `/v1/models` in this environment on 2026-05-02.

This is not a permanent model capability table. Treat it as a point-in-time parity report and prefer capability metadata over model-name guessing when implementing behavior.

## Terminology

`anthropic` means direct Anthropic API access using Anthropic API keys and documented API semantics.

`claude` means Claude Code-compatible access through Anthropic's Claude Code CLI behavior, including Claude Code headers, local Claude OAuth where configured, Claude-specific beta headers, request preflight metadata, and Claude Code's model/effort/context-management decisions.

Similarly, `openai` means direct OpenAI API access, while `codex` means Codex/ChatGPT-compatible access. Codex may use ChatGPT account headers, turn-state headers, WebSocket session affinity, and internal continuation behavior that direct OpenAI API clients do not use.

Do not merge these pairs into one provider behavior model. They share API families, but their authentication, headers, model aliases, continuation semantics, quota telemetry, and provider-specific metadata differ.

## Current llmadapter Claude Path

Relevant implementation areas:

- `providers/anthropic/messages/claude.go`
- `providers/anthropic/messages/codec.go`
- `anthropicwire/wire.go`
- `providers/anthropic/messages/decoder.go`

Current behavior:

- Sends Claude Code-compatible user agent and Stainless headers, but with older pinned values.
- Adds `beta=true`.
- Sends a fixed `Anthropic-Beta` string.
- Injects a billing system block and Claude agent SDK system block.
- Adds `metadata.user_id` from local Claude config where available.
- Supports Anthropic extended thinking through manual `thinking:{type:"enabled", budget_tokens:N}`.
- Maps canonical reasoning effort to manual thinking budgets.
- Adds prompt-cache `cache_control` hints on system, last message block, and tools when cache policy requests it.
- Parses text, tool-use, thinking, signature, finish, usage, ping, and API error stream events.
- Preserves raw provider usage JSON for usage events.
- Does not currently model Claude Code's newer `output_config.effort`, `thinking:{type:"adaptive"}`, or `context_management` request/response fields.

## Observed Claude Code Request Skeleton

Even in low-noise `--bare` mode, Claude Code sends more than the raw user prompt.

Startup:

- `HEAD /` against the configured base URL.
- A concurrent title-generation request using Haiku 4.5, structured output, and no user-facing tools.
- The main Messages request.

Main request shape:

```json
{
  "model": "claude-sonnet-4-6",
  "max_tokens": 32000,
  "stream": true,
  "system": [
    {
      "type": "text",
      "text": "x-anthropic-billing-header: ..."
    },
    {
      "type": "text",
      "text": "You are a Claude agent, built on Anthropic's Claude Agent SDK.",
      "cache_control": {
        "type": "ephemeral"
      }
    }
  ],
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "<system-reminder>...current date...</system-reminder>"
        },
        {
          "type": "text",
          "text": "user prompt",
          "cache_control": {
            "type": "ephemeral"
          }
        }
      ]
    }
  ],
  "metadata": {
    "user_id": "[redacted]"
  },
  "context_management": {
    "edits": [
      {
        "type": "clear_thinking_20251015",
        "keep": "all"
      }
    ]
  },
  "thinking": {
    "type": "adaptive"
  },
  "output_config": {
    "effort": "high"
  },
  "tools": []
}
```

Claude Code also sends `X-Claude-Code-Session-Id` on POST requests. llmadapter currently generates an internal session ID for metadata construction but does not send this header.

## Header Diff

Observed Claude Code `2.1.112` main request headers:

```text
Anthropic-Beta: claude-code-20250219,interleaved-thinking-2025-05-14,context-management-2025-06-27,prompt-caching-scope-2026-01-05,advisor-tool-2026-03-01,effort-2025-11-24
Anthropic-Dangerous-Direct-Browser-Access: true
Anthropic-Version: 2023-06-01
User-Agent: claude-cli/2.1.112 (external, sdk-cli)
X-App: cli
X-Claude-Code-Session-Id: [redacted]
X-Stainless-Arch: x64
X-Stainless-Lang: js
X-Stainless-Os: Linux
X-Stainless-Package-Version: 0.81.0
X-Stainless-Retry-Count: 0
X-Stainless-Runtime: node
X-Stainless-Runtime-Version: v24.3.0
X-Stainless-Timeout: 600
```

Current llmadapter Claude-compatible constants are older:

```text
User-Agent: claude-cli/2.1.85 (external, sdk-cli)
X-Stainless-Package-Version: 0.74.0
Anthropic-Beta: claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14,context-management-2025-06-27,prompt-caching-scope-2026-01-05,effort-2025-11-24
```

Notable differences:

- Claude Code includes `advisor-tool-2026-03-01`.
- Claude Code includes `oauth-2025-04-20` only on OAuth/subscription auth paths, not API-key paths.
- Claude Code includes `effort-2025-11-24` for effort-capable request shapes, but not for every model.
- Claude Code includes `X-Claude-Code-Session-Id`.
- Claude Code versions and Stainless package versions have moved.

Implementation implication: beta/header selection should be request/model/auth-aware instead of one fixed string for every Claude-compatible request.

## Effort And Thinking Matrix

Live traces show that Claude Code does not simply map the CLI `--effort` flag to the request body. Behavior is model-gated.

| Claude CLI input | Resolved model | Request shape |
| --- | --- | --- |
| `--model haiku --effort medium` | `claude-haiku-4-5-20251001` | No `output_config.effort`; no effort beta; legacy `thinking:{type:"enabled", budget_tokens:31999}` |
| `--model claude-sonnet-4-6` | `claude-sonnet-4-6` | `thinking:{type:"adaptive"}`, `output_config.effort:"high"`, effort beta |
| `--model claude-sonnet-4-6 --effort low` | `claude-sonnet-4-6` | `thinking:{type:"adaptive"}`, `output_config.effort:"low"`, effort beta |
| `--model claude-sonnet-4-6 --effort max` | `claude-sonnet-4-6` | `thinking:{type:"adaptive"}`, `output_config.effort:"max"`, effort beta |
| `--model claude-opus-4-6` | `claude-opus-4-6` | `thinking:{type:"adaptive"}`, `output_config.effort:"high"`, effort beta |
| `--model opus` | `claude-opus-4-7` | `thinking:{type:"adaptive"}`, `output_config.effort:"xhigh"`, effort beta |
| `--model opus --effort low` | `claude-opus-4-7` | `thinking:{type:"adaptive"}`, `output_config.effort:"low"`, effort beta |
| `--model opus --effort max` | `claude-opus-4-7` | `thinking:{type:"adaptive"}`, `output_config.effort:"max"`, effort beta |

Current Anthropic `/v1/models` availability in this environment:

```text
claude-opus-4-7
claude-sonnet-4-6
claude-opus-4-6
claude-haiku-4-5-20251001
```

`claude-sonnet-4-7` is not available in this environment. When forced with `--model claude-sonnet-4-7`, Claude Code built a request but treated the model as unknown/legacy:

```json
{
  "model": "claude-sonnet-4-7",
  "thinking": {
    "type": "enabled",
    "budget_tokens": 31999
  }
}
```

That request returned `404 not_found_error`.

Implementation implications:

- Do not infer effort support from `sonnet-4-*` or `opus-4-*` string patterns alone.
- Use modeldb exposure metadata or explicit operator overrides.
- `ReasoningEffortMax` remains the canonical llmadapter value. Modeldb exposure `parameter_value_mappings` map canonical `max` to Anthropic's Opus 4.7 wire value `xhigh` where required.
- Preserve the old manual-budget path for models that do not support `output_config.effort`.
- llmadapter now uses adaptive thinking plus `output_config.effort` when the routed modeldb exposure supports adaptive thinking and the requested effort, and callers set effort without an explicit manual budget.

Official Anthropic docs currently describe effort as supported for Mythos Preview, Opus 4.7, Opus 4.6, Sonnet 4.6, and Opus 4.5. They also say manual `budget_tokens` is deprecated for Opus 4.6 and Sonnet 4.6 and unsupported for Opus 4.7. See `https://platform.claude.com/docs/en/build-with-claude/effort`.

## Context Management

Claude Code currently sends:

```json
{
  "context_management": {
    "edits": [
      {
        "type": "clear_thinking_20251015",
        "keep": "all"
      }
    ]
  }
}
```

Observed response stream `message_delta` includes:

```json
{
  "type": "message_delta",
  "usage": {
    "iterations": [
      {
        "type": "message",
        "input_tokens": 111,
        "output_tokens": 8,
        "cache_read_input_tokens": 0,
        "cache_creation_input_tokens": 0
      }
    ]
  },
  "context_management": {
    "applied_edits": []
  }
}
```

Anthropic docs describe context editing as automatic context-window management by clearing stale tool calls/results or thinking blocks. Documented edit types include:

- `clear_tool_uses_20250919`
- `clear_thinking_20251015`

The docs also describe separate compaction support through beta `compact-2026-01-12` and edit type `compact_20260112`, which can emit `compaction` blocks and `compaction_delta` stream events. This was not observed in the Claude Code traces in this session.

Sources:

- `https://platform.claude.com/docs/en/build-with-claude/context-editing`
- `https://platform.claude.com/docs/en/build-with-claude/compaction`
- `https://platform.claude.com/docs/en/build-with-claude/context-windows`

Implementation implications:

- Request support for `context_management` now exists through `unified.AnthropicExtensions.ContextManagement`.
- Claude-compatible preflight now adds the observed `clear_thinking_20251015` edit by default only when the encoded request has thinking enabled/adaptive and the request did not already supply context management. Anthropic rejects this edit when thinking is absent.
- Response wire structs now preserve `message_delta.context_management`, including `applied_edits`, in provider raw data.
- Unknown context-management edit/event shapes should continue to be preserved as raw provider data.
- Do not implement automatic client-side compaction in llmadapter's stateless provider path. That belongs above llmadapter, but llmadapter should expose upstream context-management telemetry so consumers can decide what happened.

## Advisor Tool

`advisor-tool-2026-03-01` is an Anthropic beta for a server-side Advisor Tool. It allows the model handling the request to consult a stronger advisor model inside the same Messages request.

Official request shape uses a server tool definition like:

```json
{
  "type": "advisor_20260301",
  "name": "advisor",
  "model": "claude-opus-4-7"
}
```

Observed in Claude Code:

- The beta header is present in current main and title-generation requests.
- The advisor tool definition was not present in the low-noise traces.
- Local Claude Code binary strings contain `/advisor` command wiring, `advisorModel`, feature gates, `server_tool_use` handling, and `advisor_tool_result` handling.

Source:

- `https://platform.claude.com/docs/en/agents-and-tools/tool-use/advisor-tool`

Implementation implications:

- llmadapter should not silently encode canonical function tools as advisor tools. Advisor is a provider-specific server tool with different semantics and cost.
- If exposed, it should be represented as a provider-specific Anthropic/Claude extension or a future canonical built-in/server-tool abstraction.
- The Anthropic stream decoder now preserves unknown content-block deltas as raw events and already preserves unknown content-block starts. Full canonical advisor support is still not implemented.
- Usage/cost for advisor calls may appear in `usage.iterations[]`; llmadapter now preserves `iterations[]` in provider raw usage, but does not yet project advisor usage into a separate canonical cost category.

## Stream And Usage Diff

Observed stream fields now represented by current llmadapter wire structs:

- `message_start.message.stop_details`
- `message_start.message.usage.cache_creation.ephemeral_5m_input_tokens`
- `message_start.message.usage.cache_creation.ephemeral_1h_input_tokens`
- `message_start.message.usage.service_tier`
- `message_start.message.usage.inference_geo`
- `message_delta.delta.stop_details`
- `message_delta.usage.iterations[]`
- `message_delta.context_management.applied_edits`

Observed or documented content/event shapes that should be preserved:

- `thinking`
- `signature_delta`
- `server_tool_use`
- `advisor_tool_result`
- `compaction`
- `compaction_delta`

Current llmadapter handles `thinking` and `signature_delta`, preserves unknown SSE events as `RawEvent`, and preserves unknown content-block deltas as `RawEvent`. Full canonical models for advisor and compaction events are still intentionally deferred.

Remaining implementation implications:

- Preserve complete raw JSON for unknown content block starts if future event types need fields not currently represented by `ContentBlock`.
- Add canonical event types only after the provider semantics are stable enough.

## Exact Missing Pieces

Compatibility-critical for current Claude Code parity:

- Still missing: canonical advisor tool request/response abstraction.
- Still missing: canonical compaction event abstraction.
- Partially implemented: raw preservation for unknown/new Anthropic stream events.
- Implemented: `X-Claude-Code-Session-Id` header.
- Implemented: current Claude Code header versions.
- Implemented: auth-aware beta header composition.
- Implemented: `advisor-tool-2026-03-01` beta in Claude-compatible request paths.
- Implemented: modeldb metadata-aware effort/adaptive-thinking encoding for routed effort-capable models.
- Implemented: exact Opus 4.7 canonical `max` to wire `xhigh` effort mapping via modeldb exposure metadata.
- Implemented: `output_config.effort`.
- Implemented: `thinking:{type:"adaptive"}`.
- Implemented: `context_management.edits` through extension passthrough and a Claude preflight default for thinking requests.
- Implemented: `message_delta.context_management` wire preservation.
- Implemented: nested usage wire fields, including `iterations[]`.

Important but not necessarily required for stateless llmadapter:

- Claude Code title-generation request emulation.
- Claude Code `HEAD /` connectivity check.
- Claude Code current-date system reminder.
- Claude Code LSP/MCP/plugin/tool loading behavior outside `--bare`.
- Advisor tool enablement and prompting policy.
- Server-side compaction request generation.

Non-goals for llmadapter provider paths:

- Stateful conversation compaction policy.
- Automatic history summarization.
- Tool-result retention policy.
- User-session UI features from Claude Code.

Those belong above llmadapter, but llmadapter should avoid blocking them by exposing upstream telemetry and preserving provider-specific raw events.

## Recommended Implementation Order

1. Complete raw JSON preservation for unknown content block start shapes where the current `ContentBlock` struct would drop provider-specific fields.
2. Keep Anthropic effort support backed by modeldb exposure metadata rather than model-name allowlists.
3. Add model/capability-aware reasoning mapping refinements:
   - effort-capable models use adaptive thinking plus `output_config.effort`;
   - legacy models use manual thinking budgets;
   - unknown models do not guess effort support from family strings.
4. Add focused decoder fixtures for advisor and compaction stream shapes using sanitized captures.
5. Only then consider opt-in advisor/context-compaction canonical request abstractions.

## Research Commands

Representative sanitized commands used for live tracing:

```sh
env GOCACHE=/tmp/go-cache go run ./cmd/llmadapter proxy --max-body-bytes 12000 --analyze claude -- --bare --print --verbose --output-format stream-json --include-partial-messages --model claude-sonnet-4-6 --tools "" --system-prompt "" --no-session-persistence --disable-slash-commands --strict-mcp-config --mcp-config='{"mcpServers":{}}' "Reply with exactly: sonnet46 ok"
```

```sh
env GOCACHE=/tmp/go-cache go run ./cmd/llmadapter proxy --max-body-bytes 12000 --analyze claude -- --bare --print --verbose --output-format stream-json --include-partial-messages --model opus --tools "" --system-prompt "" --no-session-persistence --disable-slash-commands --strict-mcp-config --mcp-config='{"mcpServers":{}}' --effort max "Reply with exactly: opus max ok"
```

```sh
curl -sS https://api.anthropic.com/v1/models -H "x-api-key: $ANTHROPIC_API_KEY" -H "anthropic-version: 2023-06-01"
```

All sensitive header values, tokens, account identifiers, cookies, request bodies containing private local context, and signatures must be redacted before committing any trace fixture.
