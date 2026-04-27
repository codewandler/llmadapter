# Codex WebSocket Continuation Plan

Status: in progress. llmadapter batches through WebSocket transport, branch-safe internal continuation, runtime execution metadata, and live Codex WebSocket continuation/cache evidence are implemented. The remaining integration work is in agentsdk: metadata-driven projection and explicit Codex session/branch hints for tree-shaped conversations.

Goal: make `codex_responses` use Codex WebSocket by default when available, fall back internally to HTTP/SSE replay when WebSocket cannot be used, and keep consumers such as agentsdk correct for tree-shaped conversations.

## Design Position

`llmadapter` stays stateless at the public API boundary. Consumers send a full canonical request projection. The Codex provider may optimize internally by diffing the current full projection against a previously completed projection for the same Codex session/branch and using WebSocket `previous_response_id` only when lineage is proven.

This keeps HTTP/SSE fallback safe because the full request is always available.

## Current Facts

- Codex HTTP/SSE does not support `previous_response_id`.
- Codex WebSocket does support `previous_response_id` through the WebSocket request shape.
- OpenAI platform Responses has an official WebSocket mode for persistent `/v1/responses` connections; llmadapter exposes this for direct OpenAI Responses clients through `responses.WithWebSocketMode(...)`, while Codex keeps Codex-specific auth/session behavior.
- OpenAI platform Realtime is a separate WebSocket/WebRTC API surface and should not be conflated with Responses WebSocket mode.
- Codex CLI keeps a session-scoped model client, creates turn-scoped sessions, and lazily opens or prewarms WebSocket.
- `codex_responses` should remain one provider endpoint. WebSocket versus HTTP/SSE is a provider-internal transport choice, not two public providers.
- agentsdk owns the tree conversation model, branch selection, turn commit semantics, and durable state.

## Public Semantics

From a consumer perspective, Codex remains replay-projection capable:

```text
consumer/agentsdk
  -> sends full projected canonical request
  -> llmadapter codex provider chooses internal transport
  -> WebSocket path may send delta + previous_response_id internally
  -> HTTP/SSE fallback sends full replay request
```

The consumer must not be required to send only deltas for Codex. If a future high-level delta API is desired, it belongs in agentsdk, backed by full projection before calling llmadapter.

## Interaction Mode

Not every request should pay the complexity cost of WebSocket setup. Add an explicit request/session intent:

```go
type InteractionMode string

const (
	InteractionAuto    InteractionMode = "auto"
	InteractionOneShot InteractionMode = "one_shot"
	InteractionSession InteractionMode = "session"
)
```

Recommended behavior:

- `one_shot`: no durable provider session, no WebSocket prewarm, no internal continuation. Use the simplest HTTP/SSE path.
- `session`: optimize for multi-turn. Codex may open or prewarm WebSocket and maintain branch-safe internal continuation state.
- `auto`: infer from stable hints. If a stable cache/session/branch key exists, behave like `session`; otherwise behave like `one_shot`.

Codex policy:

```text
one_shot -> HTTP/SSE replay
session  -> prefer WebSocket, fallback to HTTP/SSE replay
auto     -> session only when stable session/branch hints exist, otherwise one_shot
```

Default recommendations:

- agentsdk sends `session` because it owns a durable tree conversation.
- `llmadapter infer` defaults to `one_shot`.
- gateway requests default to `auto`.
- library callers default to `auto` unless they explicitly set a mode.

## Metadata To Add

Add explicit continuation and transport metadata to llmadapter route/provider events:

```go
type ContinuationMode string

const (
	ContinuationReplay             ContinuationMode = "replay"
	ContinuationPreviousResponseID ContinuationMode = "previous_response_id"
	ContinuationProviderSession    ContinuationMode = "provider_session"
)

type TransportKind string

const (
	TransportHTTP      TransportKind = "http"
	TransportHTTPSSE   TransportKind = "http_sse"
	TransportWebSocket TransportKind = "websocket"
)
```

Expose at least:

- `consumer_continuation`: what callers must provide.
- `internal_continuation`: diagnostic metadata for what the provider actually used internally.
- `transport`: diagnostic metadata for the actual provider transport used for the completed turn.

