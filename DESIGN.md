# llmadapter Design

## Status

Final draft for the core design of `llmadapter`.

This document describes the core interfaces, canonical request/event model, provider routing model, stream pipeline, transport abstraction, option pattern, unmapped-event handling, and testing strategy.

The design intentionally excludes true realtime bidirectional APIs for now. A WebSocket may still be used as a unidirectional event-stream transport, equivalent in role to SSE, NDJSON, chunked JSON, or a fake test frame stream.

---

## 1. Purpose

`llmadapter` provides a unified Go client and gateway layer for multiple upstream LLM API families.

Known target API families include:

- OpenAI Responses
- OpenAI Chat Completions
- OpenAI legacy Completions
- Anthropic Messages
- Gemini GenerateContent / streaming APIs
- Ollama native Generate / Chat
- Ollama OpenAI-compatible APIs
- OpenRouter Chat Completions / Responses / Anthropic-compatible APIs
- Amazon Bedrock Converse / ConverseStream
- Mistral Chat
- Cohere Chat v2
- other OpenAI-compatible APIs

The central design objective is to avoid `M × N` direct adapters.

Instead of mapping every external API directly to every other API, every API kind maps to and from a canonical intermediate representation:

```text
endpoint wire API <-> adapt layer <-> unified.Request / unified.Event <-> provider wire API
```

---

## 2. Primary usage modes

### 2.1 Programmatic client mode

Application code creates a `unified.Request`, sends it to a `unified.Client`, and consumes `unified.Event` values.

```go
type Client interface {
    Request(ctx context.Context, req unified.Request) (<-chan unified.Event, error)
}
```

### 2.2 HTTP compatibility gateway mode

An HTTP server exposes compatibility endpoints such as:

```text
/v1/responses
/v1/chat/completions
/v1/completions
/v1/messages
/api/chat
/api/generate
```

Incoming requests are decoded into the canonical representation, routed to an upstream provider endpoint, then encoded back into the downstream endpoint's expected wire format.

---

## 3. Non-goals

The core design does not attempt to make every provider feature look identical.

Provider APIs differ in meaningful ways:

- some support server-side state;
- some support previous response IDs;
- some support reasoning controls or reasoning deltas;
- some support built-in web/file/code tools;
- some support citations or grounding metadata;
- some support prompt caching;
- some use top-level system instructions;
- some use role-based system messages;
- some stream tool-call arguments incrementally;
- some only return final tool-call arguments;
- some return usage only at the end;
- some use SSE, NDJSON, chunked JSON, or WebSocket messages.

Therefore, the design must support:

- explicit capability discovery;
- explicit lossiness reporting;
- provider-specific extensions;
- strict versus best-effort mapping modes;
- stateful event transformation;
- raw/unmapped event preservation where requested.

The following are explicitly out of scope for the core design:

- multi-turn conversation/session management (callers manage message history);
- automatic token counting or context window management;
- multiple choices per request (`n > 1`) as a first-class unified concept;
- metadata and auxiliary endpoints such as `/v1/models` or `/api/tags` (these may be added as simple handlers later but are not part of the core pipeline design);
- true realtime bidirectional APIs.

---

## 4. Core terminology

### API kind

An exact API implementation shape.

```go
type ApiKind string

const (
    ApiOpenAIResponses       ApiKind = "openai.responses"
    ApiOpenAIChatCompletions ApiKind = "openai.chat_completions"
    ApiAnthropicMessages     ApiKind = "anthropic.messages"
    ApiGeminiGenerateContent ApiKind = "gemini.generate_content"
    ApiOllamaChat            ApiKind = "ollama.chat"
    ApiOllamaGenerate        ApiKind = "ollama.generate"
    ApiOpenRouterChatCompletions   ApiKind = "openrouter.chat_completions"
    ApiOpenRouterResponses         ApiKind = "openrouter.responses"
    ApiOpenRouterAnthropicMessages ApiKind = "openrouter.anthropic_messages"
    ApiBedrockConverse             ApiKind = "bedrock.converse"
    ApiMistralChat                 ApiKind = "mistral.chat"
    ApiCohereChatV2                ApiKind = "cohere.chat_v2"
)
```

### API family

A compatibility family or protocol family.

This distinguishes an exact implementation from a compatible protocol surface.

```go
type ApiFamily string

const (
    FamilyOpenAIResponses       ApiFamily = "openai.responses"
    FamilyOpenAIChatCompletions ApiFamily = "openai.chat_completions"
    FamilyAnthropicMessages     ApiFamily = "anthropic.messages"
    FamilyGeminiGenerateContent ApiFamily = "gemini.generate_content"
    FamilyOllamaChat            ApiFamily = "ollama.chat"
    FamilyBedrockConverse       ApiFamily = "bedrock.converse"
)
```

Example:

```go
ProviderEndpoint{
    ProviderName: "openrouter",
    APIKind:      ApiOpenRouterResponses,
    Family:       FamilyOpenAIResponses,
}
```

OpenRouter Responses may be compatible with the OpenAI Responses family, but it is still a distinct implementation kind.

### Provider

A logical upstream service or runtime.

Examples:

```text
openai-prod
anthropic-prod
openrouter-prod
ollama-local
bedrock-us-east-1
vertex-project-x
```

### Provider endpoint

A concrete callable API surface exposed by a provider.

A single provider can expose multiple provider endpoints.

Example:

```text
provider: openrouter
  endpoint: openrouter.chat_completions
  endpoint: openrouter.responses
  endpoint: openrouter.anthropic_messages
```

### Endpoint

A downstream HTTP compatibility surface exposed by `llmadapter`.

Examples:

```text
/v1/responses
/v1/chat/completions
/v1/messages
/api/chat
```

### Wire type

A provider-specific or endpoint-specific Go struct mirroring an external API.

Examples:

```text
openairesponses.Request
anthropicmessages.MessageRequest
gemini.GenerateContentRequest
ollama.ChatRequest
```

### Unified type

The canonical semantic representation used by the core pipeline.

Examples:

```text
unified.Request
unified.Event
unified.Message
unified.Tool
```

### Adapt type

A richer gateway envelope around unified types. It carries HTTP metadata, raw bodies, decoded wire payloads, original API kind, mapping warnings, routing metadata, and extension data.

Examples:

```text
adapt.Request
adapt.Warning
adapt.MappingMode
```

---

## 5. High-level pipelines

### 5.1 Programmatic client path

```text
caller unified.Request
  -> adapt.Request wrapper
  -> unified request processors
  -> provider request codec
  -> provider wire request processors
  -> native provider client
  -> provider.Event stream
  -> provider wire event processors
  -> provider event decoder
  -> unified.Event stream
  -> unified event processors
  -> caller
```

### 5.2 HTTP gateway path

```text
HTTP request
  -> endpoint decoder
  -> adapt.Request
  -> unified request processors
  -> router
  -> provider endpoint selection
  -> provider request codec
  -> provider wire request processors
  -> native provider client
  -> provider.Event stream
  -> provider wire event processors
  -> provider event decoder
  -> unified.Event stream
  -> unified event processors
  -> endpoint event encoder / HTTP writer
  -> HTTP response
```

### 5.3 Optional passthrough path

Passthrough is an optimization, not the foundation.

```text
HTTP request body
  -> optional inspection / auth / model rewrite
  -> upstream HTTP request body

upstream response stream
  -> optional byte/SSE/frame filter
  -> downstream HTTP response stream
```

Same nominal API family does not guarantee exact compatibility. Passthrough should only be selected by an explicit route mode.

---

## 6. Package layout

Suggested layout:

