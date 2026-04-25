package gateway

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/internal/routeattempt"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
)

type Endpoint interface {
	DecodeHTTP(ctx context.Context, r *http.Request) (adapt.Request, error)
	WriteEvents(ctx context.Context, w http.ResponseWriter, req adapt.Request, events <-chan unified.Event) error
	WriteError(ctx context.Context, w http.ResponseWriter, err error) error
}

type Handler struct {
	Endpoint    Endpoint
	Router      router.Router
	Health      *HealthTracker
	MaxAttempts int
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tw := &trackingResponseWriter{ResponseWriter: w}
	req, err := h.Endpoint.DecodeHTTP(ctx, r)
	if err != nil {
		_ = h.Endpoint.WriteError(ctx, tw, err)
		return
	}
	routes, err := routeattempt.Candidates(ctx, h.Router, req)
	if err != nil {
		_ = h.Endpoint.WriteError(ctx, tw, err)
		return
	}
	var failures []error
	attempts := 0
	for _, route := range orderByHealth(routes, h.Health) {
		if routeattempt.ReachedMaxAttempts(attempts, h.MaxAttempts) {
			break
		}
		attempts++
		attempt := req
		attempt.Unified = routeattempt.RequestForRoute(req.Unified, route)
		events, err := route.Client.Request(ctx, attempt.Unified)
		if err != nil {
			h.Health.MarkFailure(route)
			failures = append(failures, routeattempt.Error(route, err))
			if !routeattempt.Retryable(err) {
				break
			}
			continue
		}
		if err := h.Endpoint.WriteEvents(ctx, tw, attempt, events); err != nil {
			if tw.wrote {
				h.Health.MarkFailure(route)
				return
			}
			h.Health.MarkFailure(route)
			failures = append(failures, routeattempt.Error(route, err))
			if !routeattempt.Retryable(err) {
				break
			}
			continue
		}
		h.Health.MarkSuccess(route)
		return
	}
	if err := errors.Join(failures...); err != nil {
		_ = h.Endpoint.WriteError(ctx, tw, err)
		return
	}
}

func orderByHealth(routes []router.Route, health *HealthTracker) []router.Route {
	if health == nil || len(routes) < 2 {
		return routes
	}
	now := time.Now()
	ordered := make([]router.Route, 0, len(routes))
	var unhealthy []router.Route
	for _, route := range routes {
		if health.unhealthy(route, now) {
			unhealthy = append(unhealthy, route)
			continue
		}
		ordered = append(ordered, route)
	}
	return append(ordered, unhealthy...)
}

type trackingResponseWriter struct {
	http.ResponseWriter
	wrote bool
}

func (w *trackingResponseWriter) WriteHeader(statusCode int) {
	w.wrote = true
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *trackingResponseWriter) Write(b []byte) (int, error) {
	w.wrote = true
	return w.ResponseWriter.Write(b)
}
