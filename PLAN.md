# llmadapter Implementation Plan

Concrete execution steps for phases 1–3 from DESIGN.md §33.

Each step produces compilable, tested code. Steps within a phase are ordered by dependency. No step should require forward references to unwritten code.

---

## Phase 1: Core IR and stream pipeline

Foundation types and pipeline mechanics. No providers, no transport, no HTTP.

### Step 1.1: `unified` package — content and blob types

Create `unified/content.go`.

Types:

```text
ContentKind           (string enum)
ContentPart           (interface with unexported contentKind())
TextPart
ImagePart
AudioPart
VideoPart
FilePart
ReasoningPart
RefusalPart
BlobSourceKind        (string enum)
BlobSource
```

Tests:

```text
each part returns correct ContentKind
```

File: `unified/content.go`, `unified/content_test.go`

### Step 1.2: `unified` package — messages and instructions

Create `unified/message.go`.

Types:

```text
Role                  (string enum: user, assistant, system, tool)
Message               (Role, ID, Name, Content, ToolCalls, ToolResults, Meta)
InstructionKind       (string enum: system, developer, policy)
Instruction           (Kind, Content, Name, Meta)
```

File: `unified/message.go`

### Step 1.3: `unified` package — tools

Create `unified/tool.go`.

Types:

```text
ToolKind              (string enum)
Tool                  (Kind, Name, Description, InputSchema, Config)
ToolChoiceMode        (string enum)
ToolChoice            (Mode, Name)
ToolCall              (ID, Name, Arguments, Index)
ToolResult            (ToolCallID, Name, Content, IsError)
```

File: `unified/tool.go`

### Step 1.4: `unified` package — response format, reasoning, safety

Create `unified/response_format.go`.

Types:

```text
ResponseFormatKind    (string enum)
ResponseFormat        (Kind, Schema, Name, Strict)
ReasoningEffort       (string enum)
ReasoningConfig       (Effort, MaxTokens, Expose, Extensions)
SafetyConfig          (Policies, Extensions)
```

File: `unified/response_format.go`

### Step 1.5: `unified` package — extensions

Create `unified/extensions.go`.

Types:

```text
Extensions            (struct with unexported map[string]json.RawMessage)
Extensions.Set(key, value) error
Extensions.Has(key) bool
Extensions.Keys() []string
GetExtension[T](e, key) (T, bool, error)
```

Extension key constants:

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
Set and GetExtension roundtrip for string, int, struct
GetExtension on missing key returns (zero, false, nil)
GetExtension with type mismatch returns error
Has returns true/false correctly
Keys returns sorted keys
Set with nil value marshals null
```

File: `unified/extensions.go`, `unified/extensions_test.go`

### Step 1.6: `unified` package — request

Create `unified/request.go`.

Types:

```text
Request               (Model, Messages, Instructions, MaxOutputTokens, Temperature,
                       TopP, TopK, Stop, Seed, ResponseFormat, Tools, ToolChoice,
                       Reasoning, Safety, Stream, User, Extensions)
```

File: `unified/request.go`

### Step 1.7: `unified` package — events

Create `unified/event.go`.

Types:

```text
Event                 (interface with unexported isEvent())
MessageStartEvent     (ID, Model, Role, Time)
MessageDoneEvent      (ID)
ContentBlockStartEvent (Index, Kind, ID, Name)
ContentBlockDoneEvent  (Index, Kind)
TextDeltaEvent        (Index, Text)
ReasoningDeltaEvent   (Index, Text)
RefusalDeltaEvent     (Index, Text)
ToolCallStartEvent    (Index, ID, Name)
ToolCallArgsDeltaEvent (Index, ID, Delta)
ToolCallDoneEvent     (Index, ID, Name, Args)
CitationEvent         (Index, Citation)
Citation              (Type, Text, URL, Title, StartOffset, EndOffset, DocumentID, Meta)
UsageEvent            (InputTokens, OutputTokens, ReasoningTokens, CacheReadTokens,
                       CacheWriteTokens, TotalTokens, ProviderRaw)