`consumer_continuation` is the only public projection-strategy signal. Consumers must not use provider name, API family, `transport`, or `internal_continuation` to decide whether to replay or send native continuation IDs.

For Codex WebSocket:

```text
consumer_continuation=replay
internal_continuation=previous_response_id
transport=websocket
```

For Codex HTTP/SSE fallback:

```text
consumer_continuation=replay
internal_continuation=replay
transport=http_sse
```

For OpenAI Responses native continuation:

```text
consumer_continuation=previous_response_id
internal_continuation=previous_response_id
transport=http_sse
```

## Codex Provider Implementation Plan

1. Keep `providers/openai/responses` as the canonical OpenAI Responses wire/client implementation.
2. Keep `providers/openai/codex` as a wrapper over OpenAI Responses with Codex auth, URL/header behavior, default instructions, session/window headers, and unsupported-field handling.
3. Add a Codex WebSocket transport implementation inside `providers/openai/codex`.
4. Add a Codex session manager keyed by stable caller/session hints plus request identity.
5. Prefer WebSocket by default.
6. Fall back to HTTP/SSE only for setup/pre-stream WebSocket failures.
7. Once a WebSocket stream has started, surface errors instead of silently retrying through HTTP/SSE.
8. Discard the affected Codex WebSocket session after stale writes or incomplete streams, but do not permanently disable WebSocket for the provider after a transient pre-stream fallback.
9. Do not expose WebSocket and HTTP/SSE as separate provider endpoint types.

## Branch-Safe Internal Continuation

The Codex provider must never attach to the wrong previous response. Internal WebSocket continuation can be used only when all lineage checks pass.

Maintain Codex continuation state per branch head:

```go
type CodexContinuationState struct {
	SessionKey          string
	BranchKey           string
	Model               string
	InstructionHash     string
	LastFullInputHash    string
	LastInputFingerprint string
	LastResponseID       string
	WindowID             string
	TurnState            string
	WebSocketDisabled    bool
}
```

Lineage checks before sending WebSocket `previous_response_id`:

- Same Codex session key.
- Same branch key.
- Same model.
- Same instruction/developer prompt hash, unless Codex semantics prove replacement is safe.
- Previous response ID exists.
- Current full projected request is an append-only extension of the previous full projected request, verified by canonical per-input-item prefix hashes.
- Previous turn was completed and committed.
- No branch switch, rollback, compaction rewrite, system prompt rewrite, tool-result mutation, or model change invalidated the chain.

If any check fails:

- Use WebSocket without `previous_response_id` if the full request can be sent safely.
- Otherwise use HTTP/SSE replay.
- Do not advance continuation state until the provider turn completes successfully.

## Request Hints

Add optional namespaced Codex hints in `unified.Request.Extensions`:

```text
codex.interaction_mode
codex.session_id
codex.branch_id
codex.branch_head_id
codex.input_base_hash
codex.parent_response_id
```

These are hints for keying and diagnostics. The provider must compute its own request fingerprint from the outgoing full wire body and must not trust caller-provided hashes as proof of lineage.

## agentsdk Changes

agentsdk should:

1. Continue to own the conversation tree and branch selection.
2. Continue to build full replay projections for Codex calls.
3. Pass stable Codex hints through llmadapter extensions:
   - `codex.interaction_mode=session`.
   - session ID from the agentsdk conversation/session.
   - branch ID or branch path.
   - branch head ID.
   - optional parent provider response ID for diagnostics.
4. Use llmadapter use-case selection for model routing:
   - select with `agentic_coding` by default for agentic coding flows.
   - fail closed unless a provider/model/API row is approved by compatibility evidence.
5. Choose public projection strategy from `consumer_continuation`, not API family or provider name.
6. Store per-turn provider metadata:
   - provider name.
   - API kind.
   - native model.
   - transport used.
   - consumer continuation mode.
   - internal continuation mode.
   - response ID.
   - cache/usage data.
7. Only use public `previous_response_id` projection when the selected route explicitly says `consumer_continuation=previous_response_id` and the previous committed turn metadata matches.
8. Treat Codex as replay at the public projection boundary even when the actual Codex transport reports WebSocket internal continuation.