```text
llmadapter/
  unified/
    request.go
    event.go
    message.go
    content.go
    tool.go
    response_format.go
    usage.go
    extensions.go
    client.go

  adapt/
    request.go
    codec.go
    loss.go
    stream.go
    processors.go

  pipeline/
    processor.go
    transform.go
    chain.go
    coalesce.go

  transport/
    bytestream.go
    http.go
    websocket.go
    fake.go
    retry.go
    ratelimit.go

  coreprovider/
    config.go
    options.go
    headers.go
    client.go

  router/
    router.go
    registry.go
    modelmap.go
    capabilities.go

  gateway/
    http.go
    sse.go
    ndjson.go
    errors.go

  providers/
    openai/responses/
    openai/chatcompletions/
    anthropic/messages/
    gemini/generatecontent/
    ollama/chat/
    ollama/generate/
    openrouter/
    bedrock/converse/
    mistral/chat/
    cohere/chatv2/

  endpoints/
    openairesponses/
    openaichatcompletions/
    anthropicmessages/
    ollamachat/
```

---

## 7. Canonical request model

The canonical request should be rich enough for modern multimodal, tool-using, streaming APIs without being tied to one provider.

```go
package unified

import "encoding/json"

type Request struct {
    Model string

    Messages     []Message
    Instructions []Instruction

    MaxOutputTokens *int
    Temperature     *float64
    TopP            *float64
    TopK            *int
    Stop            []string
    Seed            *int64

    ResponseFormat *ResponseFormat

    Tools      []Tool
    ToolChoice *ToolChoice

    Reasoning *ReasoningConfig
    Safety    *SafetyConfig

    Stream bool
    User   string

    Extensions Extensions
}
```

Use pointers for optional scalar values. For example, `Temperature *float64` can distinguish unset from explicitly zero.

---

## 8. Request extensions

Provider-specific semantic parameters should not pollute the core request model, but they must still be available to programmatic users.

Use a typed extension bag.

```go
type Extensions struct {
    values map[string]json.RawMessage
}

func (e *Extensions) Set(key string, value any) error {
    if e.values == nil {
        e.values = map[string]json.RawMessage{}
    }

    b, err := json.Marshal(value)
    if err != nil {
        return err
    }

    e.values[key] = b
    return nil
}

func GetExtension[T any](e Extensions, key string) (T, bool, error) {
    var zero T

    raw, ok := e.values[key]
    if !ok {
        return zero, false, nil
    }

    var out T
    if err := json.Unmarshal(raw, &out); err != nil {
        return zero, true, err
    }

    return out, true, nil
}
```

Use namespaced extension keys.

```go
const (
    ExtOpenAIPreviousResponseID = "openai.responses.previous_response_id"
    ExtOpenAIStore              = "openai.responses.store"
    ExtAnthropicBetas           = "anthropic.betas"
    ExtGeminiSafetySettings     = "gemini.safety_settings"
    ExtOpenRouterModels         = "openrouter.models"
    ExtOpenRouterRoute          = "openrouter.route"
    ExtOpenRouterProvider       = "openrouter.provider"
    ExtOpenRouterProviderPrefs  = "openrouter.provider_preferences"
    ExtOpenRouterPlugins        = "openrouter.plugins"
    ExtOpenRouterDebug          = "openrouter.debug"
    ExtOpenRouterTrace          = "openrouter.trace"
    ExtOpenRouterSessionID      = "openrouter.session_id"
    ExtOllamaOptions            = "ollama.options"
)
```

Example:

```go
req := unified.Request{
    Model: "gpt-4.1",
    Messages: []unified.Message{
        {
            Role: unified.RoleUser,
            Content: []unified.ContentPart{
                unified.TextPart{Text: "Continue from the previous response."},
            },
        },
    },
}

_ = req.Extensions.Set(
    unified.ExtOpenAIPreviousResponseID,
    "resp_123",
)
```

The OpenAI Responses codec may consume that extension. Other codecs may ignore it, warn, or error depending on mapping mode.

---

## 9. Instructions and messages

System/developer instructions should be separated from normal messages because providers disagree on where these live.

```go
type InstructionKind string

const (
    InstructionSystem    InstructionKind = "system"
    InstructionDeveloper InstructionKind = "developer"
    InstructionPolicy    InstructionKind = "policy"
)

type Instruction struct {
    Kind    InstructionKind
    Content []ContentPart
    Name    string
    Meta    map[string]any
}
```

Messages:

```go
type Role string

const (
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleSystem    Role = "system"
    RoleTool      Role = "tool"
)

type Message struct {
    Role Role
    ID   string
    Name string

    Content []ContentPart

    ToolCalls   []ToolCall
    ToolResults []ToolResult

    Meta map[string]any
}
```

---

## 10. Content parts

Use typed content parts. Do not represent everything as plain strings.

```go
type ContentKind string

const (
    ContentText       ContentKind = "text"
    ContentImage      ContentKind = "image"
    ContentAudio      ContentKind = "audio"
    ContentVideo      ContentKind = "video"
    ContentFile       ContentKind = "file"
    ContentDocument   ContentKind = "document"
    ContentReasoning  ContentKind = "reasoning"
    ContentRefusal    ContentKind = "refusal"
)

type ContentPart interface {
    contentKind() ContentKind
}

type TextPart struct {
    Text string
}

func (TextPart) contentKind() ContentKind { return ContentText }

type ImagePart struct {
    Source   BlobSource
    MimeType string
    Detail   string
}

func (ImagePart) contentKind() ContentKind { return ContentImage }

type FilePart struct {
    Source   BlobSource
    MimeType string
    Filename string
}

func (FilePart) contentKind() ContentKind { return ContentFile }

type ReasoningPart struct {
    Text      string
    Signature string
    Encrypted []byte
}

func (ReasoningPart) contentKind() ContentKind { return ContentReasoning }

type RefusalPart struct {
    Text string
}

func (RefusalPart) contentKind() ContentKind { return ContentRefusal }

type AudioPart struct {
    Source   BlobSource
    MimeType string
}

func (AudioPart) contentKind() ContentKind { return ContentAudio }

type VideoPart struct {
    Source   BlobSource
    MimeType string
}

func (VideoPart) contentKind() ContentKind { return ContentVideo }
```

Tool calls and tool results are not content kinds. The canonical location for tool calls in messages is `Message.ToolCalls` and `Message.ToolResults`. Content parts represent user-provided or model-generated content blocks.

Blob sources:

```go
type BlobSourceKind string

const (
    BlobSourceURL    BlobSourceKind = "url"
    BlobSourceBase64 BlobSourceKind = "base64"
    BlobSourceBytes  BlobSourceKind = "bytes"
    BlobSourceFileID BlobSourceKind = "file_id"
)

type BlobSource struct {
    Kind   BlobSourceKind
    URL    string
    Base64 string
    Bytes  []byte
    FileID string
}
```

---

## 11. Tools

```go
type ToolKind string

const (
    ToolFunction    ToolKind = "function"
    ToolWebSearch   ToolKind = "web_search"
    ToolFileSearch  ToolKind = "file_search"
    ToolCodeExec    ToolKind = "code_exec"
    ToolComputerUse ToolKind = "computer_use"
    ToolCustom      ToolKind = "custom"
)

type Tool struct {
    Kind        ToolKind
    Name        string
    Description string

    InputSchema json.RawMessage

    Config map[string]any
}

type ToolChoiceMode string

const (
    ToolChoiceAuto     ToolChoiceMode = "auto"
    ToolChoiceNone     ToolChoiceMode = "none"
    ToolChoiceRequired ToolChoiceMode = "required"
    ToolChoiceSpecific ToolChoiceMode = "specific"
)

type ToolChoice struct {
    Mode ToolChoiceMode
    Name string
}

type ToolCall struct {
    ID        string
    Name      string
    Arguments json.RawMessage
    Index     int
}

type ToolResult struct {
    ToolCallID string
    Name       string
    Content    []ContentPart
    IsError    bool
}
```

