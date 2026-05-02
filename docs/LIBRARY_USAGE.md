# Library Usage

Use `llmadapter` as a Go library when you want a stateless `unified.Client` over one or more provider endpoints.

## Core Client Interface

All provider clients and mux clients implement:

```go
type Client interface {
	Request(ctx context.Context, req unified.Request) (<-chan unified.Event, error)
}
```

Most consumers send a `unified.Request` and collect the event stream:

```go
resp, err := unified.Collect(ctx, events)
```

Advanced consumers can process events incrementally for streaming UI, usage accounting, reasoning display, tool loops, or raw provider metadata.

## Auto-Detected Mux Client

`adapterconfig.AutoMuxClient` detects env/local credentials and builds a mux client:

```go
result, err := adapterconfig.AutoMuxClient(adapterconfig.AutoOptions{
	UseModelDB:    true,
	DynamicModels: true,
	Intents: []adapterconfig.AutoIntent{
		{Name: "haiku"},
		{Name: "openai/gpt-5.5"},
	},
})
if err != nil {
	return err
}

client := result.Client
```

Useful options:

- `EnableEnv`: detect API-key env vars.
- `EnableLocalClaude`: detect local Claude Code OAuth credentials.
- `EnableLocalCodex`: detect local Codex/ChatGPT OAuth credentials.
- `UseModelDB`: use modeldb catalog/aliases for model resolution.
- `DynamicModels`: add dynamic model routes for enabled providers.
- `SourceAPI`: pin an incoming API kind.
- `Intents`: add routes for specific model names.
- `ModelDBAliases`: inject or override aliases from the host application.

## Workload Compatibility

Use `adapterconfig` plus `compatibility` when a host application needs to list candidates suitable for a workload such as agentic coding:

```go
evaluations, err := adapterconfig.EvaluateCompatibilityCandidates(
	result.Config,
	"haiku",
	"",
	compatibility.UseCaseAgenticCoding,
)
if err != nil {
	return err
}
```

To return candidates whose offline capability evidence satisfies the profile:

```go
candidates, err := adapterconfig.CompatibleCandidates(
	result.Config,
	"haiku",
	"",
	compatibility.UseCaseAgenticCoding,
	true,
)
```

This API consumes the same modeldb-backed route candidates used by the mux client and gateway. It does not perform a separate model lookup or live provider call.

For strict workload selection, load a live compatibility evidence artifact and select through modeldb runtime views. This is the path consumers such as `agentsdk` should use when they only want provider/model/API combinations that are approved for a workload:

```go
evidence, err := adapterconfig.LoadCompatibilityEvidence(
	adapterconfig.DefaultCompatibilityEvidencePath(compatibility.UseCaseAgenticCoding),
)
if err != nil {
	return err
}

selection, err := result.SelectModelForUseCase(
	"haiku",
	"",
	adapterconfig.UseCaseSelectionOptions{
		UseCase:  compatibility.UseCaseAgenticCoding,
		Evidence: evidence,
	},
)
if err != nil {
	return err
}

// Use selection.Resolution.Provider, ProviderAPI, SourceAPI, and NativeModel
// to show or pin the selected route.
```

Strict selection fails closed when no approved live evidence matches. Modeldb remains responsible for aliases, offerings, runtime access, pricing, and capability metadata; the compatibility artifact supplies workload certification.

## Config-Driven Mux Client

Load JSON config and build a client:

```go
cfg, err := adapterconfig.Load("examples/llmadapter.example.json")
if err != nil {
	return err
}

client, err := adapterconfig.NewMuxClient(cfg)
if err != nil {
	return err
}
```

Pin a source API when needed:

```go
client, err := adapterconfig.NewMuxClient(
	cfg,
	adapterconfig.WithSourceAPI(adapt.ApiOpenAIResponses),
)
```

Disable pre-stream fallback:

```go
client, err := adapterconfig.NewMuxClient(
	cfg,
	adapterconfig.WithFallback(false),
)
```

## Sending A Text Request

