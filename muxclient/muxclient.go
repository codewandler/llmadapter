package muxclient

import (
	"context"
	"errors"
	"fmt"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
)

type Client struct {
	router   router.Router
	source   adapt.ApiKind
	fallback bool
}

type Option func(*Client)

func New(r router.Router, opts ...Option) *Client {
	c := &Client{router: r, source: adapt.ApiOpenAIResponses, fallback: true}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

func WithSourceAPI(api adapt.ApiKind) Option {
	return func(c *Client) {
		if api != "" {
			c.source = api
		}
	}
}

func WithFallback(enabled bool) Option {
	return func(c *Client) {
		c.fallback = enabled
	}
}

func (c *Client) Request(ctx context.Context, req unified.Request) (<-chan unified.Event, error) {
	if c == nil || c.router == nil {
		return nil, fmt.Errorf("muxclient: router is required")
	}
	adaptReq := adapt.Request{
		SourceAPI: c.source,
		Unified:   req,
	}
	routes, err := routeCandidates(ctx, c.router, adaptReq)
	if err != nil {
		return nil, err
	}
	var failures []error
	for _, route := range routes {
		attempt := req
		if route.NativeModel != "" {
			attempt.Model = route.NativeModel
		}
		events, err := route.Client.Request(ctx, attempt)
		if err == nil {
			return events, nil
		}
		failures = append(failures, fmt.Errorf("provider %s/%s failed: %w", route.ProviderName, route.TargetAPI, err))
		if !c.fallback {
			break
		}
	}
	if err := errors.Join(failures...); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("muxclient: no route attempted for api %q model %q", c.source, req.Model)
}

func routeCandidates(ctx context.Context, r router.Router, req adapt.Request) ([]router.Route, error) {
	if candidateRouter, ok := r.(router.CandidateRouter); ok {
		return candidateRouter.Routes(ctx, req)
	}
	route, err := r.Route(ctx, req)
	if err != nil {
		return nil, err
	}
	return []router.Route{route}, nil
}