FinishReason          (string enum)
CompletedEvent        (FinishReason, MessageID)
WarningEvent          (Code, Message, Source, Meta)
RawEvent              (APIKind, Type, JSON, Value)
ErrorEvent            (Err, Recoverable)
```

Tests:

```text
every event type satisfies Event interface
```

File: `unified/event.go`, `unified/event_test.go`

### Step 1.8: `unified` package — usage, response, collect

Create `unified/usage.go` and `unified/response.go`.

Types:

```text
Response              (ID, Model, Content, ToolCalls, FinishReason, Usage, Warnings, Raw)
Collect(ctx, <-chan Event) (Response, error)
```

Tests:

```text
Collect empty stream returns zero Response
Collect text-only stream assembles Content with single TextPart
Collect with tool calls populates ToolCalls
Collect with UsageEvent populates Usage
Collect with CompletedEvent populates FinishReason
Collect with ErrorEvent returns error
Collect with WarningEvents populates Warnings
Collect with RawEvents populates Raw
Collect with cancelled context returns context error
Collect multi-block stream (text + tool call) assembles correctly
```

File: `unified/response.go`, `unified/response_test.go`

### Step 1.9: `unified` package — errors

Create `unified/errors.go`.

Types:

```text
APIError              (StatusCode, Type, Code, Message, Param, RetryAfter, ProviderRaw)
APIError.Error() string
```

Tests:

```text
APIError satisfies error interface
APIError.Error() formats correctly
errors.As extracts *APIError from wrapped error
```

File: `unified/errors.go`, `unified/errors_test.go`

### Step 1.10: `unified` package — client interface

Create `unified/client.go`.

Types:

```text
Client                (interface: Request(ctx, Request) (<-chan Event, error))
```

File: `unified/client.go`

### Step 1.11: `adapt` package — types and request envelope

Create `adapt/types.go` and `adapt/request.go`.

Types:

```text
ApiKind               (string enum with all known API kinds)
ApiFamily             (string enum with all known families)
MappingMode           (string enum: strict, best_effort)
Warning               (Code, Field, Message)
UnsupportedFieldError (APIKind, Field, Reason)
HTTPRequestInfo       (Method, Path, Query, Headers, Remote)
Request               (SourceAPI, HTTP, RawBody, Raw, Unified, MappingMode, Warnings, Extensions)
```

Tests:

```text
UnsupportedFieldError satisfies error interface
```

File: `adapt/types.go`, `adapt/request.go`, `adapt/request_test.go`

### Step 1.12: `adapt` package — codec interfaces

Create `adapt/codec.go`.

Types:

```text
EventDecoder[In, Out] (interface: Push, Close)
EventEncoder[In, Out] (interface: Push, Close)
ProviderCodec[Req, Evt] (interface: ApiKind, EncodeRequest, NewEventDecoder)
RequestProcessor      (interface: ProcessRequest)
ProviderRequestProcessor[Req] (interface: ProcessProviderRequest)
```

File: `adapt/codec.go`

### Step 1.13: `pipeline` package — processor, chain, transform

Create `pipeline/processor.go`, `pipeline/chain.go`, `pipeline/transform.go`.

Types:

```text
Processor[E]          (interface: Push, Close)
Chain[E]              (struct wrapping []Processor[E])
NewChain[E](...Processor[E]) *Chain[E]
Chain.Push(ctx, E) ([]E, error)
Chain.Close(ctx) ([]E, error)

