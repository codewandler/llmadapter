package chatcompletions

import "encoding/json"

type requestWire struct {
	Model         string             `json:"model"`
	Messages      []messageWire      `json:"messages"`
	MaxTokens     *int               `json:"max_tokens,omitempty"`
	Temperature   *float64           `json:"temperature,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
	Stop          []string           `json:"stop,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	StreamOptions *streamOptionsWire `json:"stream_options,omitempty"`
	Tools         []toolWire         `json:"tools,omitempty"`
	ToolChoice    any                `json:"tool_choice,omitempty"`
	User          string             `json:"user,omitempty"`
}

type streamOptionsWire struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type messageWire struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	Name       string         `json:"name,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []toolCallWire `json:"tool_calls,omitempty"`
}

type toolWire struct {
	Type     string           `json:"type"`
	Function functionToolWire `json:"function"`
}

type functionToolWire struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type toolCallWire struct {
	Index    int                  `json:"index,omitempty"`
	ID       string               `json:"id,omitempty"`
	Type     string               `json:"type,omitempty"`
	Function toolCallFunctionWire `json:"function"`
}

type toolCallFunctionWire struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type chunkWire struct {
	ID      string       `json:"id,omitempty"`
	Model   string       `json:"model,omitempty"`
	Choices []choiceWire `json:"choices,omitempty"`
	Usage   *usageWire   `json:"usage,omitempty"`
	Error   *errorWire   `json:"error,omitempty"`
}

type choiceWire struct {
	Index        int       `json:"index,omitempty"`
	Delta        deltaWire `json:"delta,omitempty"`
	FinishReason string    `json:"finish_reason,omitempty"`
}

type deltaWire struct {
	Role      string         `json:"role,omitempty"`
	Content   string         `json:"content,omitempty"`
	ToolCalls []toolCallWire `json:"tool_calls,omitempty"`
}

type usageWire struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

type errorWire struct {
	Type    string `json:"type,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
