package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

const inferDebugMaxBodyBytes = 256 * 1024

const (
	inferDebugModeHTTPSSE   = "http_sse"
	inferDebugModeWebSocket = "websocket"
)

type inferDebugScopes struct {
	enabled  bool
	request  bool
	response bool
	stream   bool
	events   bool
}

func parseInferDebugScopes(values []string) (inferDebugScopes, error) {
	var scopes inferDebugScopes
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			scope := strings.ToLower(strings.TrimSpace(part))
			if scope == "" {
				continue
			}
			scopes.enabled = true
			switch scope {
			case "all":
				scopes.request = true
				scopes.response = true
				scopes.stream = true
				scopes.events = true
			case "request", "requests":
				scopes.request = true
			case "response", "responses":
				scopes.response = true
			case "stream", "streams":
				scopes.stream = true
			case "event", "events":
				scopes.events = true
			default:
				return inferDebugScopes{}, fmt.Errorf("invalid debug scope %q; valid scopes: all, request, response, stream, events", scope)
			}
		}
	}
	return scopes, nil
}

func (s inferDebugScopes) httpEnabled() bool {
	return s.request || s.response || s.stream
}

type inferDebugTransport struct {
	inner  transport.ByteStreamTransport
	w      io.Writer
	scopes inferDebugScopes
	mode   string
	seq    atomic.Uint64
}

func newInferDebugTransport(w io.Writer, scopes inferDebugScopes) *inferDebugTransport {
	return newInferDebugTransportWithInner(w, scopes, inferDebugModeHTTPSSE, transport.NewHTTPByteStreamTransport(transport.HTTPTransportConfig{FrameFormat: transport.FrameFormatSSE}))
}

func newInferDebugWebSocketTransport(w io.Writer, scopes inferDebugScopes) *inferDebugTransport {
	return newInferDebugTransportWithInner(w, scopes, inferDebugModeWebSocket, transport.NewWebSocketByteStreamTransport(transport.WebSocketTransportConfig{
		EnableCompression: true,
		ForceIPv4:         true,
	}))
}

func newInferDebugTransportWithInner(w io.Writer, scopes inferDebugScopes, mode string, inner transport.ByteStreamTransport) *inferDebugTransport {
	if w == nil {
		w = io.Discard
	}
	return &inferDebugTransport{
		inner:  inner,
		w:      w,
		scopes: scopes,
		mode:   mode,
	}
}

func (t *inferDebugTransport) Open(ctx context.Context, req *transport.Request) (transport.ByteStream, error) {
	id := t.seq.Add(1)
	reqForOpen := req
	var body []byte
	if req.Body != nil {
		var err error
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		copyReq := *req
		copyReq.Body = bytes.NewReader(body)
		reqForOpen = &copyReq
	}
	if t.scopes.request {
		t.logRequest(id, req, body)
	}
	stream, err := t.inner.Open(ctx, reqForOpen)
	if err != nil {
		if t.scopes.response {
			t.logResponseError(id, err)
		}
		return nil, err
	}
	if t.scopes.response {
		t.logResponse(id, stream)
	}
	if t.scopes.stream {
		stream = &inferDebugByteStream{id: id, inner: stream, w: t.w, mode: t.mode}
	}
	return stream, nil
}

func (t *inferDebugTransport) logRequest(id uint64, req *transport.Request, body []byte) {
	t.logf(id, "transport: %s\n", t.mode)
	t.logf(id, ">>> request %s %s\n", req.Method, req.URL)
	writeInferDebugHeaders(t.w, id, req.Header)
	if len(body) > 0 {
		contentType := req.Header.Get("Content-Type")
		if t.mode == inferDebugModeWebSocket {
			t.logBody(id, ">>> websocket frame", contentType, body)
		} else {
			t.logBody(id, ">>> request body", contentType, body)
		}
	}
}

func (t *inferDebugTransport) logResponse(id uint64, stream transport.ByteStream) {
	t.logf(id, "<<< response headers\n")
	if carrier, ok := stream.(transport.HeaderCarrier); ok {
		writeInferDebugHeaders(t.w, id, carrier.Header())
		return
	}
	t.logf(id, "  headers unavailable\n")
}