Tool-call argument deltas should be represented as bytes until complete. Partial JSON fragments are not necessarily valid JSON.

---

## 12. Response format, reasoning, and safety

```go
type ResponseFormatKind string

const (
    ResponseFormatText       ResponseFormatKind = "text"
    ResponseFormatJSON       ResponseFormatKind = "json"
    ResponseFormatJSONSchema ResponseFormatKind = "json_schema"
)

type ResponseFormat struct {
    Kind   ResponseFormatKind
    Schema json.RawMessage
    Name   string
    Strict bool
}
```

```go
type ReasoningEffort string

const (
    ReasoningEffortLow    ReasoningEffort = "low"
    ReasoningEffortMedium ReasoningEffort = "medium"
    ReasoningEffortHigh   ReasoningEffort = "high"
)

type ReasoningConfig struct {
    Effort    *ReasoningEffort
    MaxTokens *int
    Expose    bool

    Extensions map[string]any
}
```

```go
type SafetyConfig struct {
    Policies   []string
    Extensions map[string]any
}
```

---

## 13. Canonical event model

The event model represents a semantic stream, not a specific provider's SSE chunks.

Important properties:

- supports many-to-many mapping;
- supports partial text deltas;
- supports message and content block lifecycle events;
- supports incremental tool-call arguments;
- supports reasoning deltas;
- supports citations and annotations;
- supports usage events;
- supports final completion state;
- supports warnings, raw events, and errors.

```go
type Event interface {
    isEvent()
}
```

### Message lifecycle

```go
type MessageStartEvent struct {
    ID    string
    Model string
    Role  Role
    Time  time.Time
}

func (MessageStartEvent) isEvent() {}

type MessageDoneEvent struct {
    ID string
}

func (MessageDoneEvent) isEvent() {}
```

### Content lifecycle and deltas

```go
type ContentBlockStartEvent struct {
    Index int
    Kind  ContentKind
    ID    string
    Name  string
}

func (ContentBlockStartEvent) isEvent() {}

type ContentBlockDoneEvent struct {
    Index int
    Kind  ContentKind
}

func (ContentBlockDoneEvent) isEvent() {}

type TextDeltaEvent struct {
    Index int
    Text  string
}

func (TextDeltaEvent) isEvent() {}

type ReasoningDeltaEvent struct {
    Index int
    Text  string
}

func (ReasoningDeltaEvent) isEvent() {}

type RefusalDeltaEvent struct {
    Index int
    Text  string
}

func (RefusalDeltaEvent) isEvent() {}
```

### Tool-call events

```go
type ToolCallStartEvent struct {
    Index int
    ID    string
    Name  string
}

func (ToolCallStartEvent) isEvent() {}

type ToolCallArgsDeltaEvent struct {
    Index int
    ID    string
    Delta []byte
}

func (ToolCallArgsDeltaEvent) isEvent() {}

type ToolCallDoneEvent struct {
    Index int
    ID    string
    Name  string
    Args  json.RawMessage
}

func (ToolCallDoneEvent) isEvent() {}
```

### Citations and annotations

```go
type CitationEvent struct {
    Index    int
    Citation Citation
}

func (CitationEvent) isEvent() {}

type Citation struct {
    Type        string
    Text        string
    URL         string
    Title       string
    StartOffset int
    EndOffset   int
    DocumentID  string
    Meta        map[string]any
}
```

### Usage and completion

```go
type UsageEvent struct {
    InputTokens      int
    OutputTokens     int
    ReasoningTokens  int
    CacheReadTokens  int
    CacheWriteTokens int
    TotalTokens      int

    ProviderRaw any
}

func (UsageEvent) isEvent() {}

type FinishReason string

const (
    FinishReasonStop          FinishReason = "stop"
    FinishReasonLength        FinishReason = "length"
    FinishReasonToolCall      FinishReason = "tool_call"
    FinishReasonContentFilter FinishReason = "content_filter"
    FinishReasonError         FinishReason = "error"
    FinishReasonUnknown       FinishReason = "unknown"
)

type CompletedEvent struct {
    FinishReason FinishReason
    MessageID     string
}

func (CompletedEvent) isEvent() {}
```

### Warnings, raw events, and errors

```go
type WarningEvent struct {
    Code    string
    Message string
    Source  string
    Meta    map[string]any
}

func (WarningEvent) isEvent() {}

type RawEvent struct {
    APIKind ApiKind
    Type    string

    JSON  json.RawMessage
    Value any
}

func (RawEvent) isEvent() {}

type ErrorEvent struct {
    Err         error
    Recoverable bool
}

func (ErrorEvent) isEvent() {}
```

### Event ordering contract

A well-formed unified event stream follows this ordering:

```text
MessageStartEvent
  ContentBlockStartEvent (index 0)
    TextDeltaEvent* | ReasoningDeltaEvent* | RefusalDeltaEvent*
    CitationEvent*
  ContentBlockDoneEvent (index 0)
  ContentBlockStartEvent (index 1)
    ToolCallStartEvent
    ToolCallArgsDeltaEvent*
    ToolCallDoneEvent
  ContentBlockDoneEvent (index 1)
  ...
  UsageEvent
  CompletedEvent
MessageDoneEvent
```

`CompletedEvent` carries the finish reason and is emitted before `MessageDoneEvent`. `MessageDoneEvent` is the final lifecycle event for a message. A stream must emit at most one `CompletedEvent` and one `MessageDoneEvent`.

Provider event decoders should synthesize missing lifecycle events where possible. The `CompletionInjector` processor (§20) handles the case where a provider stream ends without an explicit completion signal.

### Audio and video output events

Audio and video output streaming events are deferred. The `CapabilitySet` declares `AudioOutput` for future use. When needed, `AudioDeltaEvent` and `VideoDeltaEvent` event types will be added following the same delta pattern.

### Structured API errors

LLM provider APIs return rich structured errors. The unified layer should capture these.

```go
type APIError struct {
    StatusCode int
    Type       string
    Code       string
    Message    string
    Param      string
    RetryAfter time.Duration

    ProviderRaw any
}

func (e *APIError) Error() string {
    return fmt.Sprintf("%s: %s (status %d)", e.Type, e.Message, e.StatusCode)
}
```

`APIError` satisfies `error` and can be carried by `ErrorEvent.Err`, returned from `NativeClient.Request`, or returned from `ByteStreamTransport.Open`. It preserves the original provider error in `ProviderRaw` for provider-specific error handling.

Transport errors, API errors, and codec/mapping errors are all representable as `error` values. Callers can use `errors.As` to extract `*APIError` when provider-level detail is needed.

---

## 14. Adapt request envelope

`adapt.Request` is used by the gateway and by adapted clients internally.

```go
type Request struct {
    SourceAPI ApiKind

    HTTP *HTTPRequestInfo

    RawBody []byte
    Raw     any

    Unified unified.Request

    MappingMode MappingMode
    Warnings    []Warning

    Extensions unified.Extensions
}

type HTTPRequestInfo struct {
    Method  string
    Path    string
    Query   map[string][]string
    Headers map[string][]string
    Remote  string
}
```

Programmatic users usually only create `unified.Request`. Gateway code creates `adapt.Request`.