Current agentsdk gap after llmadapter `v1.0.0-rc.7`:

- agentsdk already avoids public `previous_response_id` for Codex via provider/API heuristics, but it should switch to the explicit `RouteEvent.ConsumerContinuation` metadata from llmadapter.
- agentsdk currently sends `CacheKey` but not Codex-specific session/branch extensions. That is sufficient for linear cache affinity, but not sufficient for tree-shaped Codex WebSocket state. It should set `codex.interaction_mode=session`, `codex.session_id`, and `codex.branch_id` from the active conversation session/branch when the selected route is `codex.responses`.
- agentsdk should persist route/provider metadata including `transport`, `consumer_continuation`, and `internal_continuation` with committed continuations for diagnostics. Projection decisions should depend on `consumer_continuation` only.
- The llmadapter release that ships Codex WebSocket support must include migration notes for agentsdk and miniagent maintainers: Codex remains public replay/stateless, WebSocket/partial requests are internal optimizations, and `consumer_continuation` is the only projection-strategy signal consumers should rely on.

## agentsdk Tree Safety Rules

agentsdk must invalidate or avoid native public continuation when:

- The user switches branches.
- The selected model/provider/API changes.
- The system/developer prompt changes in a way that is not supported by the provider continuation semantics.
- A tool result or assistant message is edited.
- A turn failed or was not committed.
- A compaction/reprojection changed the prefix.

For Codex, those invalidations should still be passed as full replay; llmadapter Codex can then decide whether its internal WebSocket chain is still usable.

## Compatibility Evidence Changes

Extend compatibility artifacts with continuation and transport metadata:

```json
{
  "consumer_continuation": "replay",
  "internal_continuation": "previous_response_id",
  "transport": "websocket"
}
```

Agentic coding should require correctness, cache accounting, tools, structured output, reasoning, and usage. It should not require public `previous_response_id`; replay is acceptable when provider caching/accounting make costs observable.

## Tests

llmadapter tests:

- Codex HTTP/SSE drops `previous_response_id` with warning.
- Codex WebSocket is preferred by default when available.
- Codex falls back to HTTP/SSE on pre-stream WebSocket failure.
- Codex does not fall back after stream start.
- Codex internal continuation is used only when branch/session/model/input lineage matches.
- Codex resets internal continuation on branch switch, model switch, compaction rewrite, or prompt rewrite.
- Route/provider metadata reports actual transport and continuation modes.
- OpenAI Responses still advertises public `previous_response_id` continuation.
- OpenRouter does not inherit public continuation unless live-verified.

agentsdk tests:

- Model selection uses llmadapter `agentic_coding` compatibility evidence.
- Codex projection remains full replay even when metadata later reports WebSocket internal continuation.
- Public `previous_response_id` is used only for routes with `consumer_continuation=previous_response_id`.
- Branch switch does not reuse previous response from another branch.
- Failed turns do not advance continuation state.
- Tool-loop commits advance state only after a successful assistant/tool boundary.

## Rollout Phases

### Phase 1: Metadata And Public Strategy

- Add continuation and transport metadata types.
- Expose metadata through provider descriptors, route events, config inspection, CLI resolve/conformance JSON, and compatibility artifact rows.
- Mark Codex public continuation as replay.
- Mark OpenAI Responses public continuation as `previous_response_id` only where live-verified.

### Phase 2: Codex HTTP/SSE Correctness

- Keep Codex on HTTP/SSE.
- Drop unsupported `previous_response_id` with warning.
- Ensure agentsdk can identify Codex as replay through metadata.
- Update docs and compatibility evidence.

### Phase 3: agentsdk Use-Case Selection

- Update agentsdk runtime/model setup to call llmadapter use-case selection for `agentic_coding`.
- Persist route/provider continuation metadata per turn.
- Use metadata-driven projection strategy.
- Pass `interaction_mode=session` plus stable session/branch hints for Codex-capable routes.

### Phase 3b: CLI Continuation Diagnostics

