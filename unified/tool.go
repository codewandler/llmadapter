package unified

import "encoding/json"

type ToolKind string

const (
	ToolKindFunction ToolKind = "function"
)

type Tool struct {
	Kind         ToolKind        `json:"kind"`
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	InputSchema  json.RawMessage `json:"input_schema,omitempty"`
	Config       map[string]any  `json:"config,omitempty"`
	CacheControl *CacheControl   `json:"cache_control,omitempty"`
}

type ToolChoiceMode string

const (
	ToolChoiceAuto     ToolChoiceMode = "auto"
	ToolChoiceAny      ToolChoiceMode = "any"
	ToolChoiceNone     ToolChoiceMode = "none"
	ToolChoiceRequired ToolChoiceMode = "required"
	ToolChoiceTool     ToolChoiceMode = "tool"
)

type ToolChoice struct {
	Mode ToolChoiceMode `json:"mode"`
	Name string         `json:"name,omitempty"`
}

type ToolCall struct {
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
	Index     int             `json:"index,omitempty"`
}

type ToolResult struct {
	ToolCallID string        `json:"tool_call_id"`
	Name       string        `json:"name,omitempty"`
	Content    []ContentPart `json:"content,omitempty"`
	IsError    bool          `json:"is_error,omitempty"`
}