Item[T]               (Value, Err)
Transform[In, Out](ctx, <-chan In, EventDecoder[In, Out]) <-chan Item[Out]
```

`Chain.Close` must cascade: close-produced events from processor N are pushed through processors N+1..end.

Tests for `Chain`:

```text
empty chain passes through
single passthrough processor
single filtering processor (drops some events)
single expanding processor (one in, many out)
two processors compose correctly
Close cascades through chain
error from Push stops chain
error from Close propagates
```

Tests for `Transform`:

```text
transforms all events from input channel
decoder.Close is called when input closes
errors from Push are emitted as Item.Err
errors from Close are emitted as Item.Err
context cancellation stops transform and emits ctx.Err
backpressure: slow consumer does not lose events
input channel closing produces final flushed events
```

File: `pipeline/processor.go`, `pipeline/chain.go`, `pipeline/chain_test.go`, `pipeline/transform.go`, `pipeline/transform_test.go`

### Step 1.14: `pipeline` package — built-in processors

Create `pipeline/coalesce.go`, `pipeline/filter.go`, `pipeline/inject.go`.

Processors:

```text
TextCoalescer         (MaxBytes, Push, Close, flush)
ReasoningFilter       (Expose, Push, Close)
CompletionInjector    (Push, Close)
```

Tests:

```text
TextCoalescer: buffers below threshold, emits nothing
TextCoalescer: emits when threshold reached
TextCoalescer: flushes before non-text event
TextCoalescer: flush on Close
TextCoalescer: non-text events pass through immediately
TextCoalescer: empty Close returns nil
ReasoningFilter: drops reasoning when Expose=false
ReasoningFilter: passes reasoning when Expose=true
ReasoningFilter: passes non-reasoning events always
CompletionInjector: passes CompletedEvent and does not inject on Close
CompletionInjector: injects CompletedEvent on Close if none seen
CompletionInjector: passes all non-completed events through
```

File: `pipeline/coalesce.go`, `pipeline/coalesce_test.go`, `pipeline/filter.go`, `pipeline/filter_test.go`, `pipeline/inject.go`, `pipeline/inject_test.go`

### Phase 1 checkpoint

After step 1.14, verify:

```text
go build ./unified/... ./adapt/... ./pipeline/...
go test ./unified/... ./adapt/... ./pipeline/...
go vet ./...
```

All packages compile, all tests pass, zero external dependencies beyond stdlib.

---

## Phase 2: Transport foundation

Byte-frame transport abstraction, HTTP implementation, SSE/NDJSON parsers, fake transport for tests.

### Step 2.1: `transport` package — core types

Create `transport/transport.go`.

Types:

```text
Request               (Method, URL, Header, Body, Extensions)
ByteStreamTransport   (interface: Open(ctx, *Request) (ByteStream, error))
ByteStream            (interface: Recv(ctx) ([]byte, error), Close() error)
FrameDecoder[Evt]     (interface: PushFrame(ctx, []byte) ([]Evt, error), Close(ctx) ([]Evt, error))
```

File: `transport/transport.go`

### Step 2.2: `transport` package — SSE frame parser

Create `transport/sse.go`.

An SSE parser reads from an `io.Reader` and produces SSE events as byte frames.

Types:

```text
SSEFrame              (Event, Data, ID, Retry)
SSEReader             (struct wrapping bufio.Scanner)
NewSSEReader(r io.Reader) *SSEReader
SSEReader.Next() (SSEFrame, error)   // returns io.EOF at end
```

Behavior:

```text
parses multi-line data fields (joined with \n)
handles event: field
handles id: field
handles retry: field
ignores comment lines (starting with :)
handles \r\n, \n, \r line endings
handles empty data lines
returns io.EOF on reader exhaustion
```

Tests:

```text
single data-only event
multi-line data event
event with event: type
event with id:
event with retry:
comment-only lines skipped
multiple events separated by blank lines
empty data field
mixed line endings (\r\n and \n)
stream ends cleanly with io.EOF
malformed lines (no colon) treated as field with empty value
consecutive blank lines (empty dispatch)
```

File: `transport/sse.go`, `transport/sse_test.go`

### Step 2.3: `transport` package — NDJSON frame parser

Create `transport/ndjson.go`.

Types:

```text
NDJSONReader          (struct wrapping bufio.Scanner)
NewNDJSONReader(r io.Reader) *NDJSONReader
NDJSONReader.Next() ([]byte, error)  // returns io.EOF at end
```

Behavior:

```text
reads one JSON object per line
skips empty lines
returns raw bytes (not parsed)
returns io.EOF on reader exhaustion
handles lines up to a configurable max size (default 1MB)
```

Tests:

```text
single line object
multiple line objects
empty lines skipped
large line within limit
line exceeding limit returns error
stream ends with io.EOF
lines with trailing whitespace
```

File: `transport/ndjson.go`, `transport/ndjson_test.go`

### Step 2.4: `transport` package — HTTP byte stream transport

Create `transport/http.go`.

Types:

```text
FrameFormat           (string enum: SSE, NDJSON, Raw)