`adapt.Request.Extensions` carries gateway-level and routing metadata (tenant identity, route overrides, audit context). `unified.Request.Extensions` carries provider-semantic parameters (previous response IDs, provider-specific safety settings, prompt caching hints). The two extension bags serve different audiences and should not be conflated.

---

## 15. Lossiness and mapping policy

```go
type MappingMode string

const (
    MappingStrict     MappingMode = "strict"
    MappingBestEffort MappingMode = "best_effort"
)

type Warning struct {
    Code    string
    Field   string
    Message string
}

type UnsupportedFieldError struct {
    APIKind ApiKind
    Field   string
    Reason  string
}
```

Rules:

- strict mode returns errors for unsupported semantics;
- best-effort mode may degrade behavior and append warnings;
- lossy mapping should never be silently hidden.

Examples of possibly lossy fields:

```text
previous_response_id
server-side response storage
provider-native safety settings
prompt cache controls
built-in tools
provider-specific reasoning controls
provider-specific routing preferences
citations/grounding metadata
```

---

## 16. Core interfaces

### App-facing client

```go
type Client interface {
    Request(ctx context.Context, req unified.Request) (<-chan unified.Event, error)
}
```

### Provider-native client

```go
type NativeClient[Req any, Evt any] interface {
    ApiKind() ApiKind
    Request(ctx context.Context, req Req) (<-chan Evt, error)
}
```

### Provider codec

```go
type ProviderCodec[ProviderReq any, ProviderEvt any] interface {
    ApiKind() ApiKind

    EncodeRequest(ctx context.Context, req adapt.Request) (ProviderReq, error)

    NewEventDecoder() EventDecoder[ProviderEvt, unified.Event]
}
```

`NewEventEncoder` is deliberately omitted from the core codec interface. The primary flow is provider events → unified events (decoding). The reverse direction (unified events → provider wire events) is only needed for endpoint response writing, and that is handled by `EndpointCodec.WriteEvents`, not by the provider codec. If a future use case requires encoding unified events into provider wire format (e.g., provider-to-provider relay), a separate `ProviderEventEncoder` interface can be introduced.

### Endpoint codec

```go
type EndpointCodec interface {
    ApiKind() ApiKind

    DecodeHTTP(ctx context.Context, r *http.Request) (adapt.Request, error)

    WriteEvents(
        ctx context.Context,
        w http.ResponseWriter,
        events <-chan unified.Event,
    ) error

    WriteError(ctx context.Context, w http.ResponseWriter, err error) error
}
```

`WriteEvents` must handle both streaming and non-streaming responses. For `stream: true`, it writes SSE/NDJSON frames as events arrive. For `stream: false`, it accumulates all events, builds a complete JSON response body, and writes it as `application/json`. The endpoint codec inspects the original request (available via the adapt request or context) to determine the response mode.

### Stateful event transformers

```go
type EventDecoder[In any, Out any] interface {
    Push(ctx context.Context, ev In) ([]Out, error)
    Close(ctx context.Context) ([]Out, error)
}

type EventEncoder[In any, Out any] interface {
    Push(ctx context.Context, ev In) ([]Out, error)
    Close(ctx context.Context) ([]Out, error)
}
```

### Same-type event processors

```go
type Processor[E any] interface {
    Push(ctx context.Context, ev E) ([]E, error)
    Close(ctx context.Context) ([]E, error)
}
```

---

## 17. Injection points

The pipeline exposes multiple intentional injection points.

### Unified request processors

Operate on `adapt.Request` before provider request encoding.

```go
type RequestProcessor interface {
    ProcessRequest(ctx context.Context, req *adapt.Request) error
}
```

Examples:

```text
model alias rewriting
default parameter injection
tenant policy checks
tool schema validation
capability validation
request logging
```

### Provider wire request processors

Operate after provider request encoding but before transport.

```go
type ProviderRequestProcessor[Req any] interface {
    ProcessProviderRequest(ctx context.Context, req *Req) error
}
```

Examples:

```text
set provider-native fields
set previous_response_id
set provider metadata
force stream=true
mutate provider-specific options
```

### Transport request processors

Operate on transport-level requests.

For HTTP-specific transports:

```go
type HTTPRequestProcessor interface {
    ProcessHTTPRequest(ctx context.Context, req *http.Request) error
}
```

Examples:

```text
static headers
dynamic headers
auth headers
beta headers
trace headers
idempotency keys
custom user agent
```

### Provider wire event processors

Operate on provider events before decoding into unified events.

```go
type ProviderEventProcessor[Evt any] interface {
    Push(ctx context.Context, ev Evt) ([]Evt, error)
    Close(ctx context.Context) ([]Evt, error)
}
```

Examples:

```text
provider-specific event logging
stream repair
raw metrics
provider event filtering
```

### Unified event processors

Operate after provider events become `unified.Event`.

```go
type UnifiedEventProcessor interface {
    Push(ctx context.Context, ev unified.Event) ([]unified.Event, error)
    Close(ctx context.Context) ([]unified.Event, error)
}
```

Examples:

```text
text delta coalescing
reasoning filtering
tool-call validation
usage accumulation
synthetic completion injection
citation normalization
policy filtering
```

### Endpoint event processors

For HTTP gateway mode, endpoint codecs may own additional processors right before writing downstream events.

Examples:

```text
suppress events unsupported by Chat Completions
encode citations into metadata
inject final usage chunks
shape stream into endpoint-specific lifecycle
```

---

## 18. Stream transformation helper

### Error signaling convention

`Transform` uses `Item[T]` internally for error propagation within the pipeline. The public `Client.Request` interface returns a bare `<-chan unified.Event`. Errors in the public channel are signaled by emitting an `ErrorEvent` as the last event before closing the channel. Pipeline internals use `Item[T]` and convert to `ErrorEvent` at the boundary.

### Backpressure and goroutine lifecycle

The `Transform` helper spawns a goroutine that reads from the input channel. Backpressure is applied naturally: if the consumer stops reading from the output channel, the producer goroutine blocks on send. When context is cancelled, the producer goroutine exits via the `ctx.Done()` select case.

Callers must either consume all events from the returned channel or cancel the context. Failure to do either will leak the producer goroutine. The `ByteStream.Close()` method ensures the underlying transport resources (HTTP response body, WebSocket connection) are released when the pipeline shuts down.

```go
type Item[T any] struct {
    Value T
    Err   error
}

func Transform[In any, Out any](
    ctx context.Context,
    in <-chan In,
    decoder adapt.EventDecoder[In, Out],
) <-chan Item[Out] {
    out := make(chan Item[Out])

    go func() {
        defer close(out)

        emit := func(values []Out) bool {
            for _, value := range values {
                select {
                case <-ctx.Done():
                    return false
                case out <- Item[Out]{Value: value}:
                }
            }
            return true
        }

        for {
            select {
            case <-ctx.Done():
                out <- Item[Out]{Err: ctx.Err()}
                return

            case ev, ok := <-in:
                if !ok {
                    flushed, err := decoder.Close(ctx)
                    if err != nil {
                        out <- Item[Out]{Err: err}
                        return
                    }
                    emit(flushed)
                    return
                }

                values, err := decoder.Push(ctx, ev)
                if err != nil {
                    out <- Item[Out]{Err: err}
                    return
                }

                if !emit(values) {
                    return
                }
            }
        }
    }()

    return out
}
```

---

## 19. Processor chain

