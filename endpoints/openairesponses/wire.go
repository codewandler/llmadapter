package openairesponses

import "encoding/json"

type Request struct {
	Model                string          `json:"model"`
	Input                []InputItem     `json:"input,omitempty"`
	Instructions         string          `json:"instructions,omitempty"`
	MaxOutputTokens      *int            `json:"max_output_tokens,omitempty"`
	Temperature          *float64        `json:"temperature,omitempty"`
	TopP                 *float64        `json:"top_p,omitempty"`
	Stream               bool            `json:"stream,omitempty"`
	User                 string          `json:"user,omitempty"`
	PreviousResponseID   string          `json:"previous_response_id,omitempty"`
	Store                *bool           `json:"store,omitempty"`
	PromptCacheKey       string          `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string          `json:"prompt_cache_retention,omitempty"`
	Text                 TextConfig      `json:"text,omitempty"`
	Tools                []Tool          `json:"tools,omitempty"`
	ToolChoice           json.RawMessage `json:"tool_choice,omitempty"`
	OpenRouterModels     json.RawMessage `json:"models,omitempty"`
	OpenRouterRoute      json.RawMessage `json:"route,omitempty"`
	OpenRouterProvider   json.RawMessage `json:"provider,omitempty"`
	OpenRouterPrefs      json.RawMessage `json:"provider_preferences,omitempty"`
	OpenRouterPlugins    json.RawMessage `json:"plugins,omitempty"`
	OpenRouterDebug      json.RawMessage `json:"debug,omitempty"`
	OpenRouterTrace      json.RawMessage `json:"trace,omitempty"`
	OpenRouterSessionID  json.RawMessage `json:"session_id,omitempty"`
}

type TextConfig struct {
	Format json.RawMessage `json:"format,omitempty"`
}

type InputItem struct {
	Type      string        `json:"type"`
	Role      string        `json:"role,omitempty"`
	ID        string        `json:"id,omitempty"`
	Status    string        `json:"status,omitempty"`
	Content   []ContentPart `json:"content,omitempty"`
	CallID    string        `json:"call_id,omitempty"`
	Name      string        `json:"name,omitempty"`
	Arguments string        `json:"arguments,omitempty"`
	Output    string        `json:"output,omitempty"`
}

type ContentPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	FileID   string `json:"file_id,omitempty"`
}

type Tool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type Response struct {
	ID     string       `json:"id,omitempty"`
	Object string       `json:"object"`
	Model  string       `json:"model,omitempty"`
	Status string       `json:"status,omitempty"`
	Output []OutputItem `json:"output,omitempty"`
	Usage  Usage        `json:"usage,omitempty"`
	Error  *ErrorBody   `json:"error,omitempty"`
}

type OutputItem struct {
	ID        string        `json:"id,omitempty"`
	Type      string        `json:"type"`
	Role      string        `json:"role,omitempty"`
	Status    string        `json:"status,omitempty"`
	Content   []ContentPart `json:"content,omitempty"`
	CallID    string        `json:"call_id,omitempty"`
	Name      string        `json:"name,omitempty"`
	Arguments string        `json:"arguments,omitempty"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}

type ErrorBody struct {
	Type    string `json:"type,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type Event struct {
	Type         string       `json:"type,omitempty"`
	ResponseID   string       `json:"response_id,omitempty"`
	OutputIndex  int          `json:"output_index,omitempty"`
	ContentIndex int          `json:"content_index,omitempty"`
	Delta        string       `json:"delta,omitempty"`
	Response     *Response    `json:"response,omitempty"`
	Item         *OutputItem  `json:"item,omitempty"`
	Part         *ContentPart `json:"part,omitempty"`
	Arguments    string       `json:"arguments,omitempty"`
	Error        *ErrorBody   `json:"error,omitempty"`
}
