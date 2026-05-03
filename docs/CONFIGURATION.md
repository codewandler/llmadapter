# Configuration

`llmadapter` JSON config defines provider instances, routes, modeldb catalog/overlays, health behavior, and fallback limits.

The same config path is used by:

- `llmadapter serve`
- `llmadapter infer --config`
- `llmadapter routes --config`
- `llmadapter models --config`
- `llmadapter resolve --config`
- `adapterconfig.Load`
- `adapterconfig.NewMuxClient`

## Minimal Config

```json
{
  "providers": [
    {
      "name": "anthropic",
      "type": "anthropic",
      "api_key_env": "ANTHROPIC_API_KEY",
      "model": "claude-haiku-4-5-20251001"
    }
  ],
  "routes": [
    {
      "source_api": "anthropic.messages",
      "model": "claude-haiku",
      "provider": "anthropic",
      "provider_api": "anthropic.messages",
      "native_model": "claude-haiku-4-5-20251001",
      "weight": 100
    }
  ]
}
```

Inspect:

```sh
go run ./cmd/llmadapter serve --config llmadapter.json --inspect-config
```

## Top-Level Fields

| Field | Purpose |
| --- | --- |
| `addr` | Gateway listen address, for example `:8080`. |
| `health_cooldown` | Go duration for temporarily deprioritizing failed provider/API/model pairs. |
| `max_attempts` | Maximum ranked route attempts before returning the combined route-attempt error. `0` means all compatible candidates. |
| `modeldb` | Catalog path, overlays, and local aliases. |
| `providers` | Provider instances. |
| `routes` | Source API/model to provider endpoint routing. |

Gateway request hardening is currently fixed by implementation defaults: endpoint request bodies are limited to 10 MiB, the HTTP server uses read-header/read/write/idle timeouts, and provider HTTP clients require TLS 1.2 or newer. These are not yet config fields.

## Providers

Provider fields:

| Field | Purpose |
| --- | --- |
| `name` | Provider instance name used by routes. |
| `type` | Provider endpoint implementation type. |
| `api_key_env` | Env var containing an API key. |
| `api_key` | Inline API key; useful for tests, not recommended for committed configs. |
| `base_url` | Provider base URL override. |
| `model` | Default native model for fixed routes without `native_model`. |
| `priority` | Tie-breaker after route weight. |
| `modeldb_service_id` | Optional service identity override for modeldb metadata/pricing. Known provider types infer this automatically. |
| `capabilities` | Optional override of provider descriptor defaults. |

Supported provider endpoint types are documented in [PROVIDER_MATRIX.md](PROVIDER_MATRIX.md).

Known provider endpoint types infer their modeldb service identity automatically: Anthropic/Claude map to `anthropic`, OpenAI maps to `openai`, Codex maps to `codex`, Bedrock maps to `bedrock`, OpenRouter maps to `openrouter`, and MiniMax maps to `minimax`. Set `modeldb_service_id` only when using a custom provider type or deliberately overriding that identity.

Amazon Bedrock Mantle Responses can be configured as an OpenAI Responses-family endpoint:

```json
{
  "providers": [
    {
      "name": "bedrock",
      "type": "bedrock_responses",
      "api_key_env": "BEDROCK_API_KEY",
      "model": "openai.gpt-oss-120b"
    }
  ],
  "routes": [
    {
      "source_api": "openai.responses",
      "model": "bedrock-gpt-oss",
      "provider": "bedrock",
      "provider_api": "bedrock.responses",
      "native_model": "openai.gpt-oss-120b"
    }
  ]
}
```

If `base_url` is omitted, `bedrock_responses` uses `https://bedrock-mantle.${AWS_REGION}.api.aws`, then `AWS_DEFAULT_REGION`, then `us-east-1`. If you paste AWS's OpenAI SDK style URL with a trailing `/v1`, llmadapter normalizes it before calling `/v1/responses`.