- Add `llmadapter infer --session <id>` to set a stable session/cache key.
- Add `llmadapter infer --branch <id>` for branch-specific continuation diagnostics.
- Add `llmadapter infer --interaction one_shot|session|auto`.
- Keep default `infer` behavior as `one_shot`.
- When `--session` is set and `--interaction` is omitted, use `session`.
- Print resolved continuation metadata before output and actual transport/continuation metadata after completion.
- Use these flags to manually test Codex WebSocket continuation, HTTP/SSE fallback, and OpenAI Responses native continuation without needing a full agentsdk run.

### Phase 4: Codex WebSocket Transport

- Implement Codex WebSocket transport.
- Prefer WebSocket by default.
- Add prewarm/lazy-open behavior.
- Add pre-stream HTTP/SSE fallback.
- Emit actual transport/internal continuation metadata.

### Phase 5: Branch-Safe Internal Diffing

- Add Codex continuation state keyed by session/branch/model/request fingerprint.
- Add append-only projection diffing.
- Use WebSocket `previous_response_id` only when lineage checks pass.
- Reset safely on branch/prompt/model/rewrite changes.

### Phase 6: Live Compatibility

- Add live Codex WebSocket continuation smoke.
- Add live Codex WebSocket prompt-cache smoke requiring provider-reported cache-read accounting.
- Add fallback smoke where feasible with fake transport.
- Record transport/continuation evidence in compatibility artifacts.
- Refresh docs and conformance report output.

## Concrete Execution Checklist

### Batch 1: Continuation And Transport Metadata

Files:

- `unified/events.go`
- `router/router.go`
- `providerregistry/registry.go`
- `adapterconfig/inspect.go`
- `adapterconfig/compatibility_evidence.go`
- `compatibility/artifact.go`
- `compatibility/markdown.go`
- `cmd/llmadapter/main.go`
- `conformance/report.go`

Steps:

1. Add canonical `ContinuationMode`, `TransportKind`, and provider execution metadata types in `unified`.
2. Extend `unified.RouteEvent` or add a provider metadata event carrying:
   - `consumer_continuation`.
   - `internal_continuation`.
   - `transport`.
3. Add descriptor-level defaults in `providerregistry.Descriptor`.
4. Set initial defaults:
   - `openai_responses`: `consumer_continuation=previous_response_id`, `transport=http_sse`.
   - `codex_responses`: `consumer_continuation=replay`, `transport=http_sse`.
   - `openrouter_responses`: `consumer_continuation=replay` until live continuation is verified.
   - chat/messages providers: `consumer_continuation=replay`.
5. Expose fields in config inspection and route summaries.
6. Expose fields in `llmadapter resolve --json`, `llmadapter conformance --json`, and text output where useful.
7. Extend compatibility artifact rows with continuation/transport fields.
8. Extend generated compatibility markdown with the new columns.

Tests:

- `unified` event collection keeps metadata.
- `providerregistry` descriptors have non-empty continuation metadata.
- CLI JSON includes continuation fields.
- compatibility artifact load/save remains backward compatible with older artifacts.

Verification:

```sh
env GOCACHE=/tmp/go-cache go test ./unified ./providerregistry ./adapterconfig ./compatibility ./cmd/llmadapter ./conformance
```

### Batch 2: Codex HTTP/SSE Correctness And OpenAI Responses Ownership

Files:

- `providers/openai/responses/client.go`
- `providers/openai/responses/options.go`
- `providers/openai/responses/codec.go`
- `providers/openai/responses/wire.go`
- `providers/openai/responses/client_test.go`
- `providers/openai/codex/client.go`
- `providers/openai/codex/client_test.go`
- `providers/openrouter/responses/client.go`
- `providers/openrouter/responses/client_test.go`
- `docs/ARCHITECTURE.md`
- `docs/PROVIDER_MATRIX.md`

Steps:

1. Keep `providers/openai/responses` as the owner of the base Responses provider wire/client implementation.
2. Ensure `providers/openai/codex` imports `providers/openai/responses`, not OpenRouter.
3. Ensure `providers/openrouter/responses` imports/wraps `providers/openai/responses` and applies only OpenRouter-specific body extensions.
4. Keep Codex HTTP/SSE fallback behavior:
   - drops `previous_response_id`.
   - emits `unsupported_field_dropped`.
   - maps cache/session keys to Codex headers.
