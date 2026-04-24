# llmadapter Implementation Plan

Refined execution plan for `DESIGN.md` phases 1-3.

Primary goal: reach a working programmatic client for Anthropic Messages with a stable core IR, stream pipeline, and transport foundation. The first implementation pass should optimize for a thin, testable vertical slice, not for maximum provider coverage.

---

## Current Status

Status date: 2026-04-24.

Phases 1-3 are implemented as a first working vertical slice.

Completed:

```text
Phase 1: unified, adapt, and pipeline core packages
Phase 2: transport package with SSE, NDJSON, HTTP, fake, retry, and rate-limit wrappers
Phase 3: Anthropic Messages programmatic client path
Phase 4 first slice: /v1/chat/completions endpoint codec and minimal gateway handler
Phase 4 gateway e2e slice: runnable Anthropic-backed gateway command and live gateway smoke tests
Phase 6 first slice: static router with endpoint/model matching and native model rewrite
Gateway config slice: optional JSON config for providers and static routes
Provider support slice: OpenAI Chat Completions upstream provider
Gateway provider matrix slice: OpenAI-backed route covered by live gateway smoke tests
Tool-use provider slice: OpenAI streamed tool calls and shared live tool-use smoke tests
Tool loop e2e slice: shared live tool-result continuation smoke tests
OpenRouter provider slice: native Chat Completions provider wrapper and shared smoke matrix entry
Provider endpoint routing slice: routes carry target API kind, API family, provider name, and capabilities
OpenRouter multi-endpoint slice: native Responses text streaming and Anthropic-compatible Messages support
OpenRouter Responses tool slice: native Responses function-call streaming and tool-result continuation support
Documentation slice: minimal README, AGENTS, and provider-extension agent skill
MiniMax provider slice: OpenAI-compatible Chat Completions wrapper, gateway registration, and shared text smoke matrix entry
MiniMax Messages slice: Anthropic-compatible Messages wrapper, gateway registration, and shared text/tool smoke matrix entries
Conformance cleanup slice: OpenAI Chat gateway reasoning_details encoding and structured provider HTTP/mid-stream error tests
Endpoint slice: downstream Anthropic-compatible /v1/messages gateway codec and live Anthropic-family gateway smokes
Endpoint slice: downstream OpenAI-compatible /v1/responses gateway codec and live OpenRouter Responses gateway smokes
Routing hardening slice: static router now checks request capabilities before selecting a provider endpoint
Mapping warnings slice: Anthropic-family provider best-effort request mapping emits canonical warning events for dropped unsupported fields
Mapping warnings slice: OpenAI Chat, OpenRouter Chat, and OpenRouter Responses provider mappings emit canonical warning events for dropped non-text content and unsupported tool kinds
Endpoint decode warnings slice: OpenAI Chat Completions, OpenAI Responses, and Anthropic Messages endpoint codecs retain warnings on adapt.Request for unsupported inbound fields dropped during canonical decode
OpenRouter extension passthrough slice: endpoint codecs preserve OpenRouter routing/provider/plugin/debug/trace/session controls in unified.Request.Extensions and OpenRouter Chat, Responses, and Messages providers encode those controls upstream
Weighted routing slice: static router ranks compatible candidates by route weight and endpoint priority while falling back past capability mismatches
Gateway fallback slice: gateway handlers retry lower-ranked route candidates when the selected provider fails before response bytes are written
OpenAI Chat structured-output slice: OpenAI Chat-compatible endpoint decode and provider encode preserve response_format JSON mode and JSON schema requests
OpenAI Responses structured-output slice: OpenAI Responses endpoint decode and OpenRouter Responses provider encode preserve text.format JSON mode and JSON schema requests
Endpoint image decode slice: OpenAI Chat, OpenAI Responses, and Anthropic Messages endpoint codecs preserve supported image inputs as canonical ImagePart values
Provider image passthrough slice: OpenAI Chat-compatible providers, OpenRouter Responses, and Anthropic-compatible providers encode supported canonical image inputs upstream; gateway metadata advertises vision on those endpoint families
```

Verified:

```text
env GOCACHE=/tmp/go-cache go test ./...
env GOCACHE=/tmp/go-cache go build ./...
env GOCACHE=/tmp/go-cache go vet ./...
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run TestSmokeTextStream -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestGatewaySmoke' -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeTextStream/openai_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestGatewaySmoke.*/openai_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolUse' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolResultContinuation' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeTextStream/openrouter_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolUse/openrouter_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolResultContinuation/openrouter_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestGatewaySmoke.*/openrouter_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeTextStream' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolUse' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolResultContinuation' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestGatewaySmoke' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeTextStream/minimax_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestGatewaySmoke.*/minimax_chat' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeTextStream/minimax_messages' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolUse/minimax_messages' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestSmokeToolResultContinuation/minimax_messages' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestGatewaySmoke.*/minimax_messages' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestAnthropicMessagesGatewaySmoke' -count=1 -v
env GOCACHE=/tmp/go-cache TEST_INTEGRATION=1 go test ./tests/e2e -run 'TestResponsesGatewaySmoke' -count=1 -v
```

