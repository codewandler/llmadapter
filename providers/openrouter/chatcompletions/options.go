package chatcompletions

import (
	"fmt"
	"strings"

	openai "github.com/codewandler/llmadapter/providers/openai/chatcompletions"
	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

const defaultBaseURL = "https://openrouter.ai/api"

type Option interface {
	applyOpenRouter(*config)
}

type config struct {
	apiKey    string
	baseURL   string
	transport transport.ByteStreamTransport
}

type optionFunc func(*config)

func (f optionFunc) applyOpenRouter(c *config) { f(c) }

func NewClient(opts ...Option) (unified.Client, error) {
	cfg := config{baseURL: defaultBaseURL}
	for _, opt := range opts {
		opt.applyOpenRouter(&cfg)
	}
	if cfg.apiKey == "" {
		return nil, fmt.Errorf("openrouter API key is required")
	}
	openAIOpts := []openai.Option{
		openai.WithAPIKey(cfg.apiKey),
		openai.WithBaseURL(cfg.baseURL),
	}
	if cfg.transport != nil {
		openAIOpts = append(openAIOpts, openai.WithTransport(cfg.transport))
	}
	return openai.NewClient(openAIOpts...)
}

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