HTTPTransportConfig   (Client *http.Client, FrameFormat)
HTTPByteStreamTransport (struct)
NewHTTPByteStreamTransport(cfg HTTPTransportConfig) *HTTPByteStreamTransport

httpByteStream        (unexported struct implementing ByteStream)
```

Behavior:

```text
Open sends HTTP request using http.Client
Open checks response status; non-2xx returns *unified.APIError with status, body preview
For SSE format: wraps response body in SSEReader, Recv returns each SSE data frame
For NDJSON format: wraps response body in NDJSONReader, Recv returns each line
For Raw format: first Recv returns full body, second returns io.EOF
Close closes response body
Context cancellation during Recv returns ctx.Err()
```

Tests (using httptest.Server):

```text
SSE: streams multiple frames
SSE: returns io.EOF after last frame
NDJSON: streams multiple frames
Raw: returns full body as single frame then EOF
non-2xx status returns APIError with correct StatusCode
Close closes underlying body
context cancellation during stream returns error
request has correct method, URL, headers, body
```

File: `transport/http.go`, `transport/http_test.go`

### Step 2.5: `transport` package — fake byte stream transport

Create `transport/fake.go`.

Types:

```text
FakeByteStreamTransport (struct)
  Frames      [][]byte
  Err         error         // error to return after all frames (or instead of frames)
  ErrAtFrame  int           // return Err at this frame index (-1 = after all frames)
  OpenErr     error         // error to return from Open
  Seen        []*Request    // captured requests

fakeByteStream (unexported)
```

Behavior:

```text
Open appends request to Seen, returns fakeByteStream or OpenErr
Recv returns frames in order
Recv returns Err at configured frame index
Recv returns io.EOF after all frames if no Err configured
Close is a no-op
```

Tests:

```text
returns frames in order then EOF
returns error at configured frame
OpenErr returned from Open
captures requests in Seen
empty frames returns EOF immediately
```

File: `transport/fake.go`, `transport/fake_test.go`

### Step 2.6: `transport` package — retry wrapper

Create `transport/retry.go`.

Types:

```text
RetryMode             (string enum: never, before_stream)

RetryConfig           (Mode, MaxRetries, InitialBackoff, MaxBackoff, RetryableStatus func(int) bool)
RetryTransport        (struct wrapping ByteStreamTransport)
NewRetryTransport(inner ByteStreamTransport, cfg RetryConfig) *RetryTransport
```

Behavior:

```text
Mode=never: delegates directly, no retry
Mode=before_stream: on Open error or retryable status, retries with backoff
retries up to MaxRetries times
uses exponential backoff with jitter
does not retry after first successful Recv (stream has started)
respects context cancellation during backoff
default RetryableStatus: 429, 500, 502, 503, 504
```

Tests (using FakeByteStreamTransport sequences):

```text
never mode: no retry on error
before_stream: retries on OpenErr, succeeds on second attempt
before_stream: retries on retryable status error
before_stream: gives up after MaxRetries
before_stream: does not retry non-retryable status
context cancellation during backoff returns ctx.Err()
successful open with no errors: no retry logic invoked
```

File: `transport/retry.go`, `transport/retry_test.go`

### Phase 2 checkpoint

After step 2.6, verify:

```text
go build ./transport/...
go test ./transport/...
go vet ./...
```

No external dependencies beyond stdlib. The `transport` package imports `unified` only for `APIError`.

---

## Phase 3: One complete provider path — Anthropic Messages

End-to-end: `unified.Request` → Anthropic wire request → HTTP transport → SSE byte stream → Anthropic wire events → `unified.Event` stream.

This phase produces a working `unified.Client` for Anthropic.

### Step 3.1: `providers/anthropic/messages` — wire types

Create wire types mirroring the Anthropic Messages API.

Request types:

```text
MessageRequest        (Model, MaxTokens, Messages, System, Temperature, TopP, TopK,
                       StopSequences, Stream, Tools, ToolChoice, Metadata)