```go
maxTokens := 512
events, err := client.Request(ctx, unified.Request{
	Model:           "haiku",
	MaxOutputTokens: &maxTokens,
	Stream:          true,
	Messages: []unified.Message{{
		Role: unified.RoleUser,
		Content: []unified.ContentPart{
			unified.TextPart{Text: "Explain Go channels briefly."},
		},
	}},
})
if err != nil {
	return err
}

resp, err := unified.Collect(ctx, events)
if err != nil {
	return err
}
```

Extract text:

```go
var text strings.Builder
for _, part := range resp.Content {
	if part, ok := part.(unified.TextPart); ok {
		text.WriteString(part.Text)
	}
}
```

## Reasoning

Request visible reasoning where supported:

```go
budget := 1024
req.Reasoning = &unified.ReasoningConfig{
	Effort:    unified.ReasoningEffortHigh,
	MaxTokens: &budget,
	Expose:    true,
}
```

Reasoning support is provider/API-kind specific. Check [PROVIDER_MATRIX.md](PROVIDER_MATRIX.md) before depending on it.

`ReasoningEffortMax` is available for providers that expose a maximum reasoning-effort mode. Anthropic-family providers keep explicit `MaxTokens` as manual thinking-budget intent; modeldb-resolved models whose exposure metadata supports adaptive thinking and the requested effort use adaptive thinking plus provider `output_config.effort` when only `Effort` is set. Provider-specific wire values, such as Anthropic Opus 4.7 `xhigh`, are mapped from canonical llmadapter values by modeldb exposure metadata.

## Tools

Declare function tools:

```go
req.Tools = []unified.Tool{{
	Kind:        unified.ToolKindFunction,
	Name:        "lookup_city",
	Description: "Looks up a city by name.",
	InputSchema: json.RawMessage(`{
		"type":"object",
		"properties":{"city":{"type":"string"}},
		"required":["city"],
		"additionalProperties":false
	}`),
}}
```

Force a tool:

```go
req.ToolChoice = &unified.ToolChoice{
	Mode: unified.ToolChoiceTool,
	Name: "lookup_city",
}
```

Tool-loop orchestration is intentionally above llmadapter. llmadapter maps tool calls/results and preserves streamed arguments; your runtime decides how to execute tools and commit conversation state.

## Prompt Caching

Request-level cache intent:

```go
req.CachePolicy = unified.CachePolicyOn
req.CacheKey = "session-123"
req.CacheTTL = "1h"
```

Explicit block cache boundary:

```go
req.Instructions = []unified.Instruction{{
	Kind: unified.InstructionSystem,
	Content: []unified.ContentPart{unified.TextPart{
		Text:         "stable system prefix",
		CacheControl: unified.EphemeralCache("1h"),
	}},
}}
```

Provider mappings are best-effort. Anthropic/Claude/Codex prompt-cache accounting is live smoke-tested; other compatible mappings may not report provider cache counters.

## Continuation And Transport

Route metadata tells consumers how to project the next turn:

- `ConsumerContinuation`: public contract for the caller. Use this field only when deciding whether to replay history or send a provider-native continuation ID.
- `InternalContinuation`: diagnostic metadata for provider-internal optimizations.
- `Transport`: diagnostic metadata for the provider transport used or advertised for the turn.

Codex is intentionally public replay even when it uses WebSocket internally. A branchable runtime such as `agentsdk` should still send full replay projections, but may pass stable Codex hints so llmadapter can keep the provider-internal WebSocket chain branch-safe:

```go
err := unified.SetCodexExtensions(&req.Extensions, unified.CodexExtensions{
	InteractionMode: unified.InteractionSession,
	SessionID:       conversationID,
	BranchID:        branchID,
})
```

Without a stable session/cache key, Codex session requests stay on HTTP/SSE. Direct OpenAI Responses clients can opt into official Responses WebSocket mode with:

```go
client, err := responses.NewClient(
	responses.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
	responses.WithWebSocketMode(responses.WebSocketModeAuto),
)
```

The mode is three-state plus default:

