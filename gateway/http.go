package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
)

type Endpoint interface {
	DecodeHTTP(ctx context.Context, r *http.Request) (adapt.Request, error)
	WriteEvents(ctx context.Context, w http.ResponseWriter, req adapt.Request, events <-chan unified.Event) error
	WriteError(ctx context.Context, w http.ResponseWriter, err error) error
}

type Handler struct {
	Endpoint Endpoint
	Router   router.Router
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tw := &trackingResponseWriter{ResponseWriter: w}
	req, err := h.Endpoint.DecodeHTTP(ctx, r)
	if err != nil {
		_ = h.Endpoint.WriteError(ctx, tw, err)
		return
	}
	routes, err := routeCandidates(ctx, h.Router, req)
	if err != nil {
		_ = h.Endpoint.WriteError(ctx, tw, err)
		return
	}
	var failures []error
	for _, route := range routes {
		attempt := req
		if route.NativeModel != "" {
			attempt.Unified.Model = route.NativeModel
		}
		events, err := route.Client.Request(ctx, attempt.Unified)
		if err != nil {
			failures = append(failures, routeError(route, err))
			continue
		}
		if err := h.Endpoint.WriteEvents(ctx, tw, attempt, events); err != nil {
			if tw.wrote {
				return
			}
			failures = append(failures, routeError(route, err))
			continue
		}
		return
	}
	if err := errors.Join(failures...); err != nil {
		_ = h.Endpoint.WriteError(ctx, tw, err)
		return
	}
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

func routeError(route router.Route, err error) error {
	return fmt.Errorf("provider %s/%s failed: %w", route.ProviderName, route.TargetAPI, err)
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