Implemented package surface:

```text
unified/
adapt/
pipeline/
transport/
providers/anthropic/messages/
providers/openai/chatcompletions/
providers/openrouter/chatcompletions/
providers/openrouter/messages/
providers/openrouter/responses/
providers/minimax/chatcompletions/
providers/minimax/messages/
tests/e2e/
endpoints/openaichatcompletions/
endpoints/openairesponses/
endpoints/anthropicmessages/
gateway/
cmd/llmadapter-gateway/
router/
.agents/skills/llmadapter-provider-extension/
```

Gateway command config:

```text
LLMADAPTER_CONFIG optionally points to a JSON config file
LLMADAPTER_ADDR sets the listen address when no config file is used
ANTHROPIC_API_KEY provides default Anthropic credentials when no config file is used
LLMADAPTER_UPSTREAM_MODEL sets the default native model override when no config file is used
provider config supports api_key or api_key_env
route config supports source_api, model, provider, provider_api, and native_model
```

Anthropic path coverage:

```text
unified.Request -> Anthropic MessageRequest
HTTP byte-stream request construction
raw SSE event block parsing
Anthropic wire event decoding
Anthropic wire event -> unified.Event mapping
text streaming
tool-use streaming with argument deltas
usage and finish-reason mapping
unified.Collect(...)
fake transport integration tests
live Anthropic smoke test through unified.Client
```

Gateway path coverage:

```text
OpenAI Chat Completions HTTP request -> unified.Request
unified.Event -> OpenAI Chat Completions non-streaming JSON response
unified.Event -> OpenAI Chat Completions SSE chunks
Anthropic Messages HTTP request -> unified.Request
unified.Event -> Anthropic Messages non-streaming JSON response
unified.Event -> Anthropic Messages SSE chunks
OpenAI Responses HTTP request -> unified.Request
unified.Event -> OpenAI Responses non-streaming JSON response
unified.Event -> OpenAI Responses SSE chunks
minimal gateway handler with configured unified.Client
endpoint-shaped errors before response start
runnable Anthropic-backed /v1/chat/completions gateway command
live gateway smoke tests for streaming and non-streaming requests
gateway route selection through router.StaticRouter
native model rewrite before provider client invocation
route results preserve provider endpoint metadata: target API kind, compatibility family, provider name, and capabilities
same OpenAI Chat endpoint smoke-tested against Anthropic and OpenAI upstreams
same Anthropic Messages endpoint smoke-tested against Anthropic, OpenRouter Messages, and MiniMax Messages upstreams
same OpenAI Responses endpoint smoke-tested against OpenRouter Responses upstream
shared unified.Client tool-use smoke tests pass against Anthropic and OpenAI
shared unified.Client tool-result continuation smoke tests pass against Anthropic and OpenAI
shared unified.Client text smoke tests pass across Anthropic, OpenAI Chat, OpenRouter Chat, OpenRouter Responses, and OpenRouter Messages
OpenRouter Chat, Responses, and Messages pass shared tool-use and tool-result continuation smokes
OpenRouter Responses routes through the OpenAI Chat gateway smoke path via canonical text conversion
MiniMax Chat uses the OpenAI-compatible stream path and is registered in text and gateway smoke matrices
MiniMax Messages uses the Anthropic-compatible stream path and is registered in text, tool, continuation, and gateway smoke matrices
```

Live e2e defaults:

```text
TEST_INTEGRATION=1 enables live e2e tests
ANTHROPIC_API_KEY provides Anthropic credentials
ANTHROPIC_MODEL overrides the default Anthropic smoke-test model
default Anthropic smoke-test model: claude-haiku-4-5-20251001
OPENAI_API_KEY or OPENAI_KEY provides OpenAI credentials
OPENAI_MODEL overrides the default OpenAI smoke-test model
default OpenAI smoke-test model: gpt-4.1-mini
OPENROUTER_API_KEY or OPENROUTER_KEY provides OpenRouter credentials
OPENROUTER_MODEL overrides the default OpenRouter smoke-test model
default OpenRouter smoke-test model: openai/gpt-4.1-mini
OPENROUTER_RESPONSES_MODEL overrides the default OpenRouter Responses smoke-test model
default OpenRouter Responses smoke-test model: openai/gpt-4.1-mini
OPENROUTER_MESSAGES_MODEL overrides the default OpenRouter Messages smoke-test model
default OpenRouter Messages smoke-test model: anthropic/claude-sonnet-4
MINIMAX_API_KEY or MINIMAX_KEY provides MiniMax credentials
MINIMAX_MODEL overrides the default MiniMax smoke-test model
default MiniMax smoke-test model: MiniMax-M2.7
MINIMAX_MESSAGES_MODEL overrides the default MiniMax Messages smoke-test model
default MiniMax Messages smoke-test model: MiniMax-M2.7
```

Known follow-up gaps:

