package unified

type Request struct {
	Model string `json:"model"`

	Messages     []Message     `json:"messages,omitempty"`
	Instructions []Instruction `json:"instructions,omitempty"`

	MaxOutputTokens *int     `json:"max_output_tokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"top_p,omitempty"`
	TopK            *int     `json:"top_k,omitempty"`
	Stop            []string `json:"stop,omitempty"`
	Seed            *int64   `json:"seed,omitempty"`

	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`

	Tools      []Tool      `json:"tools,omitempty"`
	ToolChoice *ToolChoice `json:"tool_choice,omitempty"`

	Reasoning *ReasoningConfig `json:"reasoning,omitempty"`
	Safety    *SafetyConfig    `json:"safety,omitempty"`

	Stream bool   `json:"stream,omitempty"`
	User   string `json:"user,omitempty"`

	CachePolicy CachePolicy `json:"cache_policy,omitempty"`
	CacheKey    string      `json:"cache_key,omitempty"`
	CacheTTL    string      `json:"cache_ttl,omitempty"`

	Extensions Extensions `json:"extensions,omitempty"`
}
