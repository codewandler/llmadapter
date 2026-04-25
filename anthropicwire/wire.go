package anthropicwire

import (
	"bytes"
	"encoding/json"
	"strings"
)

type MessageRequest struct {
	Model               string           `json:"model"`
	MaxTokens           int              `json:"max_tokens"`
	Messages            []InputMessage   `json:"messages,omitempty"`
	System              *SystemContent   `json:"system,omitempty"`
	Temperature         *float64         `json:"temperature,omitempty"`
	TopP                *float64         `json:"top_p,omitempty"`
	TopK                *int             `json:"top_k,omitempty"`
	StopSequences       []string         `json:"stop_sequences,omitempty"`
	Stream              bool             `json:"stream,omitempty"`
	Thinking            *ThinkingConfig  `json:"thinking,omitempty"`
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

type ThinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

type InputMessage struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Cache     *CacheControl   `json:"cache_control,omitempty"`
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

type CacheControl struct {
	Type string `json:"type"`
	TTL  string `json:"ttl,omitempty"`
}

type SystemContent struct {
	Blocks []ContentBlock
}

func NewSystemContent(blocks ...ContentBlock) *SystemContent {
	out := &SystemContent{}
	for _, block := range blocks {
		if block.Type == "" {
			continue
		}
		out.Blocks = append(out.Blocks, block)
	}
	if len(out.Blocks) == 0 {
		return nil
	}
	return out
}

func (s *SystemContent) MarshalJSON() ([]byte, error) {
	if s == nil || len(s.Blocks) == 0 {
		return []byte("null"), nil
	}
	if s.canMarshalAsString() {
		return json.Marshal(s.Text())
	}
	return json.Marshal(s.Blocks)
}

func (s *SystemContent) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		s.Blocks = nil
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		s.Blocks = []ContentBlock{{Type: "text", Text: text}}
		return nil
	}
	var blocks []ContentBlock
	if err := json.Unmarshal(data, &blocks); err != nil {
		return err
	}
	s.Blocks = blocks
	return nil
}

func (s *SystemContent) Text() string {
	if s == nil {
		return ""
	}
	parts := make([]string, 0, len(s.Blocks))
	for _, block := range s.Blocks {
		if block.Type == "text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func (s *SystemContent) Append(blocks ...ContentBlock) {
	if s == nil {
		return
	}
	for _, block := range blocks {
		if block.Type == "" {
			continue
		}
		s.Blocks = append(s.Blocks, block)
	}
}

func (s *SystemContent) Prepend(blocks ...ContentBlock) {
	if s == nil {
		return
	}
	filtered := make([]ContentBlock, 0, len(blocks)+len(s.Blocks))
	for _, block := range blocks {
		if block.Type != "" {
			filtered = append(filtered, block)
		}
	}
	s.Blocks = append(filtered, s.Blocks...)
}

func (s *SystemContent) ApplyCacheToLastText(cache *CacheControl) bool {
	if s == nil || cache == nil {
		return false
	}
	for i := len(s.Blocks) - 1; i >= 0; i-- {
		if s.Blocks[i].Type == "text" && s.Blocks[i].Text != "" {
			s.Blocks[i].Cache = cache
			return true
		}
	}
	return false
}

func (s *SystemContent) canMarshalAsString() bool {
	if s == nil || len(s.Blocks) == 0 {
		return false
	}
	for _, block := range s.Blocks {
		if block.Type != "text" || block.Cache != nil || block.Source != nil || block.ID != "" || block.Name != "" || len(block.Input) != 0 || block.Content != nil || block.IsError || len(block.Citations) != 0 || block.Thinking != "" || block.Signature != "" {
			return false
		}
	}
	return true
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
	Cache       *CacheControl   `json:"cache_control,omitempty"`
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