```text
Provider mappings and endpoint codecs now cover common unsupported dropped fields with warnings; more specialized fields still need broader warning/extension coverage
Anthropic request mapping covers the phase-3 vertical slice, not the full Messages API
non-streaming Anthropic response bodies are not yet modeled separately from stream events
SSE parser intentionally skips empty dispatches; revisit if an endpoint needs exact spec-level empty event behavior
Raw/unmapped event preservation is minimal and should be expanded before gateway work
router is static and now includes capability checks plus deterministic weighted ranking; gateway retries pre-response provider failures, but there is no probabilistic load balancing, active health scoring, or capability conversion policy yet
gateway config is intentionally minimal; routes can disambiguate same-provider endpoints with provider_api, but there is no full registry yet
OpenAI provider is stream-first and covers smoke-tested text and tool-use paths
OpenRouter Chat Completions provider reuses the OpenAI-compatible stream path against OpenRouter's native chat endpoint
OpenRouter Responses provider is stream-first and covers smoke-tested text and function-call tool loops
OpenRouter Messages provider reuses the Anthropic-compatible stream path against OpenRouter's native messages endpoint
MiniMax Chat provider reuses the OpenAI-compatible stream path against MiniMax's /v1/chat/completions endpoint
MiniMax Messages provider reuses the Anthropic-compatible stream path against MiniMax's /anthropic/v1/messages endpoint
MiniMax Chat is currently marked text-streaming capable only; MiniMax Messages is the first MiniMax endpoint advertised as tool-capable
OpenAI-backed gateway route is smoke-tested for streaming and non-streaming responses
OpenAI Chat endpoint mapping is a compatibility slice, not full API coverage
Provider support is currently strong for text + function-tool loops, OpenAI-family structured-output requests, and basic image input passthrough; broad multimodal/media conformance is not complete
OpenRouter extension passthrough is raw JSON preservation; extension schemas and validation are intentionally deferred
streaming provider errors after response start need a final policy
runnable gateway uses one Anthropic route and can optionally override upstream model via env
```

Implementation assessment:

```text
Foundation is solid for a vertical-slice adapter: canonical request/event model, stream-first provider clients, deterministic weighted routing, pre-response gateway fallback, fake transport unit tests, and live outside-in e2e tests are all working.
Main intentional shortcuts are hardcoded provider construction in the gateway command, stream-first provider paths, and minimal warning/raw-event preservation.
Current live tests are good smoke coverage, not full conformance coverage.
Important remaining test gaps: invalid credentials/models, active health scoring, parallel tool calls, malformed tool args, deeper endpoint-codec conformance, broader reasoning/citations conformance, full audio/video/file provider conformance, and provider-specific extension schema validation.
```

Next planned phase:

```text
MiniMax provider continuation: validate MiniMax Chat tool-use/tool-result continuation before advertising tools
Endpoint continuation: expand downstream endpoint conformance beyond the minimal text/tool slices
```

MiniMax research notes:

```text
Official docs describe Text Generation through Anthropic SDK (recommended) and OpenAI SDK.
Anthropic-compatible base URL: https://api.minimax.io/anthropic
Anthropic-compatible endpoint: /anthropic/v1/messages
OpenAI-compatible base URL: https://api.minimax.io/v1
OpenAI-compatible endpoint: /v1/chat/completions
For llmadapter's existing OpenAI-compatible client wrapper, configure base URL as https://api.minimax.io because the client appends /v1/chat/completions.
Supported text models include MiniMax-M2.7, MiniMax-M2.7-highspeed, MiniMax-M2.5, MiniMax-M2.5-highspeed, MiniMax-M2.1, MiniMax-M2.1-highspeed, and MiniMax-M2.
Anthropic-compatible text supports streaming, system, max_tokens, temperature, top_p, tools, tool_choice, metadata, and thinking.
Anthropic-compatible messages support text, tool_use, tool_result, and thinking; image/document inputs are not supported yet.
MiniMax also offers speech, video, image, music, and file APIs; these are out of scope for the current text-first llmadapter provider path.
Sources:
https://platform.minimax.io/docs/api-reference/api-overview
https://platform.minimax.io/docs/api-reference/text-anthropic-api
https://platform.minimax.io/docs/api-reference/text-openai-api
https://platform.minimax.io/docs/api-reference/text-chat-anthropic
https://platform.minimax.io/docs/api-reference/text-chat
```

MiniMax implementation plan:

