package responses

import (
	"strings"

	"github.com/codewandler/llmadapter/transport"
)

type Option interface {
	applyOpenRouterResponses(*config)
}

type config struct {
	apiKey        string
	baseURL       string
	warningSource string
	transport     transport.ByteStreamTransport
}

type optionFunc func(*config)

func (f optionFunc) applyOpenRouterResponses(c *config) { f(c) }

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

func WithWarningSource(source string) Option {
	return optionFunc(func(c *config) {
		c.warningSource = source
	})
}
