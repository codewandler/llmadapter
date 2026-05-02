package messages

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/modelmeta"
	"github.com/codewandler/llmadapter/pipeline"
	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

const (
	defaultBaseURL = "https://api.anthropic.com"
	defaultVersion = "2023-06-01"
)

type Option interface {
	applyAnthropic(*Config) error
}

type Config struct {
	APIKey       string
	BaseURL      string
	Version      string
	Betas        []string
	NoAPIKeyAuth bool

	Headers       http.Header
	HeaderFns     []HeaderFunc
	Transport     transport.ByteStreamTransport
	QuotaProvider string

	RequestProcessors         []adapt.RequestProcessor
	ProviderRequestProcessors []adapt.ProviderRequestProcessor[MessageRequest]
	ProviderEventProcessors   []pipeline.Processor[Event]
	UnifiedEventProcessors    []pipeline.Processor[unified.Event]
	ClaudeHeaders             bool
	BuiltInModelMetadata      bool
}

func NewClient(opts ...Option) (unified.Client, error) {
	cfg := Config{
		BaseURL:              defaultBaseURL,
		Version:              defaultVersion,
		Headers:              make(http.Header),
		QuotaProvider:        "anthropic",
		BuiltInModelMetadata: true,
	}
	for _, opt := range opts {
		if err := opt.applyAnthropic(&cfg); err != nil {
			return nil, err
		}
	}
	if cfg.APIKey == "" && !cfg.NoAPIKeyAuth {
		return nil, fmt.Errorf("anthropic API key is required")
	}
	if cfg.Transport == nil {
		cfg.Transport = transport.NewDefaultRetryTransport(transport.NewHTTPByteStreamTransport(transport.HTTPTransportConfig{FrameFormat: transport.FrameFormatSSE}))
	}
	if len(cfg.Betas) > 0 {
		cfg.Headers.Set("anthropic-beta", strings.Join(cfg.Betas, ","))
	}
	if cfg.ClaudeHeaders {
		cfg.HeaderFns = append(cfg.HeaderFns, claudeHeadersFunc())
	}
	if cfg.BuiltInModelMetadata {
		cfg.RequestProcessors = append([]adapt.RequestProcessor{
			modelmeta.BuiltInRequestMetadataProcessor("anthropic", adapt.FamilyAnthropicMessages),
		}, cfg.RequestProcessors...)
	}

	native := &NativeClient{
		transport:     cfg.Transport,
		baseURL:       cfg.BaseURL,
		apiKey:        cfg.APIKey,
		version:       cfg.Version,
		headers:       cfg.Headers,
		headerFns:     cfg.HeaderFns,
		quotaProvider: cfg.QuotaProvider,
	}
	return &AdaptedClient{
		native:          native,
		codec:           Codec{},
		reqProcs:        cfg.RequestProcessors,
		provReqProcs:    cfg.ProviderRequestProcessors,
		provEvtProcs:    cfg.ProviderEventProcessors,
		unifiedEvtProcs: cfg.UnifiedEventProcessors,
	}, nil
}

type optionFunc func(*Config) error

func (f optionFunc) applyAnthropic(c *Config) error { return f(c) }

func WithAPIKey(key string) Option {
	return optionFunc(func(c *Config) error {
		c.APIKey = key
		c.NoAPIKeyAuth = false
		return nil
	})
}

func WithBaseURL(url string) Option {
	return optionFunc(func(c *Config) error {
		c.BaseURL = strings.TrimRight(url, "/")
		return nil
	})
}

func WithVersion(version string) Option {
	return optionFunc(func(c *Config) error {
		c.Version = version
		return nil
	})
}

func WithBeta(beta string) Option {
	return optionFunc(func(c *Config) error {
		c.Betas = append(c.Betas, beta)
		return nil
	})
}

func WithHeader(key, value string) Option {
	return optionFunc(func(c *Config) error {
		if c.Headers == nil {
			c.Headers = make(http.Header)
		}
		c.Headers.Add(key, value)
		return nil
	})
}

func WithHeaderFunc(fn HeaderFunc) Option {
	return optionFunc(func(c *Config) error {
		c.HeaderFns = append(c.HeaderFns, fn)
		return nil
	})
}

func WithTransport(t transport.ByteStreamTransport) Option {
	return optionFunc(func(c *Config) error {
		c.Transport = t
		return nil
	})
}

func WithRequestProcessor(p adapt.RequestProcessor) Option {
	return optionFunc(func(c *Config) error {
		c.RequestProcessors = append(c.RequestProcessors, p)
		return nil
	})
}

func WithUnifiedEventProcessor(p pipeline.Processor[unified.Event]) Option {
	return optionFunc(func(c *Config) error {
		c.UnifiedEventProcessors = append(c.UnifiedEventProcessors, p)
		return nil
	})
}

func WithoutBuiltInModelMetadata() Option {
	return optionFunc(func(c *Config) error {
		c.BuiltInModelMetadata = false
		return nil
	})
}

func WithProviderRequestProcessor(p adapt.ProviderRequestProcessor[MessageRequest]) Option {
	return optionFunc(func(c *Config) error {
		c.ProviderRequestProcessors = append(c.ProviderRequestProcessors, p)
		return nil
	})
}

func WithProviderEventProcessor(p pipeline.Processor[Event]) Option {
	return optionFunc(func(c *Config) error {
		c.ProviderEventProcessors = append(c.ProviderEventProcessors, p)
		return nil
	})
}