```text
1. Add adapt API kinds if/when implementation starts:
   - minimax.anthropic_messages -> family anthropic.messages (implemented)
   - minimax.chat_completions -> family openai.chat_completions (implemented)
2. Add providers/minimax/messages as an Anthropic-compatible wrapper over providers/anthropic/messages. (implemented)
   - Base URL: https://api.minimax.io/anthropic
   - Credential env: MINIMAX_API_KEY or MINIMAX_KEY
   - Default model: MiniMax-M2.7
   - Capabilities: streaming, tools, reasoning/thinking; no vision/document input initially
3. Add shared live e2e entries:
   - TestSmokeTextStream/minimax_messages
   - TestSmokeToolUse/minimax_messages
   - TestSmokeToolResultContinuation/minimax_messages (implemented and live-verified)
4. Add providers/minimax/chatcompletions as an OpenAI-compatible wrapper over providers/openai/chatcompletions. (implemented)
   - Base URL: https://api.minimax.io
   - Default model: MiniMax-M2.7
   - Validate text/tool streaming behavior before setting Tools: true
5. Register gateway provider types:
   - minimax_messages (implemented)
   - minimax_chat (implemented)
6. Update README and PLAN with MiniMax env vars and verification commands. (implemented for minimax_chat)
```

---

## Target outcome

At the end of phases 1-3, this must work:

```text
unified.Request
  -> Anthropic Messages request codec
  -> HTTP byte-stream transport
  -> SSE frame decoding
  -> Anthropic wire events
  -> unified.Event stream
  -> unified.Collect(...)
```

The public surface at that point is intentionally small:

```text
unified.Request
unified.Event
unified.Client
providers/anthropic/messages.NewClient(...)
```

Out of scope for these phases:

```text
HTTP gateway endpoints
router / model registry / fallback routing
multi-provider support
OpenAI Responses / Chat compatibility handlers
coreprovider shared package
gateway package
router package
websocket transport
provider-to-provider relays
JSON-schema emulation across providers
```

---

## Working rules

Every step must:

```text
compile without forward references
include focused unit tests
leave the repo in a buildable state
prefer stdlib only
```

Implementation order matters more than package purity in the first pass. If a shared abstraction is only used by Anthropic in phases 1-3, keep it local unless the abstraction is already clearly stable.

---

## Locked decisions for phases 1-3

These decisions remove ambiguity from the original plan and should be treated as fixed unless implementation proves them wrong.

### 1. Public error signaling

Inside the pipeline, use `pipeline.Item[T]` to propagate errors.

At the public `unified.Client` boundary, convert pipeline errors into a final `unified.ErrorEvent` and close the channel.

### 2. SSE transport contract

`transport.ByteStream` remains byte-oriented.

For `FrameFormatSSE`, `transport.HTTPByteStreamTransport` returns one complete raw SSE event block per `Recv`, not just the `data:` payload. This avoids losing the upstream `event:` field, which Anthropic requires to dispatch event types correctly.

`transport/sse.go` therefore owns both:

```text
splitting a response stream into SSE event blocks
parsing one raw SSE block into transport.SSEFrame
```

### 3. Retry and rate limiting live in `transport`

Phase 2 includes:

```text
RetryMode
RetryTransport
RateLimiter interface
RateLimitedTransport wrapper
```

Anthropic phase 3 only needs `WithTransport(...)`. Exposing dedicated rate-limit/retry options can wait until phase 4+.

### 4. No `coreprovider` package yet

`DESIGN.md` describes a future shared provider config package. Do not introduce it in phases 1-3.

For the first implementation, Anthropic owns its local:

```text
Config
Option
HeaderFunc
default base URL / version
processor wiring
```

If the pattern survives phase 4, extract it later.

### 5. First-pass lossiness policy

For Anthropic in phase 3:

```text
strict mode: return UnsupportedFieldError
best_effort mode: append warnings and skip unsupported fields
```

Do not emulate unsupported semantics yet. In particular:

```text
ResponseFormat JSON schema: warn/error only
Seed: warn/error only
provider-specific reasoning controls: extension or warn/error only
```

### 6. Vertical-slice priority

Phase 3 should be implemented in this order:

```text
text-only streaming
usage + completion mapping
tool-use streaming
public constructor
integration coverage
```

Do not start with the full feature matrix.

---

## Phase 1: Core IR and stream pipeline

Goal: define the canonical types and processor pipeline needed by a single provider path. No transport, no HTTP gateway, no router.

### Step 1.1: `unified` content, messages, tools, response config

Create:

```text
unified/content.go
unified/message.go
unified/tool.go
unified/response_format.go
```

Types:

```text
ContentKind
ContentPart
TextPart
ImagePart
AudioPart
VideoPart
FilePart
ReasoningPart
RefusalPart
BlobSourceKind
BlobSource

Role
Message
InstructionKind
Instruction

ToolKind
Tool
ToolChoiceMode
ToolChoice
ToolCall
ToolResult

ResponseFormatKind
ResponseFormat
ReasoningEffort
ReasoningConfig
SafetyConfig
```

Tests:

```text
content parts report the correct kind
public zero values marshal sensibly where relevant
```

Notes:

```text
keep `ContentPart` and `Event` sealed with unexported marker methods
use pointers for optional scalar request fields later
do not add provider-specific fields here
```

### Step 1.2: `unified.Extensions`

Create `unified/extensions.go`.

API:

```text
type Extensions struct
(*Extensions).Set(key string, value any) error
(Extensions).Has(key string) bool
(Extensions).Keys() []string
GetExtension[T](Extensions, key string) (T, bool, error)
```

Extension keys to add now:

```text
ExtOpenAIPreviousResponseID
ExtOpenAIStore
ExtAnthropicBetas
ExtGeminiSafetySettings
ExtOpenRouterProviderPrefs
ExtOllamaOptions
```

Tests:

```text
roundtrip for scalar and struct values
missing key returns (zero, false, nil)
type mismatch returns error
Keys returns sorted keys
nil value marshals as JSON null
```

### Step 1.3: `unified.Request`, `APIError`, `Client`

Create:

```text
unified/request.go
unified/errors.go
unified/client.go
```

Types:

```text
Request
APIError
Client
```

Tests:

```text
APIError satisfies error
APIError.Error() is stable and useful
errors.As works through wrapping
```

### Step 1.4: `unified.Event` model

Create `unified/event.go`.

Event set:

```text
MessageStartEvent
MessageDoneEvent
ContentBlockStartEvent
ContentBlockDoneEvent
TextDeltaEvent
ReasoningDeltaEvent
RefusalDeltaEvent
ToolCallStartEvent
ToolCallArgsDeltaEvent
ToolCallDoneEvent
CitationEvent
UsageEvent
CompletedEvent
WarningEvent
RawEvent
ErrorEvent
```

Support types:

```text
Citation
FinishReason
```

Tests:

```text
every event type satisfies the Event interface
```

Notes:

```text
keep `RawEvent` minimal: API kind, type, JSON bytes, optional decoded value
do not add endpoint-specific chunk types
```

### Step 1.5: `unified.Response` and `Collect`

Create:

```text
unified/usage.go
unified/response.go
```

API:

```text
Response
Collect(ctx, <-chan Event) (Response, error)
```

Tests:

```text
empty stream returns zero response
text-only stream assembles a single text block
tool-call stream assembles tool calls
usage is captured from UsageEvent
CompletedEvent sets finish reason
WarningEvents accumulate
RawEvents accumulate
ErrorEvent returns error
cancelled context returns ctx.Err()
mixed text + tool-call stream assembles correctly
```

### Step 1.6: `adapt` request envelope and codec interfaces

Create:

```text
adapt/types.go
adapt/request.go
adapt/codec.go
```

Types:

```text
ApiKind
ApiFamily
MappingMode
Warning
UnsupportedFieldError
HTTPRequestInfo
Request

EventDecoder[In, Out]
EventEncoder[In, Out]
ProviderCodec[Req, Evt]
RequestProcessor
ProviderRequestProcessor[Req]
NativeClient[Req, Evt]
```

Tests:

```text
UnsupportedFieldError satisfies error
```

Notes:

```text
define `NativeClient` here because phase 3 needs it
do not add endpoint codec interfaces yet; those are phase 4 concerns
```

### Step 1.7: generic pipeline primitives

Create:

```text
pipeline/processor.go
pipeline/chain.go
pipeline/transform.go
```

API:

```text
Processor[E]
Chain[E]
NewChain[E](...)
Item[T]
Transform[In, Out](...)
```

Behavior:

```text
Chain.Push runs left-to-right
Chain.Close cascades close-generated events through downstream processors
Transform drains input, forwards decoder output, flushes decoder on close
Transform emits Item.Err on decoder error or ctx cancellation
```

Tests:

```text
empty chain passthrough
filtering processor
expanding processor
two processors compose
Close cascade works
Push error stops chain
Close error propagates

Transform forwards all decoded values
decoder.Close runs when input closes
Push errors become Item.Err
Close errors become Item.Err
ctx cancellation stops transform
slow consumer does not lose values
```

### Step 1.8: built-in unified event processors

Create:

```text
pipeline/coalesce.go
pipeline/filter.go
pipeline/inject.go
```

Processors:

```text
TextCoalescer
ReasoningFilter
CompletionInjector
```

Tests:

```text
TextCoalescer buffers and flushes by threshold
TextCoalescer flushes before non-text events
TextCoalescer flushes on Close
ReasoningFilter drops reasoning when disabled
CompletionInjector injects one completion if none was seen
CompletionInjector does not double-inject
```

### Phase 1 checkpoint

Verify:

```text
go build ./unified/... ./adapt/... ./pipeline/...
go test ./unified/... ./adapt/... ./pipeline/...
go vet ./...
```

Done criteria:

```text
canonical request/event model is stable enough to support one provider
pipeline errors are representable without leaking implementation details
no package requires transport or provider code to compile
```

---

## Phase 2: Transport foundation

Goal: provide a reusable transport layer for streaming provider clients, with SSE support aligned to Anthropic's event model.

### Step 2.1: transport core types

Create `transport/transport.go`.

Types:

```text
Request
ByteStreamTransport
ByteStream
FrameFormat
FrameDecoder[Evt]
RateLimiter
```

Notes:

```text
`Request.Body` should be `io.Reader`
`Request.Extensions` can be `map[string]any`
`RateLimiter` should be a tiny interface: `Wait(context.Context) error`
```

### Step 2.2: SSE framing and parsing

Create `transport/sse.go`.