- `WebSocketModeDefault`: provider default. For `openai_responses`, this currently means HTTP/SSE.
- `WebSocketModeAuto`: use WebSocket only when stable session/cache or continuation hints make it useful, with pre-stream HTTP/SSE fallback.
- `WebSocketModeEnabled`: force WebSocket when possible.
- `WebSocketModeDisabled`: force HTTP/SSE.

## Redacted Transport Diagnostics

Library consumers can use the `diagnostics` package to inspect provider HTTP/SSE and WebSocket traffic without using the CLI:

```go
scopes, err := diagnostics.ParseScopes([]string{"request,response,stream"})
if err != nil {
	return err
}

client, err := adapterconfig.NewMuxClient(
	cfg,
	adapterconfig.WithSourceAPI(adapt.ApiOpenAIResponses),
	adapterconfig.WithProviderTransport(diagnostics.NewHTTPTransport(os.Stderr, scopes)),
	adapterconfig.WithProviderWebSocketTransport(diagnostics.NewWebSocketTransport(os.Stderr, scopes)),
)
```

The diagnostics transport redacts known sensitive headers and JSON keys, writes to the supplied writer, and preserves the normal `unified.Client` event stream. It is observational only; it does not change `ConsumerContinuation` or retry semantics.

Direct Codex clients can use the same mode type through `codex.WithWebSocketMode(...)` or keep the compatibility shortcut `codex.WithWebSocketEnabled(false)`. JSON config and auto mux currently use provider defaults because WebSocket is an internal provider optimization, not a route-selection feature. `openrouter_responses` does not opt into OpenAI Responses WebSocket mode.

The default OpenAI Responses WebSocket transport enables compression and forces IPv4. Use `responses.WithWebSocketTransport(...)` or `codex.WithWebSocketTransport(...)` to override that in tests or specialized environments.

## Extensions

Provider-specific controls belong in `unified.Request.Extensions` until they are stable enough to become canonical fields.

Typed helpers exist for mature extension groups:

- `unified.OpenAIResponsesExtensions`
- `unified.OpenRouterExtensions`
- `unified.AnthropicExtensions`
- `unified.CodexExtensions`

Use namespaced extensions instead of adding provider-specific fields to `unified.Request`.

Anthropic extensions currently support beta header values and raw `context_management` request data:

```go
err := unified.SetAnthropicExtensions(&req.Extensions, unified.AnthropicExtensions{
	Betas: []string{"example-beta-2026-01-01"},
	ContextManagement: json.RawMessage(`{
		"edits": [
			{"type": "clear_thinking_20251015", "keep": "all"}
		]
	}`),
})
```

## Usage And Cost

Providers emit `unified.UsageEvent` where usage is available. `unified.Collect` accumulates usage into `unified.Response.Usage`.

Token categories include:

- `input.new`
- `input.cache_read`
- `input.cache_write`
- `output`
- `output.reasoning`

When modeldb pricing metadata is available, adapterconfig can wrap routes with pricing enrichment so usage events include cost items.

Providers may also emit `unified.QuotaUsageEvent` when an upstream exposes subscription or quota-window telemetry. `unified.Collect` appends those snapshots to `unified.Response.Quotas`. Codex maps `x-codex-primary-used-percent` / `x-codex-secondary-used-percent` header families into primary and secondary quota windows. Claude-compatible access maps live `anthropic-ratelimit-unified-5h-*` and `anthropic-ratelimit-unified-7d-*` headers into the same primary and secondary session windows. Anthropic API-key access also maps documented `anthropic-ratelimit-*` headers into request/token quota windows with limit, remaining, reset, and derived used-percent fields. Provider-specific labels and statuses remain in `ProviderRaw`.

## Direct Provider Clients

You can instantiate provider clients directly:

```go
client, err := responses.NewClient(
	responses.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
)
```

Prefer `adapterconfig.NewMuxClient` or `adapterconfig.AutoMuxClient` when you want routing, modeldb metadata, pricing, and fallback.

## What Stays Above llmadapter

Keep these in your application or agent SDK:

- Conversation/session storage.
- Tool execution and commit policy.
- Context-window pruning.
- Long-term memory.
- Retry policy after streamed output starts.
- Provider selection policy that depends on user/account/business state.