InputMessage          (Role, Content)
ContentBlock          (Type + type-specific fields: text, image, tool_use, tool_result)
ToolDefinition        (Name, Description, InputSchema)
ToolChoiceWire        (Type, Name)
```

Response/event types:

```text
Event                 (interface with unexported marker)
MessageStartEvent     (Type, Message: MessageResponse)
MessageResponse       (ID, Type, Role, Content, Model, StopReason, StopSequence, Usage)
ContentBlockStartEvent (Type, Index, ContentBlock)
ContentBlockDeltaEvent (Type, Index, Delta)
Delta                 (Type, Text, PartialJSON)
ContentBlockStopEvent  (Type, Index)
MessageDeltaEvent     (Type, Delta: MessageDeltaBody, Usage)
MessageDeltaBody      (StopReason, StopSequence)
MessageStopEvent      (Type)
PingEvent             (Type)
ErrorEventWire        (Type, Error: APIErrorBody)
APIErrorBody          (Type, Message)
UsageWire             (InputTokens, OutputTokens, CacheCreationInputTokens, CacheReadInputTokens)
```

All types must marshal/unmarshal correctly to/from JSON matching the Anthropic API spec.

Tests:

```text
MessageRequest JSON roundtrip: basic text
MessageRequest JSON roundtrip: with tools
MessageRequest JSON roundtrip: with system
MessageRequest JSON roundtrip: with images
MessageResponse JSON roundtrip
each event type JSON roundtrip
omitempty fields omitted when zero
```

File: `providers/anthropic/messages/wire.go`, `providers/anthropic/messages/wire_test.go`

### Step 3.2: `providers/anthropic/messages` — SSE event parser

Create a `FrameDecoder` that turns SSE data frames into Anthropic wire events.

Types:

```text
SSEFrameDecoder       (struct implementing transport.FrameDecoder[Event])
```

Behavior:

```text
parses SSE event type + JSON data
dispatches to correct event struct based on event type
handles: message_start, content_block_start, content_block_delta,
         content_block_stop, message_delta, message_stop, ping, error
unknown event types: returns raw/warning or error per policy
```

Input: raw SSE frame bytes (the `data:` payload), with the SSE event type available.

Note: The `transport.SSEReader` produces `SSEFrame` values with both `Event` and `Data` fields. The frame decoder receives the `SSEFrame` and uses the `Event` field to determine the type.

Refine `FrameDecoder` signature if needed: the decoder may need the full `SSEFrame` rather than bare `[]byte`. If so, make `SSEFrameDecoder` implement `adapt.EventDecoder[transport.SSEFrame, Event]` instead of `transport.FrameDecoder[Event]`.

Tests:

```text
message_start frame parses correctly
content_block_start (text) frame parses correctly
content_block_start (tool_use) frame parses correctly
content_block_delta (text_delta) frame parses correctly
content_block_delta (input_json_delta) frame parses correctly
content_block_stop frame parses correctly
message_delta frame parses correctly
message_stop frame parses correctly
ping frame parses to PingEvent
error frame parses to ErrorEventWire
unknown event type handling
malformed JSON returns error
```

File: `providers/anthropic/messages/sse.go`, `providers/anthropic/messages/sse_test.go`

### Step 3.3: `providers/anthropic/messages` — request codec (unified → wire)

Create the request encoder.

Types:

```text
Codec                 (struct implementing adapt.ProviderCodec[MessageRequest, Event])
Codec.ApiKind() → ApiAnthropicMessages
Codec.EncodeRequest(ctx, adapt.Request) → (MessageRequest, error)
```

Mapping rules:

```text
unified.Request.Model → MessageRequest.Model
unified.Request.Messages → MessageRequest.Messages
  role mapping: user→user, assistant→assistant, tool→user (with tool_result content)
  content parts: TextPart→text block, ImagePart→image block, etc.
  ToolResults → tool_result content blocks
  ToolCalls → assistant message with tool_use content blocks
