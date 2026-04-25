package muxclient

import (
	"context"
	"errors"
	"fmt"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/internal/routeattempt"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
)

type Client struct {
	router      router.Router
	source      adapt.ApiKind
	fallback    bool
	maxAttempts int
}

type Option func(*Client)

func New(r router.Router, opts ...Option) *Client {
	c := &Client{router: r, fallback: true}
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

func WithMaxAttempts(max int) Option {
	return func(c *Client) {
		if max > 0 {
			c.maxAttempts = max
		}
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
	routes, err := routeattempt.Candidates(ctx, c.router, adaptReq)
	if err != nil {
		return nil, err
	}
	var failures []error
	attempts := 0
	for _, route := range routes {
		if routeattempt.ReachedMaxAttempts(attempts, c.maxAttempts) {
			break
		}
		attempts++
		attempt := routeattempt.RequestForRoute(req, route)
		events, err := route.Client.Request(ctx, attempt)
		if err == nil {
			return prependRouteEvent(ctx, route, events), nil
		}
		failures = append(failures, routeattempt.Error(route, err))
		if !c.fallback || !routeattempt.Retryable(err) {
			break
		}
	}
	if err := errors.Join(failures...); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("muxclient: no route attempted for api %q model %q", c.source, req.Model)
}

func prependRouteEvent(ctx context.Context, route router.Route, events <-chan unified.Event) <-chan unified.Event {
	out := make(chan unified.Event)
	go func() {
		defer close(out)
		routeEvent := unified.RouteEvent{
			SourceAPI:    string(route.SourceAPI),
			TargetAPI:    string(route.TargetAPI),
			TargetFamily: string(route.TargetFamily),
			ProviderName: route.ProviderName,
			PublicModel:  route.PublicModel,
			NativeModel:  route.NativeModel,
		}
		select {
		case <-ctx.Done():
			return
		case out <- routeEvent:
		}
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-events:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case out <- ev:
				}
			}
		}
	}()
	return out
}
