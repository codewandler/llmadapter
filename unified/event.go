package unified

import (
	"encoding/json"
	"time"
)

type Event interface {
	isEvent()
}

type MessageStartEvent struct {
	ID    string       `json:"id,omitempty"`
	Model string       `json:"model,omitempty"`
	Role  Role         `json:"role,omitempty"`
	Phase MessagePhase `json:"phase,omitempty"`
	Time  time.Time    `json:"time,omitempty"`
}

func (MessageStartEvent) isEvent() {}

type MessageDoneEvent struct {
	ID    string       `json:"id,omitempty"`
	Phase MessagePhase `json:"phase,omitempty"`
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
	Index     int    `json:"index"`
	Text      string `json:"text,omitempty"`
	Signature string `json:"signature,omitempty"`
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

type QuotaUsageEvent struct {
	Provider    string            `json:"provider,omitempty"`
	Plan        string            `json:"plan,omitempty"`
	Limits      []QuotaLimitUsage `json:"limits,omitempty"`
	ProviderRaw json.RawMessage   `json:"provider_raw,omitempty"`
}

func (QuotaUsageEvent) isEvent() {}

type QuotaLimitUsage struct {
	ID      string             `json:"id,omitempty"`
	Name    string             `json:"name,omitempty"`
	Windows []QuotaWindowUsage `json:"windows,omitempty"`
	Credits *QuotaCredits      `json:"credits,omitempty"`
}

type QuotaWindowUsage struct {
	Name          string  `json:"name,omitempty"`
	UsedPercent   float64 `json:"used_percent"`
	Limit         *int64  `json:"limit,omitempty"`
	Remaining     *int64  `json:"remaining,omitempty"`
	WindowMinutes *int    `json:"window_minutes,omitempty"`
	ResetsAtUnix  *int64  `json:"resets_at_unix,omitempty"`
}

type QuotaCredits struct {
	HasCredits *bool  `json:"has_credits,omitempty"`
	Unlimited  *bool  `json:"unlimited,omitempty"`
	Balance    string `json:"balance,omitempty"`
}

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

type RouteEvent struct {
	SourceAPI            string           `json:"source_api,omitempty"`
	TargetAPI            string           `json:"target_api,omitempty"`
	TargetFamily         string           `json:"target_family,omitempty"`
	ProviderName         string           `json:"provider_name,omitempty"`
	PublicModel          string           `json:"public_model,omitempty"`
	NativeModel          string           `json:"native_model,omitempty"`
	ConsumerContinuation ContinuationMode `json:"consumer_continuation,omitempty"`
	InternalContinuation ContinuationMode `json:"internal_continuation,omitempty"`
	Transport            TransportKind    `json:"transport,omitempty"`
}

func (RouteEvent) isEvent() {}

type ProviderExecutionEvent struct {
	InternalContinuation ContinuationMode `json:"internal_continuation,omitempty"`
	Transport            TransportKind    `json:"transport,omitempty"`
}

func (ProviderExecutionEvent) isEvent() {}

type ErrorEvent struct {
	Err         error `json:"-"`
	Recoverable bool  `json:"recoverable,omitempty"`
}

func (ErrorEvent) isEvent() {}
