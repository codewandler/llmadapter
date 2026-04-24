package unified

import (
	"encoding/json"
	"time"
)

type Event interface {
	isEvent()
}

type MessageStartEvent struct {
	ID    string    `json:"id,omitempty"`
	Model string    `json:"model,omitempty"`
	Role  Role      `json:"role,omitempty"`
	Time  time.Time `json:"time,omitempty"`
}

func (MessageStartEvent) isEvent() {}

type MessageDoneEvent struct {
	ID string `json:"id,omitempty"`
}

func (MessageDoneEvent) isEvent() {}

type ContentBlockStartEvent struct {
	Index int         `json:"index"`
	Kind  ContentKind `json:"kind"`
	ID    string      `json:"id,omitempty"`
	Name  string      `json:"name,omitempty"`
}

func (ContentBlockStartEvent) isEvent() {}

type ContentBlockDoneEvent struct {
	Index int         `json:"index"`
	Kind  ContentKind `json:"kind"`
}

func (ContentBlockDoneEvent) isEvent() {}

type TextDeltaEvent struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
}

func (TextDeltaEvent) isEvent() {}

type ReasoningDeltaEvent struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
}

func (ReasoningDeltaEvent) isEvent() {}

type RefusalDeltaEvent struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
}

func (RefusalDeltaEvent) isEvent() {}

type ToolCallStartEvent struct {
	Index int    `json:"index"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
}

func (ToolCallStartEvent) isEvent() {}

type ToolCallArgsDeltaEvent struct {
	Index int    `json:"index"`
	ID    string `json:"id,omitempty"`
	Delta string `json:"delta"`
}

func (ToolCallArgsDeltaEvent) isEvent() {}

type ToolCallDoneEvent struct {
	Index int             `json:"index"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Args  json.RawMessage `json:"args,omitempty"`
}

func (ToolCallDoneEvent) isEvent() {}

type CitationEvent struct {
	Index    int      `json:"index"`
	Citation Citation `json:"citation"`
}

func (CitationEvent) isEvent() {}

type Citation struct {
	Type        string         `json:"type,omitempty"`
	Text        string         `json:"text,omitempty"`
	URL         string         `json:"url,omitempty"`
	Title       string         `json:"title,omitempty"`
	StartOffset int            `json:"start_offset,omitempty"`
	EndOffset   int            `json:"end_offset,omitempty"`
	DocumentID  string         `json:"document_id,omitempty"`
	Meta        map[string]any `json:"meta,omitempty"`
}

type UsageEvent struct {
	Tokens      TokenItems      `json:"tokens,omitempty"`
	Costs       CostItems       `json:"costs,omitempty"`
	ProviderRaw json.RawMessage `json:"provider_raw,omitempty"`
}

func (UsageEvent) isEvent() {}

type FinishReason string

const (
	FinishReasonUnknown  FinishReason = "unknown"
	FinishReasonStop     FinishReason = "stop"
	FinishReasonLength   FinishReason = "length"
	FinishReasonToolCall FinishReason = "tool_call"
	FinishReasonError    FinishReason = "error"
)

type CompletedEvent struct {
	FinishReason FinishReason `json:"finish_reason,omitempty"`
	MessageID    string       `json:"message_id,omitempty"`
}

func (CompletedEvent) isEvent() {}

type WarningEvent struct {
	Code    string         `json:"code,omitempty"`
	Message string         `json:"message,omitempty"`
	Source  string         `json:"source,omitempty"`
	Meta    map[string]any `json:"meta,omitempty"`
}

func (WarningEvent) isEvent() {}

type RawEvent struct {
	APIKind string          `json:"api_kind,omitempty"`
	Type    string          `json:"type,omitempty"`
	JSON    json.RawMessage `json:"json,omitempty"`
	Value   any             `json:"value,omitempty"`
}

func (RawEvent) isEvent() {}

type ErrorEvent struct {
	Err         error `json:"-"`
	Recoverable bool  `json:"recoverable,omitempty"`
}

func (ErrorEvent) isEvent() {}