func (t *inferDebugTransport) logResponseError(id uint64, err error) {
	t.logf(id, "!!! response error: %v\n", err)
	var apiErr *unified.APIError
	if errors.As(err, &apiErr) && len(apiErr.ProviderRaw) > 0 {
		t.logBody(id, "!!! response error body", "application/json", apiErr.ProviderRaw)
	}
}

func (t *inferDebugTransport) logBody(id uint64, label string, contentType string, body []byte) {
	truncated := false
	if len(body) > inferDebugMaxBodyBytes {
		body = body[:inferDebugMaxBodyBytes]
		truncated = true
	}
	text := formatProxyBody(body, contentType)
	if truncated {
		text += "\n... truncated"
	}
	t.logf(id, "%s:\n%s\n", label, indentProxyBlock(text))
}

func (t *inferDebugTransport) logf(id uint64, format string, args ...any) {
	fmt.Fprintf(t.w, "[debug #%d] "+format, append([]any{id}, args...)...)
}

type inferDebugByteStream struct {
	id    uint64
	inner transport.ByteStream
	w     io.Writer
	mode  string
}

func (s *inferDebugByteStream) Recv(ctx context.Context) ([]byte, error) {
	frame, err := s.inner.Recv(ctx)
	if len(frame) > 0 {
		fmt.Fprintf(s.w, "[debug #%d] <<< %s frame:\n%s\n", s.id, s.mode, indentProxyBlock(formatInferDebugFrame(frame)))
	}
	if err != nil && err != io.EOF {
		fmt.Fprintf(s.w, "[debug #%d] !!! stream error: %v\n", s.id, err)
	}
	return frame, err
}

func (s *inferDebugByteStream) Write(ctx context.Context, frame []byte) error {
	writer, ok := s.inner.(transport.ByteStreamWriter)
	if !ok {
		return fmt.Errorf("%s stream does not support writes", s.mode)
	}
	if len(frame) > 0 {
		fmt.Fprintf(s.w, "[debug #%d] >>> %s frame:\n%s\n", s.id, s.mode, indentProxyBlock(formatInferDebugFrame(frame)))
	}
	return writer.Write(ctx, frame)
}

func (s *inferDebugByteStream) Close() error {
	return s.inner.Close()
}

func (s *inferDebugByteStream) Header() http.Header {
	if carrier, ok := s.inner.(transport.HeaderCarrier); ok {
		return carrier.Header()
	}
	return nil
}

func writeInferDebugHeaders(w io.Writer, id uint64, headers http.Header) {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		values := headers.Values(key)
		if isSensitiveProxyKey(key) {
			values = []string{"[redacted]"}
		}
		fmt.Fprintf(w, "[debug #%d]   %s: %s\n", id, key, strings.Join(values, ", "))
	}
}

func formatInferDebugFrame(frame []byte) string {
	if len(frame) == 0 {
		return ""
	}
	if json.Valid(frame) {
		var v any
		if err := json.Unmarshal(frame, &v); err == nil {
			pretty, err := json.MarshalIndent(redactProxyJSON(v), "", "  ")
			if err == nil {
				return string(pretty)
			}
		}
	}
	return redactProxyText(string(frame))
}

func writeInferDebugEvent(w io.Writer, ev unified.Event) {
	if w == nil {
		return
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		fmt.Fprintf(w, "[debug event] %T: %v\n", ev, ev)
		return
	}
	fmt.Fprintf(w, "[debug event] %T:\n%s\n", ev, indentProxyBlock(formatInferDebugFrame(payload)))
}

func writeInferDebugMode(w io.Writer, source string, transportKind unified.TransportKind, continuation unified.ContinuationMode) {
	if w == nil || transportKind == "" {
		return
	}
	if continuation != "" {
		fmt.Fprintf(w, "[debug mode] %s transport=%s internal_continuation=%s\n", source, transportKind, continuation)
		return
	}
	fmt.Fprintf(w, "[debug mode] %s transport=%s\n", source, transportKind)
}
