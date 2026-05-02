package messages

import (
	"context"
	"strings"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/modelmeta"
	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

const defaultBaseURL = "https://openrouter.ai/api"

type Option interface {
	applyOpenRouterMessages(*config)
}

type config struct {
	apiKey    string
	baseURL   string
	transport transport.ByteStreamTransport
}

type optionFunc func(*config)

func (f optionFunc) applyOpenRouterMessages(c *config) { f(c) }

func NewClient(opts ...Option) (unified.Client, error) {
	cfg := config{baseURL: defaultBaseURL}
	for _, opt := range opts {
		opt.applyOpenRouterMessages(&cfg)
	}
	anthropicOpts := []anthropic.Option{
		anthropic.WithAPIKey(cfg.apiKey),
		anthropic.WithBaseURL(cfg.baseURL),
		anthropic.WithHeader("Authorization", "Bearer "+cfg.apiKey),
		anthropic.WithoutBuiltInModelMetadata(),
		anthropic.WithRequestProcessor(modelmeta.BuiltInRequestMetadataProcessor("openrouter", adapt.FamilyAnthropicMessages)),
		anthropic.WithRequestProcessor(requestProcessorFunc(func(ctx context.Context, req *adapt.Request) error {
			req.SourceAPI = adapt.ApiOpenRouterAnthropicMessages
			req.Unified.Stream = true
			return nil
		})),
		anthropic.WithUnifiedEventProcessor(rawAPIKindProcessor{apiKind: string(adapt.ApiOpenRouterAnthropicMessages)}),
	}
	if cfg.transport != nil {
		anthropicOpts = append(anthropicOpts, anthropic.WithTransport(cfg.transport))
	}
	return anthropic.NewClient(anthropicOpts...)
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

type requestProcessorFunc func(context.Context, *adapt.Request) error

func (f requestProcessorFunc) ProcessRequest(ctx context.Context, req *adapt.Request) error {
	return f(ctx, req)
}

type rawAPIKindProcessor struct {
	apiKind string
}

func (p rawAPIKindProcessor) Push(ctx context.Context, ev unified.Event) ([]unified.Event, error) {
	if raw, ok := ev.(unified.RawEvent); ok && raw.APIKind == string(adapt.ApiAnthropicMessages) {
		raw.APIKind = p.apiKind
		return []unified.Event{raw}, nil
	}
	return []unified.Event{ev}, nil
}

func (p rawAPIKindProcessor) Close(ctx context.Context) ([]unified.Event, error) {
	return nil, nil
}
