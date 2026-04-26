package responses

import (
	"strings"

	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

type Option interface {
	applyOpenAIResponses(*config)
}

type config struct {
	apiKey                     string
	baseURL                    string
	warningSource              string
	supportsPreviousResponseID bool
	bodyMutator                func(unified.Request, []byte) ([]byte, []unified.WarningEvent, error)
	transport                  transport.ByteStreamTransport
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

func WithWarningSource(source string) Option {
	return optionFunc(func(c *config) {
		c.warningSource = source
	})
}

func WithPreviousResponseIDSupport(supported bool) Option {
	return optionFunc(func(c *config) {
		c.supportsPreviousResponseID = supported
	})
}

func WithBodyMutator(mutator func(unified.Request, []byte) ([]byte, []unified.WarningEvent, error)) Option {
	return optionFunc(func(c *config) {
		c.bodyMutator = mutator
	})
}