Types / functions:

```text
SSEFrame
SSEReader
NewSSEReader(io.Reader) *SSEReader
(*SSEReader).Next() ([]byte, error)         // raw SSE event block
ParseSSEFrame([]byte) (SSEFrame, error)     // parse one block
```

Required behavior:

```text
split on blank-line event boundaries
support `event:`, `data:`, `id:`, `retry:`
join multi-line data with `\n`
ignore comment lines
accept `\n`, `\r\n`, and `\r`
handle malformed `field` lines as empty-value fields
return io.EOF cleanly at stream end
```

Tests:

```text
single event
multiple events
multi-line data
event type present
id and retry fields
comment lines skipped
mixed line endings
empty data field
malformed field line
consecutive blank lines
EOF behavior
```

### Step 2.3: NDJSON framing

Create `transport/ndjson.go`.

Types:

```text
NDJSONReader
NewNDJSONReader(io.Reader) *NDJSONReader
(*NDJSONReader).Next() ([]byte, error)
```

Behavior:

```text
return one non-empty line at a time
skip empty lines
support configurable max line size, default 1MB
return io.EOF at end
```

Tests:

```text
single line
multiple lines
empty lines skipped
trailing whitespace
large line under limit
line over limit errors
EOF behavior
```

### Step 2.4: HTTP byte-stream transport

Create `transport/http.go`.

Types:

```text
HTTPTransportConfig
HTTPByteStreamTransport
NewHTTPByteStreamTransport(...)
```

Behavior:

```text
Open sends an HTTP request through http.Client
non-2xx returns *unified.APIError with status and body preview
SSE mode returns one raw SSE event block per Recv
NDJSON mode returns one line per Recv
Raw mode returns full body once, then io.EOF
Close closes the underlying response body
Recv respects ctx cancellation
```

Tests using `httptest.Server`:

```text
SSE stream yields multiple raw event blocks
NDJSON stream yields multiple lines
Raw mode returns one full body
non-2xx maps to APIError
request method, URL, headers, and body are correct
Close closes the body
ctx cancellation interrupts Recv
```

### Step 2.5: fake transport for unit and integration tests

Create `transport/fake.go`.

Types:

```text
FakeByteStreamTransport
fakeByteStream
```

Behavior:

```text
capture seen requests
allow Open error
allow frame-by-frame error injection
return io.EOF when frames are exhausted
```

Tests:

```text
frames returned in order
error at configured frame
Open error returned
seen requests captured
empty stream returns EOF
```

### Step 2.6: rate-limit wrapper

Create `transport/ratelimit.go`.

Types:

```text
RateLimitedTransport
NewRateLimitedTransport(inner ByteStreamTransport, limiter RateLimiter) *RateLimitedTransport
```

Behavior:

```text
call limiter.Wait(ctx) before inner.Open
do nothing when limiter is nil
propagate limiter errors directly
```

Tests:

```text
Wait called before Open
limiter error stops Open
nil limiter delegates directly
```

### Step 2.7: retry wrapper

Create `transport/retry.go`.

Types:

```text
RetryMode
RetryConfig
RetryTransport
NewRetryTransport(inner ByteStreamTransport, cfg RetryConfig) *RetryTransport
```

Behavior:

```text
Mode=never delegates directly
Mode=before_stream retries Open failures and retryable pre-stream API errors
default retryable statuses: 429, 500, 502, 503, 504
never retry after the first successful Recv
respect ctx cancellation during backoff
```

Tests:

```text
never mode performs no retries
Open error retries and later succeeds
retryable APIError retries
non-retryable APIError does not retry
MaxRetries is enforced
ctx cancellation during backoff returns ctx.Err()
```

### Phase 2 checkpoint

Verify:

```text
go build ./transport/...
go test ./transport/...
go vet ./...
```

Done criteria:

```text
Anthropic native client can be built entirely on transport abstractions
SSE event type information is preserved end-to-end
retry and rate-limit behavior are testable without provider code
```

---

## Phase 3: One complete provider path - Anthropic Messages

Goal: ship one real provider path from `unified.Request` to `unified.Event`.

Implementation order inside phase 3 is strict:

```text
3A text-only request/response path
3B usage + completion + tool-use support
3C public constructor and end-to-end tests
```

### Step 3.1: Anthropic wire types

Create:

```text
providers/anthropic/messages/wire.go
providers/anthropic/messages/wire_test.go
```

Request-side types:

```text
MessageRequest
InputMessage
ContentBlock
ToolDefinition
ToolChoiceWire
Metadata
```

Response/event-side types:

```text
Event
MessageStartEvent
MessageResponse
ContentBlockStartEvent
ContentBlockDeltaEvent
ContentBlockStopEvent
MessageDeltaEvent
MessageDeltaBody
MessageStopEvent
PingEvent
ErrorEventWire
APIErrorBody
UsageWire
```

Tests:

```text
request JSON roundtrip: text
request JSON roundtrip: system instructions
request JSON roundtrip: tools
request JSON roundtrip: images
response/event JSON roundtrip
omitempty behavior
```

