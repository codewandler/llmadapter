package unified

import "encoding/json"

type ResponseFormatKind string

const (
	ResponseFormatText       ResponseFormatKind = "text"
	ResponseFormatJSON       ResponseFormatKind = "json"
	ResponseFormatJSONSchema ResponseFormatKind = "json_schema"
)

type ResponseFormat struct {
	Kind   ResponseFormatKind `json:"kind"`
	Schema json.RawMessage    `json:"schema,omitempty"`
	Name   string             `json:"name,omitempty"`
	Strict bool               `json:"strict,omitempty"`
}

type ReasoningEffort string

const (
	ReasoningEffortLow    ReasoningEffort = "low"
	ReasoningEffortMedium ReasoningEffort = "medium"
	ReasoningEffortHigh   ReasoningEffort = "high"
)

type ReasoningConfig struct {
	Effort     ReasoningEffort `json:"effort,omitempty"`
	MaxTokens  *int            `json:"max_tokens,omitempty"`
	Expose     bool            `json:"expose,omitempty"`
	Extensions Extensions      `json:"extensions,omitempty"`
}

type SafetyConfig struct {
	Policies   []string   `json:"policies,omitempty"`
	Extensions Extensions `json:"extensions,omitempty"`
}