5. Assert no OpenAI/Codex package imports OpenRouter Responses.

Tests:

- Codex drops unsupported `previous_response_id`.
- Codex warning source is `codex.responses`.
- OpenAI Responses still encodes `previous_response_id`.
- OpenRouter Responses still encodes OpenRouter-only extensions.
- `rg "openrouterresponses|providers/openrouter/responses" providers/openai` returns no matches.

Verification:

```sh
env GOCACHE=/tmp/go-cache go test ./providers/openai/responses ./providers/openai/codex ./providers/openrouter/responses
```

### Batch 3: Interaction Mode And CLI Diagnostics

Files:

- `unified/extensions.go`
- `unified/extensions_test.go`
- `providers/openai/codex/config.go`
- `providers/openai/codex/client.go`
- `providers/openai/codex/client_test.go`
- `cmd/llmadapter/main.go`
- `cmd/llmadapter/main_test.go`
- `docs/CLI.md`
- `README.md`

Steps:

1. Add typed extension helpers for:
   - `codex.interaction_mode`.
   - `codex.session_id`.
   - `codex.branch_id`.
   - `codex.branch_head_id`.
   - `codex.input_base_hash`.
   - `codex.parent_response_id`.
2. Validate interaction mode values: `auto`, `one_shot`, `session`.
3. Make Codex HTTP/SSE path treat `one_shot` as no durable WebSocket/session state.
4. Add `llmadapter infer` flags:
   - `--interaction one_shot|session|auto`.
   - `--session <id>`.
   - `--branch <id>`.
5. Default `infer` to `one_shot`.
6. If `--session` is set and `--interaction` is omitted, use `session`.
7. Set `CacheKey` plus Codex extension hints from `infer`.
8. Print intended continuation/session metadata before streaming.
9. Keep existing `--no-cache` behavior: it disables prompt cache hints but should not necessarily remove explicit session diagnostics unless documented.

Tests:

- Codex extension helpers round-trip and reject invalid interaction mode.
- `infer --session demo` sets cache/session hints.
- `infer` without flags remains one-shot.
- `infer --interaction session --branch b1` emits expected request metadata in a fake-client path.

Verification:

```sh
env GOCACHE=/tmp/go-cache go test ./unified ./providers/openai/codex ./cmd/llmadapter
```

### Batch 4: agentsdk Use-Case Selection Integration

Repository:

- `../agentsdk`

Files to inspect first:

- `../agentsdk/runtime/auto.go`
- `../agentsdk/runtime/options.go`
- `../agentsdk/agent/options.go`
- `../agentsdk/agent/agent.go`
- `../agentsdk/conversation/projection_policy.go`
- `../agentsdk/conversation/continuation.go`
- `../agentsdk/runner/runner.go`
- `../agentsdk/conversation/session.go`
- `../agentsdk/CHANGELOG.md`
- `../agentsdk/README.md`

Steps:

1. Upgrade agentsdk to the llmadapter release containing Batch 1-3.
2. Replace plain model selection with llmadapter use-case selection for agentic flows:
   - default use case: `agentic_coding`.
   - fail closed when no approved evidence row matches.
3. Add runtime option to choose use case, with `agentic_coding` as the miniagent/agentic default.
4. Store route/provider metadata per committed turn:
   - provider.
   - API kind.
   - native model.
   - transport.
   - consumer continuation.
   - internal continuation.
   - response ID.
5. Projection policy:
   - use public `previous_response_id` only when `consumer_continuation=previous_response_id`.
   - Codex remains full replay because its `consumer_continuation=replay`.
6. Add Codex hints when selected provider/API is Codex:
   - `codex.interaction_mode=session`.
   - session ID.
   - branch ID.
   - branch head ID.
   - optional parent response ID.
7. Preserve existing tree safety invalidations.

Tests:

- use-case selection rejects unapproved rows.
- Codex request projection remains replay and includes Codex session hints.
- OpenAI Responses can still use public `previous_response_id`.
- branch switch does not reuse prior provider response ID.
- failed turn does not advance continuation.