### Step 3.2: Anthropic request codec (`unified` -> wire)

Create:

```text
providers/anthropic/messages/codec.go
providers/anthropic/messages/codec_test.go
testdata/anthropic/request_*.json
```

Codec responsibilities:

```text
implement adapt.ProviderCodec[MessageRequest, Event]
encode unified.Request into MessageRequest
create a new wire-event decoder later via NewEventDecoder()
```

Mapping rules for first pass:

```text
Model -> Model
MaxOutputTokens -> MaxTokens (required)
Messages -> Messages
Instructions -> System
Temperature / TopP / TopK / Stop -> direct mappings
Tools -> Tools (function tools only)
ToolChoice -> ToolChoice
Stream -> Stream
```

Lossiness rules:

```text
strict: error on missing MaxOutputTokens, unsupported tool kinds, Seed, ResponseFormat, unsupported content
best_effort: append warnings and drop unsupported fields
```

Implementation order:

```text
first: text-only user/assistant messages
then: system instructions
then: tools + tool results
then: image blocks
```

Tests:

```text
basic text request fixture
multi-turn fixture
system instruction fixture
tool definition fixture
tool result fixture
image fixture
missing MaxOutputTokens errors
strict vs best_effort behavior
```

### Step 3.3: Anthropic SSE frame decoder (raw SSE block -> wire event)

Create:

```text
providers/anthropic/messages/sse.go
providers/anthropic/messages/sse_test.go
```

API:

```text
SSEFrameDecoder implementing transport.FrameDecoder[Event]
```

Behavior:

```text
ParseSSEFrame on each raw block
dispatch using SSEFrame.Event
decode JSON from SSEFrame.Data
support message_start, content_block_start, content_block_delta,
content_block_stop, message_delta, message_stop, ping, error
```

Policy:

```text
unknown SSE event type -> error in phase 3
empty ping payload is allowed
malformed JSON returns error
```

Tests:

```text
each supported event type parses correctly
tool_use and input_json_delta cases parse correctly
unknown event type errors
bad JSON errors
```

### Step 3.4: Anthropic event decoder (wire event -> `unified.Event`)

Create:

```text
providers/anthropic/messages/decoder.go
providers/anthropic/messages/decoder_test.go
testdata/anthropic/events_*.ndjson
```

API:

```text
EventDecoder implementing adapt.EventDecoder[Event, unified.Event]
```

State to track:

```text
open content blocks by index
tool-call IDs and names
buffered tool-call argument fragments
message metadata needed for final completion mapping
```

Required mappings:

```text
message_start -> MessageStartEvent
content_block_start(text) -> ContentBlockStartEvent
content_block_delta(text_delta) -> TextDeltaEvent
content_block_stop(text) -> ContentBlockDoneEvent
message_delta -> CompletedEvent and optional UsageEvent
message_stop -> MessageDoneEvent
ping -> dropped
error -> unified.ErrorEvent wrapping unified.APIError
```

Additional mappings for 3B:

```text
tool_use block start -> ContentBlockStartEvent + ToolCallStartEvent
input_json_delta -> ToolCallArgsDeltaEvent
tool block stop -> ToolCallDoneEvent + ContentBlockDoneEvent
thinking deltas -> ReasoningDeltaEvent
stop_reason mapping:
  end_turn -> stop
  max_tokens -> length
  tool_use -> tool_call
  stop_sequence -> stop
```

Close behavior:

```text
error if a tool-call buffer is incomplete
otherwise flush nothing
```

Tests:

```text
plain text stream fixture
multi-block text fixture
tool-use fixture
parallel tool-call fixture
reasoning fixture
usage mapping
stop-reason mapping
ping dropped
error event mapped
incomplete tool buffer errors on Close
```

### Step 3.5: Anthropic native client

Create:

```text
providers/anthropic/messages/client.go
providers/anthropic/messages/client_test.go
```

API:

```text
NativeClient implementing adapt.NativeClient[MessageRequest, Event]
```

Responsibilities:

```text
marshal MessageRequest to JSON
construct POST {baseURL}/v1/messages
set content-type, x-api-key, anthropic-version
apply static headers and HeaderFuncs
open the transport
read raw SSE blocks from ByteStream
decode blocks via SSEFrameDecoder
emit wire events on a channel
close stream resources on exit
```

Tests using `transport.FakeByteStreamTransport`:

```text
correct URL and method
required headers set
custom headers and HeaderFuncs applied
request body is valid JSON
decoded wire events emitted in order
transport errors propagate
ctx cancellation stops the stream loop
```

### Step 3.6: adapted Anthropic client (`unified.Client`)

Create:

```text
providers/anthropic/messages/adapted.go
providers/anthropic/messages/adapted_test.go
```

Responsibilities:

```text
wrap unified.Request in adapt.Request
run request processors
encode with codec
run provider request processors
call NativeClient
run provider-event processor chain
decode to unified.Event
run unified-event processor chain
convert internal errors to a final unified.ErrorEvent
```

