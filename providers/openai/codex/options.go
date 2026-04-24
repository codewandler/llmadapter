package codex

import (
	"net/http"
	"strings"

	"github.com/codewandler/llmadapter/transport"
)

type Option interface {
	applyCodex(*config)
}

type config struct {
	baseURL        string
	path           string
	accessToken    string
	authPath       string
	installationID string
	betaFeatures   string
	transport      transport.ByteStreamTransport
	httpClient     *http.Client
}

type optionFunc func(*config)

func (f optionFunc) applyCodex(c *config) { f(c) }

func WithBaseURL(url string) Option {
	return optionFunc(func(c *config) {
		c.baseURL = strings.TrimRight(url, "/")
	})
}

func WithPath(path string) Option {
	return optionFunc(func(c *config) {
		c.path = path
	})
}

func WithAccessToken(token string) Option {
	return optionFunc(func(c *config) {
		c.accessToken = token
	})
}

func WithAuthPath(path string) Option {
	return optionFunc(func(c *config) {
		c.authPath = path
	})
}

func WithInstallationID(id string) Option {
	return optionFunc(func(c *config) {
		c.installationID = id
	})
}

func WithBetaFeatures(features string) Option {
	return optionFunc(func(c *config) {
		c.betaFeatures = features
	})
}

func WithTransport(t transport.ByteStreamTransport) Option {
	return optionFunc(func(c *config) {
		c.transport = t
	})
}

func WithHTTPClient(client *http.Client) Option {
	return optionFunc(func(c *config) {
		c.httpClient = client
	})
}
