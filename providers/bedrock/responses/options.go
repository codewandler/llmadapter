package responses

import (
	"strings"

	"github.com/codewandler/llmadapter/providers/bedrock/internal/mantleauth"
	"github.com/codewandler/llmadapter/transport"
)

type Option interface {
	applyBedrockResponses(*config)
}

type config struct {
	apiKey        string
	baseURL       string
	warningSource string
	tokenProvider TokenProvider
	transport     transport.ByteStreamTransport
}

type optionFunc func(*config)

func (f optionFunc) applyBedrockResponses(c *config) { f(c) }

func WithAPIKey(key string) Option {
	return optionFunc(func(c *config) {
		c.apiKey = key
	})
}

func WithTokenProvider(provider TokenProvider) Option {
	return optionFunc(func(c *config) {
		c.tokenProvider = provider
	})
}

type TokenProvider = mantleauth.TokenProvider
type TokenProviderFunc = mantleauth.TokenProviderFunc
type AWSTokenProvider = mantleauth.AWSTokenProvider

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

func WithWarningSource(source string) Option {
	return optionFunc(func(c *config) {
		c.warningSource = source
	})
}

func normalizeBaseURL(url string) string {
	url = strings.TrimRight(strings.TrimSpace(url), "/")
	return strings.TrimSuffix(url, "/v1")
}
