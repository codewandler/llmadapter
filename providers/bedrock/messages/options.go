package messages

import (
	"context"
	"strings"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/modelmeta"
	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	"github.com/codewandler/llmadapter/providers/bedrock/internal/mantleauth"
	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

const (
	EnvAPIKey        = mantleauth.EnvAPIKey
	EnvBearerToken   = mantleauth.EnvBearerToken
	EnvRegion        = mantleauth.EnvRegion
	EnvDefaultRegion = mantleauth.EnvDefaultRegion
	EnvModel         = "BEDROCK_MESSAGES_MODEL"
	DefaultModel     = "anthropic.claude-opus-4-7"
	defaultAPIKind   = "bedrock.anthropic_messages"
)

type Option interface {
	applyBedrockMessages(*config)
}

type config struct {
	apiKey        string
	baseURL       string
	tokenProvider mantleauth.TokenProvider
	transport     transport.ByteStreamTransport
}

type optionFunc func(*config)

func (f optionFunc) applyBedrockMessages(c *config) { f(c) }

func NewClient(opts ...Option) (unified.Client, error) {
	cfg := config{apiKey: mantleauth.ExplicitTokenFromEnv(), baseURL: defaultBaseURL()}
	for _, opt := range opts {
		opt.applyBedrockMessages(&cfg)
	}
	provider := cfg.tokenProvider
	if provider == nil {
		if cfg.apiKey != "" {
			provider = mantleauth.TokenProviderFunc(func(context.Context) (string, error) { return cfg.apiKey, nil })
		} else {
			provider = mantleauth.NewAWSTokenProvider(defaultRegionFromEnv())
		}
	}

	anthropicOpts := []anthropic.Option{
		anthropic.WithBearerTokenProvider(anthropicTokenProvider{provider: provider}),
		anthropic.WithBaseURL(cfg.baseURL),
		anthropic.WithoutBuiltInModelMetadata(),
		anthropic.WithRequestProcessor(modelmeta.BuiltInRequestMetadataProcessor("bedrock", adapt.FamilyAnthropicMessages)),
		anthropic.WithProviderRequestProcessor(bedrockThinkingProcessor{}),
		anthropic.WithRequestProcessor(requestProcessorFunc(func(ctx context.Context, req *adapt.Request) error {
			req.SourceAPI = adapt.ApiBedrockAnthropicMessages
			req.Unified.Stream = true
			return nil
		})),
		anthropic.WithUnifiedEventProcessor(rawAPIKindProcessor{apiKind: defaultAPIKind}),
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

func WithTokenProvider(provider mantleauth.TokenProvider) Option {
	return optionFunc(func(c *config) {
		c.tokenProvider = provider
	})
}

func WithBaseURL(url string) Option {
	return optionFunc(func(c *config) {
		c.baseURL = normalizeBaseURL(url)
	})
}

func WithTransport(t transport.ByteStreamTransport) Option {
	return optionFunc(func(c *config) {
		c.transport = t
	})
}

func normalizeBaseURL(url string) string {
	url = strings.TrimRight(strings.TrimSpace(url), "/")
	if strings.HasSuffix(url, "/anthropic/v1/messages") {
		return strings.TrimSuffix(url, "/v1/messages")
	}
	url = strings.TrimSuffix(url, "/v1/messages")
	if !strings.HasSuffix(url, "/anthropic") {
		url += "/anthropic"
	}
	return url
}

func defaultBaseURL() string {
	return "https://bedrock-mantle." + defaultRegionFromEnv() + ".api.aws/anthropic"
}

func defaultRegionFromEnv() string {
	return mantleauth.RegionFromEnv(mantleauth.DefaultRegion)
}

type anthropicTokenProvider struct {
	provider mantleauth.TokenProvider
}

func (p anthropicTokenProvider) Token(ctx context.Context) (*anthropic.Token, error) {
	token, err := p.provider.Token(ctx)
	if err != nil {
		return nil, err
	}
	return anthropic.NewStaticBearerToken(token), nil
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

type bedrockThinkingProcessor struct{}

func (p bedrockThinkingProcessor) ProcessProviderRequest(ctx context.Context, req *anthropic.MessageRequest) error {
	if req.Thinking == nil || req.Thinking.Type != "enabled" {
		return nil
	}
	req.Thinking.Type = "adaptive"
	req.Thinking.BudgetTokens = 0
	if req.OutputConfig == nil {
		req.OutputConfig = &anthropic.OutputConfig{}
	}
	if req.OutputConfig.Effort == "" {
		req.OutputConfig.Effort = "high"
	}
	return nil
}