```go
type Chain[E any] struct {
    processors []Processor[E]
}

func NewChain[E any](processors ...Processor[E]) *Chain[E] {
    return &Chain[E]{processors: processors}
}

func (c *Chain[E]) Push(ctx context.Context, ev E) ([]E, error) {
    current := []E{ev}

    for _, p := range c.processors {
        next := make([]E, 0, len(current))

        for _, item := range current {
            produced, err := p.Push(ctx, item)
            if err != nil {
                return nil, err
            }
            next = append(next, produced...)
        }

        current = next
        if len(current) == 0 {
            break
        }
    }

    return current, nil
}
```

A production `Close` implementation should cascade close-generated events through downstream processors.

---

## 20. Example processors

### Text coalescer

Buffers small text deltas and emits larger deltas.

```go
type TextCoalescer struct {
    MaxBytes int
    buf      strings.Builder
}

func (p *TextCoalescer) Push(ctx context.Context, ev unified.Event) ([]unified.Event, error) {
    delta, ok := ev.(unified.TextDeltaEvent)
    if !ok {
        flushed := p.flush()
        if len(flushed) == 0 {
            return []unified.Event{ev}, nil
        }
        return append(flushed, ev), nil
    }

    p.buf.WriteString(delta.Text)
    if p.buf.Len() >= p.MaxBytes {
        return p.flush(), nil
    }

    return nil, nil
}

func (p *TextCoalescer) Close(ctx context.Context) ([]unified.Event, error) {
    return p.flush(), nil
}

func (p *TextCoalescer) flush() []unified.Event {
    if p.buf.Len() == 0 {
        return nil
    }

    text := p.buf.String()
    p.buf.Reset()
    return []unified.Event{unified.TextDeltaEvent{Text: text}}
}
```

Production note: buffer by content block index.

### Reasoning filter

```go
type ReasoningFilter struct {
    Expose bool
}

func (p ReasoningFilter) Push(ctx context.Context, ev unified.Event) ([]unified.Event, error) {
    if _, ok := ev.(unified.ReasoningDeltaEvent); ok && !p.Expose {
        return nil, nil
    }
    return []unified.Event{ev}, nil
}

func (p ReasoningFilter) Close(ctx context.Context) ([]unified.Event, error) {
    return nil, nil
}
```

### Completion injector

```go
type CompletionInjector struct {
    seenCompleted bool
}

func (p *CompletionInjector) Push(ctx context.Context, ev unified.Event) ([]unified.Event, error) {
    if _, ok := ev.(unified.CompletedEvent); ok {
        p.seenCompleted = true
    }
    return []unified.Event{ev}, nil
}

func (p *CompletionInjector) Close(ctx context.Context) ([]unified.Event, error) {
    if p.seenCompleted {
        return nil, nil
    }
    return []unified.Event{
        unified.CompletedEvent{FinishReason: unified.FinishReasonUnknown},
    }, nil
}
```

---

## 21. Adapted client implementation

An adapted client wraps a provider-native client and provider codec while exposing the stable `unified.Client` interface.

```go
type AdaptedClient[ProviderReq any, ProviderEvt any] struct {
    Native adapt.NativeClient[ProviderReq, ProviderEvt]
    Codec  adapt.ProviderCodec[ProviderReq, ProviderEvt]

    RequestProcessors []adapt.RequestProcessor

    ProviderRequestProcessors []adapt.ProviderRequestProcessor[ProviderReq]
    ProviderEventProcessors   []pipeline.Processor[ProviderEvt]

    UnifiedEventProcessors []pipeline.Processor[unified.Event]
}
```

Conceptual flow:

```go
func (c *AdaptedClient[ProviderReq, ProviderEvt]) Request(
    ctx context.Context,
    req unified.Request,
) (<-chan unified.Event, error) {
    adaptReq := adapt.Request{
        SourceAPI:   adapt.ApiProgrammatic,
        Unified:     req,
        MappingMode: adapt.MappingBestEffort,
    }

    for _, p := range c.RequestProcessors {
        if err := p.ProcessRequest(ctx, &adaptReq); err != nil {
            return nil, err
        }
    }

    providerReq, err := c.Codec.EncodeRequest(ctx, adaptReq)
    if err != nil {
        return nil, err
    }

    for _, p := range c.ProviderRequestProcessors {
        if err := p.ProcessProviderRequest(ctx, &providerReq); err != nil {
            return nil, err
        }
    }

    providerEvents, err := c.Native.Request(ctx, providerReq)
    if err != nil {
        return nil, err
    }

    // provider event processors
    // provider event decoder
    // unified event processors
    // output channel
}
```

---

## 22. Provider event decoder example

A provider event decoder must be stateful.

One provider event can produce zero, one, or many unified events. Multiple provider events may be required before one unified event can be emitted.

```go
type AnthropicEventDecoder struct {
    toolBuffers map[int][]byte
    toolNames   map[int]string
    toolIDs     map[int]string
}

func (d *AnthropicEventDecoder) Push(
    ctx context.Context,
    ev anthropic.Event,
) ([]unified.Event, error) {
    switch ev := ev.(type) {
    case anthropic.MessageStartEvent:
        return []unified.Event{
            unified.MessageStartEvent{
                ID:    ev.Message.ID,
                Model: ev.Message.Model,
                Role:  unified.RoleAssistant,
            },
        }, nil

    case anthropic.ContentBlockDeltaEvent:
        switch ev.Delta.Type {
        case "text_delta":
            return []unified.Event{
                unified.TextDeltaEvent{
                    Index: ev.Index,
                    Text:  ev.Delta.Text,
                },
            }, nil

        case "input_json_delta":
            d.toolBuffers[ev.Index] = append(
                d.toolBuffers[ev.Index],
                []byte(ev.Delta.PartialJSON)...,
            )

            return []unified.Event{
                unified.ToolCallArgsDeltaEvent{
                    Index: ev.Index,
                    ID:    d.toolIDs[ev.Index],
                    Delta: []byte(ev.Delta.PartialJSON),
                },
            }, nil
        }

    case anthropic.ContentBlockStopEvent:
        if args, ok := d.toolBuffers[ev.Index]; ok {
            id := d.toolIDs[ev.Index]
            name := d.toolNames[ev.Index]

            delete(d.toolBuffers, ev.Index)
            delete(d.toolIDs, ev.Index)
            delete(d.toolNames, ev.Index)

            return []unified.Event{
                unified.ToolCallDoneEvent{
                    Index: ev.Index,
                    ID:    id,
                    Name:  name,
                    Args:  json.RawMessage(args),
                },
            }, nil
        }
    }

    return nil, nil
}

func (d *AnthropicEventDecoder) Close(ctx context.Context) ([]unified.Event, error) {
    if len(d.toolBuffers) > 0 {
        return nil, fmt.Errorf("stream ended with incomplete tool buffers")
    }
    return nil, nil
}
```

---

## 23. Multi-API-kind providers

A provider can expose multiple API kinds.

Therefore, routing targets provider endpoints, not providers.

Each `ProviderEndpoint` holds a `unified.Client`, which erases the generic type parameters (`ProviderReq`, `ProviderEvt`). This is intentional: the router and registry operate at the unified semantic level. Provider-specific type information is encapsulated inside the `AdaptedClient` implementation.

```go
type ProviderEndpoint struct {
    ProviderName string

    APIKind ApiKind
    Family  ApiFamily

    Client unified.Client

    Capabilities CapabilitySet

    Priority int
    Tags     map[string]string
}
```

Example:

