package responses

import "encoding/json"

type requestWire struct {
	Model               string          `json:"model"`
	Input               []inputItemWire `json:"input,omitempty"`
	Instructions        string          `json:"instructions,omitempty"`
	MaxOutputTokens     *int            `json:"max_output_tokens,omitempty"`
	Temperature         *float64        `json:"temperature,omitempty"`
	TopP                *float64        `json:"top_p,omitempty"`
	TopK                *int            `json:"top_k,omitempty"`
	Stream              bool            `json:"stream,omitempty"`
	User                string          `json:"user,omitempty"`
	Text                textConfigWire  `json:"text,omitempty"`
	Tools               []toolWire      `json:"tools,omitempty"`
	ToolChoice          any             `json:"tool_choice,omitempty"`
	OpenRouterModels    json.RawMessage `json:"models,omitempty"`
	OpenRouterRoute     json.RawMessage `json:"route,omitempty"`
	OpenRouterProvider  json.RawMessage `json:"provider,omitempty"`
	OpenRouterPrefs     json.RawMessage `json:"provider_preferences,omitempty"`
	OpenRouterPlugins   json.RawMessage `json:"plugins,omitempty"`
	OpenRouterDebug     json.RawMessage `json:"debug,omitempty"`
	OpenRouterTrace     json.RawMessage `json:"trace,omitempty"`
	OpenRouterSessionID json.RawMessage `json:"session_id,omitempty"`
}

type textConfigWire struct {
	Format any `json:"format,omitempty"`
}

type inputItemWire struct {
	Type      string            `json:"type"`
	Role      string            `json:"role,omitempty"`
	ID        string            `json:"id,omitempty"`
	Status    string            `json:"status,omitempty"`
	Content   []contentPartWire `json:"content,omitempty"`
	CallID    string            `json:"call_id,omitempty"`
	Name      string            `json:"name,omitempty"`
	Arguments string            `json:"arguments,omitempty"`
	Output    string            `json:"output,omitempty"`
}

type contentPartWire struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	FileID   string `json:"file_id,omitempty"`
}

type toolWire struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type eventWire struct {
	Type         string           `json:"type,omitempty"`
	ResponseID   string           `json:"response_id,omitempty"`
	OutputIndex  int              `json:"output_index,omitempty"`
	ContentIndex int              `json:"content_index,omitempty"`
	Delta        string           `json:"delta,omitempty"`
	Response     *responseWire    `json:"response,omitempty"`
	Item         *outputItemWire  `json:"item,omitempty"`
	Part         *contentPartWire `json:"part,omitempty"`
	Arguments    string           `json:"arguments,omitempty"`
	Error        *errorWire       `json:"error,omitempty"`
	Raw          json.RawMessage  `json:"-"`
}

type responseWire struct {
	ID     string     `json:"id,omitempty"`
	Model  string     `json:"model,omitempty"`
	Status string     `json:"status,omitempty"`
	Usage  *usageWire `json:"usage,omitempty"`
	Error  *errorWire `json:"error,omitempty"`
}

type outputItemWire struct {
	ID        string            `json:"id,omitempty"`
	Type      string            `json:"type,omitempty"`
	Role      string            `json:"role,omitempty"`
	Status    string            `json:"status,omitempty"`
	Content   []contentPartWire `json:"content,omitempty"`
	CallID    string            `json:"call_id,omitempty"`
	Name      string            `json:"name,omitempty"`
	Arguments string            `json:"arguments,omitempty"`
}

type usageWire struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	TotalTokens  int `json:"total_tokens,omitempty"`
}

type errorWire struct {
	Type    string `json:"type,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