unified.Request.Instructions → MessageRequest.System
unified.Request.MaxOutputTokens → MessageRequest.MaxTokens (required by Anthropic)
unified.Request.Temperature → MessageRequest.Temperature
unified.Request.TopP → MessageRequest.TopP
unified.Request.TopK → MessageRequest.TopK
unified.Request.Stop → MessageRequest.StopSequences
unified.Request.Tools → MessageRequest.Tools (function tools only)
unified.Request.ToolChoice → MessageRequest.ToolChoice
unified.Request.Stream → MessageRequest.Stream
unified.Request.ResponseFormat (json_schema) → tool-based structured output pattern or warning
unified.Request.Reasoning → extension or warning
```

Lossiness:

```text
strict mode: error on ResponseFormat.JSONSchema, Seed, unsupported tool kinds
best-effort mode: warn and skip unsupported fields
```

Tests (golden fixture style):

```text
basic text message → wire JSON
multi-turn conversation → wire JSON
system instructions → wire system field
tools with function definitions → wire JSON
tool results → wire JSON with tool_result blocks
image content → wire JSON with image block
MaxOutputTokens missing → error (Anthropic requires it)
unsupported fields in strict mode → UnsupportedFieldError
unsupported fields in best-effort mode → warnings
Stream=true → wire stream=true
```

Fixture files:

```text
testdata/anthropic/request_basic.unified.json
testdata/anthropic/request_basic.wire.json
testdata/anthropic/request_tools.unified.json
testdata/anthropic/request_tools.wire.json
testdata/anthropic/request_tool_results.unified.json
testdata/anthropic/request_tool_results.wire.json
testdata/anthropic/request_image.unified.json
testdata/anthropic/request_image.wire.json
testdata/anthropic/request_system.unified.json
testdata/anthropic/request_system.wire.json
```

File: `providers/anthropic/messages/codec.go`, `providers/anthropic/messages/codec_test.go`, `testdata/anthropic/...`

### Step 3.4: `providers/anthropic/messages` — event decoder (wire → unified)

Create the stateful event decoder.

Types:

```text
EventDecoder          (struct implementing adapt.EventDecoder[Event, unified.Event])
  internal state: toolBuffers, toolNames, toolIDs maps
```

Mapping rules:

```text
MessageStartEvent → unified.MessageStartEvent (ID, Model, Role=assistant)
ContentBlockStartEvent (type=text) → unified.ContentBlockStartEvent (Kind=text)
ContentBlockStartEvent (type=tool_use) → unified.ContentBlockStartEvent (Kind=tool_call)
                                       + unified.ToolCallStartEvent
                                       + register tool ID/name in state
ContentBlockDeltaEvent (text_delta) → unified.TextDeltaEvent
ContentBlockDeltaEvent (input_json_delta) → unified.ToolCallArgsDeltaEvent + buffer args
ContentBlockDeltaEvent (thinking) → unified.ReasoningDeltaEvent
ContentBlockStopEvent (text block) → unified.ContentBlockDoneEvent
ContentBlockStopEvent (tool block) → unified.ToolCallDoneEvent (with buffered args)
                                    + unified.ContentBlockDoneEvent