```go
registry.Register(router.ProviderEndpoint{
    ProviderName: "openrouter",
    APIKind:      adapt.ApiOpenRouterResponses,
    Family:       adapt.FamilyOpenAIResponses,
    Client:       openrouterResponsesClient,
    Priority:     100,
})

registry.Register(router.ProviderEndpoint{
    ProviderName: "openrouter",
    APIKind:      adapt.ApiOpenRouterChatCompletions,
    Family:       adapt.FamilyOpenAIChatCompletions,
    Client:       openrouterChatClient,
    Priority:     90,
})

registry.Register(router.ProviderEndpoint{
    ProviderName: "openrouter",
    APIKind:      adapt.ApiOpenRouterAnthropicMessages,
    Family:       adapt.FamilyAnthropicMessages,
    Client:       openrouterMessagesClient,
    Priority:     80,
})
```

Route result:

```go
type Route struct {
    Mode RouteMode

    SourceAPI ApiKind
    TargetAPI ApiKind
    TargetFamily ApiFamily

    ProviderName string

    PublicModel string
    NativeModel string

    Client unified.Client

    Capabilities CapabilitySet
}
```

Route modes:

```go
type RouteMode string

const (
    RouteModeCanonical   RouteMode = "canonical"
    RouteModePassthrough RouteMode = "passthrough"
)
```

Routing should consider:

```text
source API family
requested model
native model mapping
required capabilities
provider priority
tenant/provider policy
strict versus best-effort mode
```

---

## 24. Capability set

```go
type CapabilitySet struct {
    Streaming bool

    Tools         bool
    ParallelTools bool

    Vision      bool
    AudioInput  bool
    AudioOutput bool

    JSONMode   bool
    JSONSchema bool

    Reasoning       bool
    ReasoningDeltas bool

    Citations bool

    BuiltInWebSearch  bool
    BuiltInFileSearch bool
    CodeExecution     bool
    ComputerUse       bool

    ServerSideState bool
    PromptCaching   bool

    MaxInputTokens  int
    MaxOutputTokens int
}
```

Capabilities should influence routing and strict-mode validation.

The struct-of-bools approach is deliberate. Adding a new capability requires a code change, which is acceptable because new capabilities typically require new codec logic, new event types, or new mapping rules. A dynamic `map[string]bool` would allow extension without code changes but would lose type safety and make capability checks error-prone.

---

## 25. Transport abstraction

Realtime duplex is out of scope.

However, WebSocket can be used as a unidirectional event-stream transport. From the pipeline's perspective, this is equivalent to SSE, NDJSON, chunked JSON, or fake test frames.

The core abstraction should therefore be byte-frame streaming.

For non-streaming requests (`stream: false`), the same `ByteStreamTransport` is used. A non-streaming HTTP response is modeled as a single-frame byte stream: `Open` returns a `ByteStream` whose first `Recv` returns the complete response body, and whose second `Recv` returns `io.EOF`. This keeps the pipeline uniform.

```go
package transport

type Request struct {
    Method string
    URL    string
    Header http.Header
    Body   io.Reader

    Extensions map[string]any
}

type ByteStreamTransport interface {
    Open(ctx context.Context, req *Request) (ByteStream, error)
}

type ByteStream interface {
    Recv(ctx context.Context) ([]byte, error)
    Close() error
}
```

Implementations:

```text
HTTPByteStreamTransport      // SSE, NDJSON, chunked JSON
WebSocketByteStreamTransport // unidirectional WebSocket messages
FakeByteStreamTransport      // tests
```

Provider-specific frame decoders turn byte frames into provider events.

```go
type FrameDecoder[Evt any] interface {
    PushFrame(ctx context.Context, frame []byte) ([]Evt, error)
    Close(ctx context.Context) ([]Evt, error)
}
```

Native client flow:

```text
provider request
  -> marshal to transport.Request
  -> ByteStreamTransport.Open
  -> ByteStream.Recv frames
  -> provider frame decoder
  -> provider.Event stream
```

---

## 26. Shared transport concerns

Transport middleware owns the reusable non-provider-specific work:

```text
auth headers
static headers
dynamic headers
beta headers
trace headers
rate limiting
retry before stream start
status error decoding hook
observability
body closing
context cancellation
```

Retries must be conservative for streaming requests.

Usually safe:

```text
connection failed before request body was sent
HTTP 429/500/502/503/504 before any stream frames were emitted
```

Usually unsafe:

```text
stream already emitted tokens
tool call already started
request may have provider-side state or side effects
```

Suggested retry modes:

```go
type RetryMode string

const (
    RetryNever        RetryMode = "never"
    RetryBeforeStream RetryMode = "before_stream"
)
```

---

## 27. Provider construction options

Provider constructors should use type-safe functional options, but without repeating large config structs for every provider.

Recommended public API:

```go
client, err := anthropic.NewUnifiedClient(
    anthropic.WithAPIKey("..."),
    anthropic.WithHeader("x-my-header", "abc"),
    anthropic.WithHeaderFunc(func(ctx context.Context, r *http.Request) error {
        r.Header.Set("x-request-id", requestIDFromContext(ctx))
        return nil
    }),
    anthropic.WithTransport(myByteStreamTransport),
    anthropic.WithUnifiedEventProcessor(&pipeline.TextCoalescer{MaxRunes: 256}),

    anthropic.WithVersion("2023-06-01"),
    anthropic.WithBeta("tools-2024-04-04"),
)
```

For OpenRouter provider-level construction:

```go
or, err := openrouter.NewProvider(
    openrouter.WithAPIKey("..."),
    openrouter.WithHeader("X-Title", "myapp"),
    openrouter.WithChat(),
    openrouter.WithResponses(),
    openrouter.WithAnthropicMessages(),
)
```

### Recommended option implementation

Use provider-local type-safe option interfaces.

```go
package anthropic

type Option interface {
    applyAnthropic(*Config) error
}

type Config struct {
    Core coreprovider.Config

    Version string
    Betas   []string

    ProviderRequestProcessors []adapt.ProviderRequestProcessor[MessageRequest]
    ProviderEventProcessors   []pipeline.Processor[Event]
}
```

Provider-specific option:

```go
type betaOption struct {
    beta string
}

func WithBeta(beta string) Option {
    return betaOption{beta: beta}
}

func (o betaOption) applyAnthropic(c *Config) error {
    c.Betas = append(c.Betas, o.beta)
    return nil
}
```

Shared option behavior is delegated to `coreprovider` helpers, but exposed through provider-local wrappers for type safety and clean package boundaries.

```go
type apiKeyOption struct {
    key string
}

func WithAPIKey(key string) Option {
    return apiKeyOption{key: key}
}

func (o apiKeyOption) applyAnthropic(c *Config) error {
    return coreprovider.ApplyAPIKey(&c.Core, o.key)
}
```

This keeps:

```text
type safety
clear godoc per provider
no opts ...any
no ugly generic call sites
clean package dependency direction
```

The small provider-local wrapper duplication can later be generated.

---

## 28. Core provider config

```go
package coreprovider

type Config struct {
    BaseURL string
    APIKey  string

    Headers     http.Header
    HeaderFuncs []HeaderFunc

    Transport transport.ByteStreamTransport

    RequestProcessors []adapt.RequestProcessor

    UnifiedEventProcessors []pipeline.Processor[unified.Event]

    RetryMode   RetryMode
    RateLimiter RateLimiter

    Logger *slog.Logger
    Meter  Meter
    Tracer Tracer
}

type HeaderFunc func(ctx context.Context, req *http.Request) error
```

`Logger` is `*slog.Logger` from the standard library. `Meter` and `Tracer` are interfaces aligned with OpenTelemetry:

```go
type Meter interface {
    CounterAdd(ctx context.Context, name string, value int64, attrs ...string)
    HistogramRecord(ctx context.Context, name string, value float64, attrs ...string)
}

type Tracer interface {
    Start(ctx context.Context, name string) (context.Context, Span)
}

type Span interface {
    SetAttribute(key string, value any)
    RecordError(err error)
    End()
}
```