Authentication prefers explicit `BEDROCK_API_KEY` or `AWS_BEARER_TOKEN_BEDROCK` bearer values. If neither is configured, `bedrock_responses` loads AWS credentials through the AWS SDK default chain, including `AWS_PROFILE` and SSO-backed profiles, and generates a short-term Bedrock bearer token for the selected region. Short-term tokens are cached in memory and refreshed before expiry.

Bedrock Mantle's `/v1/models` endpoint can list models for multiple OpenAI-compatible API surfaces. `bedrock_responses` only targets `/v1/responses`; the currently live-verified Responses models are `openai.gpt-oss-120b` and `openai.gpt-oss-20b`. Other listed Mantle models may require a different endpoint shape such as Chat Completions or Messages. Bedrock currently accepts only `tool_choice: "auto"` on this Responses surface, so llmadapter sends non-auto canonical tool choices as `auto` and emits a warning.

Amazon Bedrock Mantle Anthropic-compatible Messages can be configured separately:

```json
{
  "providers": [
    {
      "name": "bedrock-claude",
      "type": "bedrock_messages",
      "model": "anthropic.claude-opus-4-7"
    }
  ],
  "routes": [
    {
      "source_api": "anthropic.messages",
      "model": "bedrock-opus",
      "provider": "bedrock-claude",
      "provider_api": "bedrock.anthropic_messages",
      "native_model": "anthropic.claude-opus-4-7"
    }
  ]
}
```

`bedrock_messages` uses the Mantle Anthropic route prefix, so the default upstream URL is `https://bedrock-mantle.${AWS_REGION}.api.aws/anthropic/v1/messages`. If `base_url` is configured, it may be the host, the `/anthropic` prefix, or the full `/anthropic/v1/messages` endpoint. Live-probed Bedrock Messages model IDs currently include `anthropic.claude-opus-4-7` and `anthropic.claude-haiku-4-5`; `anthropic.claude-opus-4-6` is not available as a Mantle Messages model ID in `us-east-1`.

Amazon Bedrock native Converse can be configured separately when you want the AWS SDK-backed Bedrock Runtime surface instead of Mantle:

```json
{
  "providers": [
    {
      "name": "bedrock-converse",
      "type": "bedrock_converse",
      "model": "anthropic.claude-sonnet-4-6"
    }
  ],
  "routes": [
    {
      "source_api": "anthropic.messages",
      "model": "bedrock-sonnet",
      "provider": "bedrock-converse",
      "provider_api": "bedrock.converse",
      "native_model": "anthropic.claude-sonnet-4-6"
    }
  ]
}
```

`bedrock_converse` uses the AWS SDK default credential chain, including `AWS_PROFILE`, SSO-backed profiles, `AWS_REGION`, and `AWS_DEFAULT_REGION`. The default region is `us-east-1`, and the default model is `anthropic.claude-sonnet-4-6`. Modeldb-backed fixed and dynamic routes prefer Bedrock `RuntimeAccess.ResolvedWireID` rows for the region runtime, falling back to `bedrock-global` access when present. Direct clients and routes without runtime metadata preserve explicit `us.`, `eu.`, `apac.`, or `global.` model prefixes and otherwise use the provider-local fallback table.

Provider JSON config does not currently expose provider-internal transport toggles. For example, `codex_responses` may use WebSocket internally for explicit session requests, but that behavior is controlled by request extensions and provider implementation defaults rather than route config. Direct provider users can control this with `responses.WithWebSocketMode(...)` for `providers/openai/responses` or `codex.WithWebSocketMode(...)` for `providers/openai/codex`.

## Routes

Route fields:

| Field | Purpose |
| --- | --- |
| `source_api` | Incoming API kind, for example `openai.responses`. |
| `model` | Public model name accepted by this route. |
| `provider` | Provider instance name. |
| `provider_api` | Exact provider API kind when a provider has multiple endpoints. |
| `native_model` | Provider wire model ID. |
| `modeldb_model` | Model/alias resolved through modeldb for this provider/API. |
| `modeldb_wire_model_id` | Explicit modeldb wire model ID for pricing/metadata. |
| `dynamic_models` | Resolve/pass requested model names dynamically. |
| `weight` | Primary route ranking value. |

