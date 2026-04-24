package chatcompletions

import (
	"strings"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/transport"
)

type Option interface {
	applyOpenAI(*config)
}

type config struct {
	apiKey    string
	baseURL   string
	apiKind   adapt.ApiKind
	transport transport.ByteStreamTransport
}

type optionFunc func(*config)

func (f optionFunc) applyOpenAI(c *config) { f(c) }

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

func WithAPIKind(kind adapt.ApiKind) Option {
	return optionFunc(func(c *config) {
		c.apiKind = kind
	})
}