Verification:

```sh
env GOCACHE=/tmp/go-cache go test ./...
```

### Batch 5: Codex WebSocket Transport Skeleton

Files:

- `providers/openai/codex/ws.go`
- `providers/openai/codex/ws_test.go`
- `providers/openai/codex/client.go`
- `providers/openai/codex/options.go`
- `providers/openai/codex/config.go`
- `providers/openai/codex/client_test.go`
- `transport/` if a reusable WebSocket byte-stream abstraction is introduced.

Steps:

1. Add Codex WebSocket option surface:
   - enabled by default for session mode.
   - disable option for tests/operators.
   - prewarm option if needed.
2. Add WebSocket request/response wire structs based on Codex source.
3. Implement lazy WebSocket open.
4. Implement pre-stream fallback to HTTP/SSE on setup failures.
5. Discard stale Codex WebSocket sessions after failed writes or incomplete streams without permanently disabling WebSocket for the provider.
6. Emit metadata:
   - WebSocket success: `transport=websocket`, `internal_continuation=previous_response_id` when used.
   - HTTP fallback: `transport=http_sse`, `internal_continuation=replay`.
7. Keep one public `codex_responses` provider endpoint.

Tests:

- session interaction prefers WebSocket.
- one-shot interaction uses HTTP/SSE.
- auto without stable session hints uses HTTP/SSE.
- auto with stable session hints uses WebSocket.
- pre-stream WebSocket failure falls back to HTTP/SSE.
- post-stream WebSocket failure does not fallback.

Verification:

```sh
env GOCACHE=/tmp/go-cache go test ./providers/openai/codex
```

### Batch 6: Branch-Safe Codex Internal Continuation

Files:

- `providers/openai/codex/session.go`
- `providers/openai/codex/session_test.go`
- `providers/openai/codex/diff.go`
- `providers/openai/codex/diff_test.go`
- `providers/openai/codex/ws.go`
- `providers/openai/codex/client.go`

Steps:

1. Add in-memory Codex session manager.
2. Key state by session key, branch key, model, and instruction hash.
3. Compute request fingerprints from the actual outgoing full wire body.
4. Implement append-only diff detection.
5. Use internal WebSocket `previous_response_id` only when lineage checks pass.
6. Reset internal continuation on:
   - branch switch.
   - model switch.
   - prompt/instruction rewrite.
   - compaction rewrite.
   - changed tool result.
   - failed/uncommitted turn.
7. Advance state only after successful completion.

Tests:

- append-only turn uses internal previous response ID.
- branch switch does not.
- model switch does not.
- prompt rewrite does not.
- failed turn does not advance state.
- retry after pre-stream failure uses HTTP/SSE replay.

Verification:

```sh
env GOCACHE=/tmp/go-cache go test ./providers/openai/codex
```

### Batch 7: Live Compatibility And Evidence

Files:

- `tests/e2e/smoke_test.go`
- `tests/e2e/usecase_agentic_coding_test.go`
- `docs/compatibility/agentic_coding.json`
- `docs/USE_CASE_MATRIX.md`
- `docs/PROVIDER_MATRIX.md`
- `docs/CLI.md`
- `CHANGELOG.md`
- `PLAN.md`

Steps:

1. Add live Codex WebSocket smoke gated by Codex local auth.
2. Record actual transport and continuation metadata in e2e compatibility rows.
3. Update `compatibility-record` output.
4. Re-run compatibility artifact generation when credentials are available.
5. Update provider matrix and use-case matrix.
6. Cut release after docs and tests are green.

Verification:

```sh
env GOCACHE=/tmp/go-cache go test ./...
env GOCACHE=/tmp/go-cache go vet ./...
env GOCACHE=/tmp/go-cache GOMODCACHE=/tmp/go-mod-cache go build ./...
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmoke.*Codex|TestUseCaseAgenticCoding' -count=1 -v
```

## Non-Goals

- Do not make llmadapter own durable conversation trees.
- Do not require consumers to send Codex deltas.
- Do not split Codex into two public provider endpoint types just for WebSocket versus HTTP/SSE.
- Do not use provider name or API family alone to decide continuation strategy.