Routes select provider endpoints, not just provider names. If a provider exposes multiple API kinds, set `provider_api` to avoid ambiguity.

Routes and config inspection expose continuation/transport metadata. Use `consumer_continuation` to decide caller projection behavior; treat `internal_continuation` and `transport` as diagnostics.

## Weighted Routing And Fallback

Compatible routes are ranked by:

1. Route `weight`.
2. Provider endpoint `priority`.
3. Source-family preference in auto-source mode.
4. Declaration order.

Fallback only happens before streaming/response bytes start. Request-shape validation errors and 400/422 provider API errors are non-retryable.

Use `max_attempts` to bound fallback:

```json
{
  "max_attempts": 2
}
```

## Dynamic Models

Dynamic routes let callers request provider/catalog models without predeclaring every model:

```json
{
  "source_api": "openai.responses",
  "provider": "openai",
  "provider_api": "openai.responses",
  "dynamic_models": true,
  "weight": 5
}
```

When modeldb is enabled for dynamic routing through catalog config or an explicit provider `modeldb_service_id`, unknown dynamic models are rejected instead of silently falling back to provider defaults. Without modeldb dynamic routing, the requested model name is passed through unchanged.

## Modeldb

Modeldb supplies:

- Alias resolution.
- Service/wire-model offerings.
- Capability narrowing.
- Token limits.
- Pricing metadata.

Example:

```json
{
  "modeldb": {
    "catalog_path": "./catalog.json",
    "overlay_paths": ["./local-modeldb.json"],
    "aliases": [
      {
        "name": "example-fast",
        "service_id": "openrouter",
        "wire_model_id": "openai/gpt-4.1-mini"
      }
    ]
  }
}
```

If `catalog_path` is omitted, the built-in modeldb catalog is used. Overlays are merged after the base catalog.
Catalog aliases are the default source of truth. `modeldb.aliases` is only for explicit operator overrides or local shortcuts, and `AutoMuxClient` does not inject llmadapter-owned built-in aliases ahead of the catalog.
Do not rely on legacy llmadapter-owned shortcuts such as `fast`, `powerful`, or `codex`; use modeldb catalog names, service-qualified names, or aliases you define in config.

## Capability Overrides

Provider descriptor capabilities are endpoint-family defaults. Override them when a configured model is narrower than the descriptor:

```json
{
  "name": "openrouter",
  "type": "openrouter_responses",
  "api_key_env": "OPENROUTER_API_KEY",
  "capabilities": {
    "streaming": true,
    "tools": true,
    "vision": false,
    "json_mode": true,
    "json_schema": true,
    "reasoning": false
  }
}
```

`resolve` and `serve --inspect-config` report capability provenance:

- `provider_descriptor`: static endpoint-family/provider defaults.
- `config_override`: explicit config override.
- `modeldb_exposure`: modeldb offering exposure metadata.

## Pricing

Pricing enrichment is optional and absent-safe. It is enabled when a route can identify:

- provider modeldb service identity, inferred from known provider types or overridden with `modeldb_service_id`
- selected wire model
- modeldb offering with pricing

Fixed route:

```json
{
  "source_api": "openai.responses",
  "model": "example-fast",
  "provider": "openrouter",
  "provider_api": "openrouter.responses",
  "modeldb_model": "example-fast",
  "weight": 100
}
```

Dynamic route:

```json
{
  "source_api": "openai.responses",
  "provider": "openrouter",
  "provider_api": "openrouter.responses",
  "dynamic_models": true,
  "weight": 1
}
```

## Example Config

The repository ships:

- `examples/llmadapter.example.json`
- `examples/modeldb.overlay.example.json`

Validate and inspect:

```sh
go run ./cmd/llmadapter serve --config examples/llmadapter.example.json --inspect-config
go run ./cmd/llmadapter resolve --config examples/llmadapter.example.json example-fast
```

The example covers provider endpoints, dynamic routes, modeldb overlays, explicit operator-defined aliases, capability overrides, pricing metadata, and route attempt limits.