All three are optional. When nil, observability is silently disabled. The interfaces are intentionally minimal; production code can wrap OpenTelemetry's `metric.Meter` and `trace.Tracer` behind these interfaces.

Request-scoped metadata (request IDs, trace context, tenant identity) should flow through `context.Context` using standard Go patterns. The library does not define its own context keys; callers and `HeaderFunc` implementations use their own.

Shared helper functions:

```go
func ApplyAPIKey(c *Config, key string) error
func ApplyBaseURL(c *Config, url string) error
func ApplyHeader(c *Config, key, value string) error
func ApplyHeaderFunc(c *Config, fn HeaderFunc) error
func ApplyTransport(c *Config, t transport.ByteStreamTransport) error
func ApplyUnifiedEventProcessor(c *Config, p pipeline.Processor[unified.Event]) error
func ApplyRequestProcessor(c *Config, p adapt.RequestProcessor) error
```

Provider packages expose these as provider-local options.

---

## 29. Unmapped events

Unmapped events should be governed by explicit policy.

```go
type UnmappedPolicy string

const (
    UnmappedDrop  UnmappedPolicy = "drop"
    UnmappedWarn  UnmappedPolicy = "warn"
    UnmappedRaw   UnmappedPolicy = "raw"
    UnmappedError UnmappedPolicy = "error"
)
```

Two directions need policies:

```go
type EventMappingPolicy struct {
    UnmappedProviderEvents UnmappedPolicy
    UnmappedUnifiedEvents  UnmappedPolicy
}
```

### Provider event cannot be mapped to unified

Options:

```text
drop       // ignore known harmless events
warn       // emit WarningEvent or log warning
raw        // emit RawEvent
error      // fail stream
```

Recommended default:

```text
known harmless events: drop
unknown events: raw or warn in debug mode
strict mode: error
```

### Unified event cannot be encoded to endpoint format

Example:

```text
unified.CitationEvent -> OpenAI Chat Completions stream
```

Options:

```text
drop
warn
encode into endpoint-specific metadata
emit raw/debug event if endpoint supports it
error in strict mode
```

Endpoint codecs should own this policy because endpoint compatibility differs.

---

## 30. HTTP gateway handler

```go
type Handler struct {
    Endpoint adapt.EndpointCodec
    Router   router.Router
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    req, err := h.Endpoint.DecodeHTTP(ctx, r)
    if err != nil {
        h.Endpoint.WriteError(ctx, w, err)
        return
    }

    route, err := h.Router.Route(ctx, req)
    if err != nil {
        h.Endpoint.WriteError(ctx, w, err)
        return
    }

    req.Unified.Model = route.NativeModel

    events, err := route.Client.Request(ctx, req.Unified)
    if err != nil {
        h.Endpoint.WriteError(ctx, w, err)
        return
    }

    if err := h.Endpoint.WriteEvents(ctx, w, events); err != nil {
        // Response may already be partially written.
        // Log; do not assume a clean error response can still be sent.
        return
    }
}
```

A production handler may choose canonical or passthrough mode based on `route.Mode`.

---

## 31. Testing strategy

Testing should be fixture-heavy and layered. Avoid relying only on full end-to-end tests.

### Golden request mapping tests

Test:

```text
source wire request -> unified.Request
unified.Request -> provider wire request
```

Suggested fixtures:

```text
testdata/
  openai_responses/
    request_basic.input.json
    request_basic.unified.json
    request_tools.input.json
    request_tools.unified.json

  anthropic_messages/
    request_tools.unified.json
    request_tools.output.json
```

### Golden event stream tests

Represent event streams as NDJSON fixtures.

```text
testdata/anthropic/events_tool_use.input.ndjson
testdata/anthropic/events_tool_use.unified.ndjson
```

Test cases should cover:

```text
plain text stream
multi-block text
tool-call argument deltas
parallel tool calls
reasoning deltas
citations
usage at end
provider error
unknown event
incomplete tool buffer
missing completion event
```

### Roundtrip tests

Test semantic invariants, not byte-for-byte equality.

Examples:

```text
OpenAI request -> unified -> OpenAI request
OpenAI Responses request -> unified -> Anthropic request -> unified
Anthropic request -> unified -> OpenAI Chat request
```

Assert:

```text
model preserved or intentionally rewritten
messages preserved
tools preserved
important params preserved
extensions preserved when applicable
warnings emitted for lossy fields
strict mode errors where expected
```

### Fake byte stream transport

```go
type FakeByteStreamTransport struct {
    Frames [][]byte
    ErrAt  int

    SeenRequests []*transport.Request
}
```

Use it to test:

```text
correct URL
correct method
correct headers
correct body
auth injection
beta headers
frame parsing
status/error behavior
cancellation
body close
```

### Static native client

```go
type StaticNativeClient[Req any, Evt any] struct {
    Events []Evt
    Reqs   []Req
    Err    error
}

func (c *StaticNativeClient[Req, Evt]) Request(
    ctx context.Context,
    req Req,
) (<-chan Evt, error) {
    c.Reqs = append(c.Reqs, req)

    if c.Err != nil {
        return nil, c.Err
    }

    ch := make(chan Evt, len(c.Events))
    for _, ev := range c.Events {
        ch <- ev
    }
    close(ch)
    return ch, nil
}
```

Useful for testing:

```text
AdaptedClient
request processors
provider request processors
provider event processors
unified event processors
Close flushing
error conversion
```

### Processor unit tests

Every processor should be tested independently with `Push` and `Close`.

Examples:

```text
TextCoalescer emits nothing until threshold
TextCoalescer flushes before non-text event
ReasoningFilter drops reasoning when disabled
CompletionInjector emits completion on Close
ToolValidator errors on malformed JSON
UsageAccumulator emits final usage
```

### Fuzz tests

Fuzz high-risk streaming parts:

```text
partial JSON tool args
malformed JSON fragments
unexpected event order
missing stop events
duplicate tool IDs
large deltas
invalid UTF-8
unknown event types
SSE parser
NDJSON parser
WebSocket frame parser
```

### HTTP gateway integration tests

Use `httptest.Server`.

Test:

```text
/v1/chat/completions request
  -> endpoint decode
  -> fake router
  -> static unified client
  -> endpoint SSE writer
```

Assert endpoint-specific response shape.

### Live backend smoke tests

Live provider tests should exercise the public `unified.Client` entrypoint from outside the provider package.

They are intentionally environment-gated, not build-tag-gated:

```text
TEST_INTEGRATION=1
```

Provider credentials and model overrides are passed through provider-specific environment variables.

Example:

```text
ANTHROPIC_API_KEY=...
ANTHROPIC_MODEL=claude-haiku-4-5-20251001
```

Live smoke tests live under:

```text
tests/e2e/
```

They should use table-driven provider setup so the same smoke scenarios can run against additional providers with minimal boilerplate.

### Lossiness tests

For every unsupported feature:

```text
strict mode -> error
best-effort mode -> warning
```

This prevents silent degradation.

---

## 32. Default test matrix per provider

Before calling a provider integration stable, cover:

```text
Request mapping:
  basic text
  system/developer instructions
  multimodal input if supported
  tools
  tool results
  JSON mode/schema
  provider-specific extension

Event mapping:
  message start
  text delta
  reasoning delta if supported
  tool call start
  tool arg delta
  tool call done
  usage
  completion
  provider error
  unknown event
  incomplete stream

Transport:
  headers
  auth
  non-2xx error
  retry before stream
  no retry after stream start
  cancellation
  body/stream close

Gateway:
  non-streaming response
  streaming response
  endpoint-shaped error
  unsupported event policy
```