MessageDeltaEvent → unified.CompletedEvent (map stop_reason → FinishReason)
                   + unified.UsageEvent (if usage present)
MessageStopEvent → unified.MessageDoneEvent
PingEvent → nil (drop)
ErrorEventWire → unified.ErrorEvent (wrap as APIError)
```

Stop reason mapping:

```text
end_turn → FinishReasonStop
max_tokens → FinishReasonLength
tool_use → FinishReasonToolCall
stop_sequence → FinishReasonStop
```

Close behavior:

```text
error if toolBuffers not empty (incomplete stream)
```

Tests (golden fixture style, NDJSON event sequences):

```text
plain text stream → unified events
multi-block text → unified events
tool use stream → unified events (start, arg deltas, done)
parallel tool calls → unified events
text + tool use mixed → unified events
reasoning/thinking deltas → unified ReasoningDeltaEvent
usage at message_delta → unified UsageEvent
stop reasons map correctly
ping events dropped
error event → unified ErrorEvent
incomplete tool buffer at Close → error
unknown content block delta type → handled per policy
```

Fixture files:

```text
testdata/anthropic/events_text.input.ndjson
testdata/anthropic/events_text.unified.ndjson
testdata/anthropic/events_tool_use.input.ndjson
testdata/anthropic/events_tool_use.unified.ndjson
testdata/anthropic/events_multi_tool.input.ndjson
testdata/anthropic/events_multi_tool.unified.ndjson
testdata/anthropic/events_error.input.ndjson
testdata/anthropic/events_error.unified.ndjson
```

File: `providers/anthropic/messages/decoder.go`, `providers/anthropic/messages/decoder_test.go`, `testdata/anthropic/...`

### Step 3.5: `providers/anthropic/messages` — native client

Create the provider-level native client that owns HTTP transport.

Types:

```text
NativeClient          (struct implementing adapt.NativeClient[MessageRequest, Event])
  transport: transport.ByteStreamTransport
  baseURL:   string
  headers:   http.Header
  headerFns: []HeaderFunc
```

Behavior:

```text
marshals MessageRequest to JSON body
builds transport.Request: POST {baseURL}/v1/messages
sets Content-Type: application/json
sets x-api-key header
sets anthropic-version header
applies static headers and HeaderFuncs
calls transport.Open
spawns goroutine: reads ByteStream frames → SSEFrameDecoder → Event channel
on error: sends error to channel, closes
on stream end: closes channel
```

Tests (using FakeByteStreamTransport):

```text
correct URL constructed
correct method (POST)
correct headers (x-api-key, anthropic-version, content-type)
request body is valid MessageRequest JSON
HeaderFuncs applied
SSE frames decoded into correct Event sequence
transport error propagated
context cancellation stops stream
```

File: `providers/anthropic/messages/client.go`, `providers/anthropic/messages/client_test.go`

### Step 3.6: `providers/anthropic/messages` — adapted client (unified.Client)

Assemble the full adapted client.

Types:

```text
AdaptedClient         (struct implementing unified.Client)
  native:   NativeClient
  codec:    Codec
  reqProcs: []adapt.RequestProcessor
  provReqProcs: []adapt.ProviderRequestProcessor[MessageRequest]
  provEvtProcs: []pipeline.Processor[Event]
  unifiedEvtProcs: []pipeline.Processor[unified.Event]
```

Behavior:

```text
wraps unified.Request in adapt.Request
runs request processors
encodes via codec
runs provider request processors
calls native client
runs provider event processors
decodes via event decoder
runs unified event processors
emits unified.Event to output channel
converts Item.Err to ErrorEvent at boundary
```

File: `providers/anthropic/messages/adapted.go`

### Step 3.7: `providers/anthropic/messages` — public constructor

Create the user-facing constructor with functional options.

Types:

```text
Option                (interface with unexported applyAnthropic)
Config                (core config + Version, Betas, provider-specific processors)

