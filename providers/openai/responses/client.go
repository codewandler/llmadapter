package responses

import (
	"fmt"

	openrouterresponses "github.com/codewandler/llmadapter/providers/openrouter/responses"
	"github.com/codewandler/llmadapter/unified"
)

const defaultBaseURL = "https://api.openai.com"

func NewClient(opts ...Option) (unified.Client, error) {
	cfg := config{baseURL: defaultBaseURL}
	for _, opt := range opts {
		opt.applyOpenAIResponses(&cfg)
	}
	if cfg.apiKey == "" {
		return nil, fmt.Errorf("openai API key is required")
	}
	return openrouterresponses.NewClient(openRouterOptions(cfg)...)
}