Tests:

```text
request processors can mutate adapt.Request
provider request processors can mutate MessageRequest
provider event processors compose before decoding
unified event processors compose after decoding
internal errors are surfaced as ErrorEvent
```

### Step 3.7: Anthropic public constructor

Create:

```text
providers/anthropic/messages/options.go
providers/anthropic/messages/options_test.go
```

Local provider API only:

```text
type HeaderFunc func(context.Context, *http.Request) error
type Option interface { applyAnthropic(*Config) error }
type Config struct { ... }
```

Options to implement now:

```text
WithAPIKey(string)
WithBaseURL(string)
WithVersion(string)
WithBeta(string)
WithHeader(key, value string)
WithHeaderFunc(HeaderFunc)
WithTransport(transport.ByteStreamTransport)
WithRequestProcessor(adapt.RequestProcessor)
WithUnifiedEventProcessor(pipeline.Processor[unified.Event])
WithProviderRequestProcessor(adapt.ProviderRequestProcessor[MessageRequest])
WithProviderEventProcessor(pipeline.Processor[Event])
```

Constructor:

```text
NewClient(opts ...Option) (unified.Client, error)
```

Behavior:

```text
require API key
apply default base URL and Anthropic version
default transport to HTTPByteStreamTransport in SSE mode
wire all processors into the adapted client
```

Tests:

```text
API key required
base URL override works
version override works
beta header appended
custom transport replaces default
processors are wired into the right stage
```

### Step 3.8: end-to-end integration coverage

Create `providers/anthropic/messages/integration_test.go`.

Coverage:

```text
text response stream -> unified events -> Collect response
tool-use stream -> unified tool-call events -> Collect response
provider error stream -> ErrorEvent -> Collect returns error
TextCoalescer integration
CompletionInjector integration
```

Use `transport.FakeByteStreamTransport` with raw SSE blocks as inputs.

### Phase 3 checkpoint

Verify:

```text
go build ./...
go test ./...
go vet ./...
```

Done criteria:

```text
a caller can construct an Anthropic client and stream unified events
text-only and tool-use responses are covered
the phase 3 path does not depend on gateway or router code
all failures surface as normal Go errors or unified.ErrorEvent
```

---

## Immediate implementation order

This is the recommended coding order across the first three phases.

### Milestone A: core compile spine

Build in this order:

```text
1.1 unified core types
1.2 extensions
1.3 request + APIError + client
1.4 events
1.5 response + Collect
1.6 adapt types
1.7 pipeline primitives
1.8 built-in processors
```

Exit condition:

```text
`go test ./unified/... ./adapt/... ./pipeline/...` passes
```

### Milestone B: transport spine

Build in this order:

```text
2.1 transport types
2.2 SSE framing/parsing
2.3 NDJSON
2.4 HTTP transport
2.5 fake transport
2.6 rate-limit wrapper
2.7 retry wrapper
```

Exit condition:

```text
raw SSE event blocks can be read and parsed without provider code
```

### Milestone C: Anthropic text-only path

Build in this order:

```text
3.1 wire types
3.2 request codec for text/system only
3.3 SSE frame decoder
3.4 event decoder for text + usage + completion only
3.5 native client
3.6 adapted client
3.7 NewClient
3.8 integration test for text streaming
```

Exit condition:

```text
simple text prompt roundtrip works end to end
```

### Milestone D: Anthropic tool-use completion

Extend:

```text
3.2 tool definitions + tool results
3.4 tool-use event mapping + argument buffering + reasoning deltas
3.8 tool-use integration coverage
```

Exit condition:

```text
tool calls survive the roundtrip into unified.Response
```

---

## Dependency summary

```text
Phase 1
  1.1 core unified types
  1.2 extensions                <- 1.1
  1.3 request/error/client      <- 1.1, 1.2
  1.4 events                    <- 1.1
  1.5 response/collect          <- 1.1, 1.4
  1.6 adapt types/interfaces    <- 1.3, 1.4
  1.7 pipeline primitives       <- 1.6
  1.8 built-in processors       <- 1.4, 1.7

Phase 2
  2.1 transport types
  2.2 SSE framing/parsing       <- 2.1
  2.3 NDJSON                    <- 2.1
  2.4 HTTP transport            <- 2.1, 2.2, 2.3, 1.3
  2.5 fake transport            <- 2.1
  2.6 rate-limit wrapper        <- 2.1
  2.7 retry wrapper             <- 2.1, 1.3

Phase 3
  3.1 Anthropic wire types
  3.2 request codec             <- 3.1, 1.3, 1.6
  3.3 SSE frame decoder         <- 3.1, 2.2
  3.4 event decoder             <- 3.1, 1.4, 1.6
  3.5 native client             <- 3.1, 3.3, 2.4
  3.6 adapted client            <- 3.2, 3.4, 3.5, 1.7, 1.8
  3.7 constructor               <- 3.5, 3.6
  3.8 integration               <- 3.7, 2.5, 1.5
```
