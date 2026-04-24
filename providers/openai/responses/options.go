package responses

import (
	"strings"

	openrouterresponses "github.com/codewandler/llmadapter/providers/openrouter/responses"
	"github.com/codewandler/llmadapter/transport"
)

type Option interface {
	applyOpenAIResponses(*config)
}

type config struct {
	apiKey    string
	baseURL   string
	transport transport.ByteStreamTransport
}

type optionFunc func(*config)

func (f optionFunc) applyOpenAIResponses(c *config) { f(c) }

func WithAPIKey(key string) Option {
	return optionFunc(func(c *config) {
		c.apiKey = key
	})
}

func WithBaseURL(url string) Option {
	return optionFunc(func(c *config) {
		c.baseURL = strings.TrimRight(url, "/")
	})
}

func WithTransport(t transport.ByteStreamTransport) Option {
	return optionFunc(func(c *config) {
		c.transport = t
	})
}

func openRouterOptions(cfg config) []openrouterresponses.Option {
	opts := []openrouterresponses.Option{
		openrouterresponses.WithAPIKey(cfg.apiKey),
		openrouterresponses.WithBaseURL(cfg.baseURL),
		openrouterresponses.WithWarningSource("openai.responses"),
	}
	if cfg.transport != nil {
		opts = append(opts, openrouterresponses.WithTransport(cfg.transport))
	}
	return opts
}
