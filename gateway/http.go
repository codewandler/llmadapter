package gateway

import (
	"context"
	"net/http"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/unified"
)

type Endpoint interface {
	DecodeHTTP(ctx context.Context, r *http.Request) (adapt.Request, error)
	WriteEvents(ctx context.Context, w http.ResponseWriter, req adapt.Request, events <-chan unified.Event) error
	WriteError(ctx context.Context, w http.ResponseWriter, err error) error
}

type Handler struct {
	Endpoint Endpoint
	Client   unified.Client
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tw := &trackingResponseWriter{ResponseWriter: w}
	req, err := h.Endpoint.DecodeHTTP(ctx, r)
	if err != nil {
		_ = h.Endpoint.WriteError(ctx, tw, err)
		return
	}
	events, err := h.Client.Request(ctx, req.Unified)
	if err != nil {
		_ = h.Endpoint.WriteError(ctx, tw, err)
		return
	}
	if err := h.Endpoint.WriteEvents(ctx, tw, req, events); err != nil && !tw.wrote {
		_ = h.Endpoint.WriteError(ctx, tw, err)
		return
	}
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