WithAPIKey(string) Option
WithBaseURL(string) Option
WithVersion(string) Option
WithBeta(string) Option
WithHeader(key, value string) Option
WithHeaderFunc(HeaderFunc) Option
WithTransport(ByteStreamTransport) Option
WithUnifiedEventProcessor(Processor[unified.Event]) Option
WithRequestProcessor(RequestProcessor) Option

NewClient(opts ...Option) (unified.Client, error)
```

Behavior:

```text
validates required config (API key)
sets defaults (baseURL, version)
assembles NativeClient, Codec, AdaptedClient
returns unified.Client
```

Tests:

```text
NewClient with API key succeeds
NewClient without API key returns error
WithBaseURL overrides default
WithVersion sets anthropic-version header
WithBeta appends anthropic-beta header
WithHeader adds custom header
WithTransport replaces default transport
custom processors are wired into pipeline
```

File: `providers/anthropic/messages/options.go`, `providers/anthropic/messages/options_test.go`

### Step 3.8: Integration test — full Anthropic path

End-to-end test using `FakeByteStreamTransport`.

```text
construct client with fake transport
send unified.Request with text message
fake transport returns SSE frames for a text response
verify unified.Event stream: MessageStart, ContentBlockStart, TextDelta(s),
  ContentBlockDone, CompletedEvent, UsageEvent, MessageDone
verify Collect produces correct Response

construct client with fake transport
send unified.Request with tools
fake transport returns SSE frames for a tool_use response
verify unified.Event stream includes ToolCallStart, ToolCallArgsDelta(s), ToolCallDone
verify Collect produces correct Response with ToolCalls

error case: fake transport returns error frames
verify ErrorEvent in stream
verify Collect returns error

processor integration: TextCoalescer coalesces deltas
processor integration: CompletionInjector synthesizes CompletedEvent
```

File: `providers/anthropic/messages/integration_test.go`

### Phase 3 checkpoint

After step 3.8, verify:

```text
go build ./...
go test ./...
go vet ./...
```

A programmatic caller can now do:

```go
client, err := anthropic.NewClient(
    anthropic.WithAPIKey("sk-..."),
)

events, err := client.Request(ctx, unified.Request{
    Model:           "claude-sonnet-4-20250514",
    MaxOutputTokens: intPtr(1024),
    Messages: []unified.Message{
        {Role: unified.RoleUser, Content: []unified.ContentPart{
            unified.TextPart{Text: "Hello"},
        }},
    },
    Stream: true,
})

resp, err := unified.Collect(ctx, events)
```

---

## Dependency graph

```text
Phase 1
  1.1 content
  1.2 message         ← 1.1
  1.3 tool            ← 1.1
  1.4 response_format
  1.5 extensions
  1.6 request         ← 1.1, 1.2, 1.3, 1.4, 1.5
  1.7 event           ← 1.1, 1.3
  1.8 response        ← 1.1, 1.3, 1.7
  1.9 errors
  1.10 client         ← 1.6, 1.7
  1.11 adapt types    ← 1.5, 1.6
  1.12 adapt codec    ← 1.7, 1.11
  1.13 pipeline       ← 1.12
  1.14 processors     ← 1.7, 1.13

Phase 2
  2.1 transport types
  2.2 SSE parser
  2.3 NDJSON parser
  2.4 HTTP transport  ← 2.1, 2.2, 2.3, 1.9
  2.5 fake transport  ← 2.1
  2.6 retry           ← 2.1, 2.5

Phase 3
  3.1 wire types
  3.2 SSE decoder     ← 3.1, 2.2
  3.3 request codec   ← 3.1, 1.11, 1.12
  3.4 event decoder   ← 3.1, 1.7, 1.12
  3.5 native client   ← 3.1, 3.2, 2.1
  3.6 adapted client  ← 3.3, 3.4, 3.5, 1.10, 1.13
  3.7 constructor     ← 3.6
  3.8 integration     ← 3.7, 2.5, 1.8, 1.14
```
