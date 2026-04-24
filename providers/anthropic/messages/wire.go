package messages

import "encoding/json"

type MessageRequest struct {
	Model               string           `json:"model"`
	MaxTokens           int              `json:"max_tokens"`
	Messages            []InputMessage   `json:"messages,omitempty"`
	System              string           `json:"system,omitempty"`
	Temperature         *float64         `json:"temperature,omitempty"`
	TopP                *float64         `json:"top_p,omitempty"`
	TopK                *int             `json:"top_k,omitempty"`
	StopSequences       []string         `json:"stop_sequences,omitempty"`
	Stream              bool             `json:"stream,omitempty"`
	Tools               []ToolDefinition `json:"tools,omitempty"`
	ToolChoice          *ToolChoiceWire  `json:"tool_choice,omitempty"`
	Metadata            map[string]any   `json:"metadata,omitempty"`
	OpenRouterModels    json.RawMessage  `json:"models,omitempty"`
	OpenRouterRoute     json.RawMessage  `json:"route,omitempty"`
	OpenRouterProvider  json.RawMessage  `json:"provider,omitempty"`
	OpenRouterPrefs     json.RawMessage  `json:"provider_preferences,omitempty"`
	OpenRouterPlugins   json.RawMessage  `json:"plugins,omitempty"`
	OpenRouterDebug     json.RawMessage  `json:"debug,omitempty"`
	OpenRouterTrace     json.RawMessage  `json:"trace,omitempty"`
	OpenRouterSessionID json.RawMessage  `json:"session_id,omitempty"`
}

type InputMessage struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Source    *BlockSource    `json:"source,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   any             `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	Citations []any           `json:"citations,omitempty"`
	Thinking  string          `json:"thinking,omitempty"`
	Signature string          `json:"signature,omitempty"`
}

type BlockSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type ToolChoiceWire struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

type Event interface {
	isAnthropicEvent()
}

type MessageStartEvent struct {
	Type    string          `json:"type"`
	Message MessageResponse `json:"message"`
}

func (MessageStartEvent) isAnthropicEvent() {}

type MessageResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type,omitempty"`
	Role         string         `json:"role,omitempty"`
	Content      []ContentBlock `json:"content,omitempty"`
	Model        string         `json:"model,omitempty"`
	StopReason   string         `json:"stop_reason,omitempty"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        *UsageWire     `json:"usage,omitempty"`
}

type ContentBlockStartEvent struct {
	Type         string       `json:"type"`
	Index        int          `json:"index"`
	ContentBlock ContentBlock `json:"content_block"`
}

func (ContentBlockStartEvent) isAnthropicEvent() {}

type ContentBlockDeltaEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta Delta  `json:"delta"`
}

func (ContentBlockDeltaEvent) isAnthropicEvent() {}

type Delta struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	Thinking    string `json:"thinking,omitempty"`
	Signature   string `json:"signature,omitempty"`
}

type ContentBlockStopEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

func (ContentBlockStopEvent) isAnthropicEvent() {}

type MessageDeltaEvent struct {
	Type  string           `json:"type"`
	Delta MessageDeltaBody `json:"delta"`
	Usage *UsageWire       `json:"usage,omitempty"`
}

func (MessageDeltaEvent) isAnthropicEvent() {}

type MessageDeltaBody struct {
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

type MessageStopEvent struct {
	Type string `json:"type"`
}

func (MessageStopEvent) isAnthropicEvent() {}

type PingEvent struct {
	Type string `json:"type"`
}

func (PingEvent) isAnthropicEvent() {}

type ErrorEventWire struct {
	Type  string       `json:"type"`
	Error APIErrorBody `json:"error"`
}

func (ErrorEventWire) isAnthropicEvent() {}

type APIErrorBody struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message,omitempty"`
}

type UsageWire struct {
	InputTokens              int `json:"input_tokens,omitempty"`
	OutputTokens             int `json:"output_tokens,omitempty"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}