---

## 33. Implementation phases

### Phase 1: Core IR and stream pipeline

Implement:

```text
unified.Request
unified.Event
unified.Extensions
adapt.Request
adapt.EventDecoder / EventEncoder
pipeline.Transform
pipeline.Processor
pipeline.Chain
unified.Client
```

### Phase 2: Transport foundation

Implement:

```text
transport.ByteStreamTransport
transport.HTTPByteStreamTransport
transport.FakeByteStreamTransport
SSE frame parser
NDJSON frame parser
retry-before-stream policy
rate limiter interface
```

### Phase 3: One complete provider path

Implement one complete path first, for example:

```text
unified.Request -> Anthropic Messages -> unified.Event
```

### Phase 4: One HTTP compatibility endpoint

Implement:

```text
/v1/chat/completions -> unified -> Anthropic Messages -> unified -> Chat Completions SSE
```

This validates request decoding, tool mapping, streaming deltas, endpoint writing, and error handling.

### Phase 5: OpenAI Responses alignment

Implement:

```text
/v1/responses -> unified -> provider
provider -> unified -> OpenAI Responses SSE
```

This stress-tests semantic event mapping.

### Phase 6: Multi-provider router

Implement:

```text
provider endpoint registry
model aliases
native model rewriting
capability checks
fallback routing
strict/best-effort policy
```

### Phase 7: Multi-API-kind provider

Implement OpenRouter or another provider exposing multiple API kinds.

Validate:

```text
shared provider config
multiple provider endpoints
family-aware routing
capability-aware routing
```

---

## 34. Final design decisions

1. The core architecture uses a canonical IR, not `M × N` direct adapters.

2. Programmatic users interact primarily with `unified.Client`, `unified.Request`, and `unified.Event`.

3. HTTP gateway code uses `adapt.Request` to preserve raw request and endpoint metadata.

4. Request conversion is mostly one-shot.

5. Event conversion is stateful stream transformation.

6. Provider event decoders and endpoint event encoders use `Push` and `Close`.

7. The pipeline exposes explicit injection points for unified requests, provider wire requests, transport requests, provider wire events, unified events, and endpoint events.

8. Provider-specific semantic parameters are passed through `unified.Request.Extensions` using namespaced keys.

9. Per-request HTTP/gateway metadata belongs in `adapt.Request`, not `unified.Request`.

10. A provider may expose multiple API kinds.

11. Routing targets provider endpoints, not providers.

12. `ApiKind` identifies the exact implementation; `ApiFamily` identifies compatibility family.

13. WebSocket is allowed only as a unidirectional byte-frame event stream for now.

14. `ByteStreamTransport` is the shared abstraction for SSE, NDJSON, chunked JSON, WebSocket event streams, and fake test frames.

15. Static headers, auth, retries, rate limits, and observability are transport/provider configuration concerns.

16. Public provider constructors should use provider-local type-safe options.

17. Shared option behavior should live in `coreprovider`, while provider packages expose local wrappers.

18. Unmapped events are governed by explicit policy: drop, warn, raw, or error.

19. Lossy mapping must either error in strict mode or warn in best-effort mode.

20. Testing should use golden fixtures, fake byte streams, static native clients, roundtrip invariants, fuzzing, gateway integration tests, and explicit lossiness tests.

21. The unified layer models exactly one assistant output per request. `n > 1` is not a first-class concept.

22. `APIError` is the structured error type for provider API errors. It satisfies `error` and can be extracted with `errors.As`.

23. Non-streaming responses use the same `ByteStreamTransport` as streaming, modeled as a single-frame byte stream.

24. `EndpointCodec.WriteEvents` handles both streaming and non-streaming response modes.

25. Audio and video output events are deferred until provider support warrants them.

---

## 35. Non-streaming response collection

For programmatic callers who do not need streaming, a `unified.Response` type and `Collect` helper materialize an event stream into a single value.

```go
type Response struct {
    ID    string
    Model string

    Content []ContentPart

    ToolCalls []ToolCall

    FinishReason FinishReason
    Usage        *UsageEvent

    Warnings []WarningEvent
    Raw      []RawEvent
}

func Collect(ctx context.Context, events <-chan Event) (Response, error) {
    var resp Response

    for ev := range events {
        switch ev := ev.(type) {
        case MessageStartEvent:
            resp.ID = ev.ID
            resp.Model = ev.Model
        case TextDeltaEvent:
            // accumulate into resp.Content
        case ToolCallDoneEvent:
            resp.ToolCalls = append(resp.ToolCalls, ToolCall{
                ID:        ev.ID,
                Name:      ev.Name,
                Arguments: ev.Args,
            })
        case UsageEvent:
            resp.Usage = &ev
        case CompletedEvent:
            resp.FinishReason = ev.FinishReason
        case WarningEvent:
            resp.Warnings = append(resp.Warnings, ev)
        case RawEvent:
            resp.Raw = append(resp.Raw, ev)
        case ErrorEvent:
            return resp, ev.Err
        }
    }

    return resp, nil
}
```

Usage:

```go
events, err := client.Request(ctx, req)
if err != nil {
    return err
}

resp, err := unified.Collect(ctx, events)
if err != nil {
    return err
}

fmt.Println(resp.Content)
```

---

## 36. Position on `n > 1` / multiple choices

The unified layer models exactly one assistant output per request.

Rationale:

- Most modern LLM application flows consume one assistant response.
- Multi-choice support would complicate every event type with choice indices.
- Tool calling, reasoning, usage aggregation, and response collection all become harder with parallel generations.
- The canonical IR should stay opinionated and simple.

Policy:

- `unified.Request`, `unified.Event`, and `unified.Response` are single-output abstractions.
- If a provider API supports `n > 1` and canonical conversion is requested:
  - strict mode: return an unsupported-field error.
  - best-effort mode: use the first choice and emit a warning.
- Passthrough mode preserves `n > 1` exactly when inbound and outbound API kinds are compatible.
- Provider codecs that receive multi-choice responses must select choice 0 internally and may preserve raw choice metadata via `RawEvent` or `Extensions`.

---

## 37. Minimal core interface summary

```go
type Client interface {
    Request(context.Context, unified.Request) (<-chan unified.Event, error)
}

type NativeClient[Req any, Evt any] interface {
    ApiKind() ApiKind
    Request(context.Context, Req) (<-chan Evt, error)
}

type ProviderCodec[Req any, Evt any] interface {
    ApiKind() ApiKind
    EncodeRequest(context.Context, adapt.Request) (Req, error)
    NewEventDecoder() EventDecoder[Evt, unified.Event]
}

type EndpointCodec interface {
    ApiKind() ApiKind
    DecodeHTTP(context.Context, *http.Request) (adapt.Request, error)
    WriteEvents(context.Context, http.ResponseWriter, <-chan unified.Event) error
    WriteError(context.Context, http.ResponseWriter, error) error
}

type EventDecoder[In any, Out any] interface {
    Push(context.Context, In) ([]Out, error)
    Close(context.Context) ([]Out, error)
}

type Processor[E any] interface {
    Push(context.Context, E) ([]E, error)
    Close(context.Context) ([]E, error)
}

type ByteStreamTransport interface {
    Open(context.Context, *transport.Request) (transport.ByteStream, error)
}

type ByteStream interface {
    Recv(context.Context) ([]byte, error)
    Close() error
}
```

This is the core around which the rest of `llmadapter` can grow.
