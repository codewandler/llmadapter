package diagnostics

import (
	"bufio"
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

const defaultMaxBodyBytes = 256 * 1024

const (
	ModeHTTPSSE   = "http_sse"
	ModeWebSocket = "websocket"
)

type Scopes struct {
	Enabled  bool
	Request  bool
	Response bool
	Stream   bool
	Events   bool
}

func ParseScopes(values []string) (Scopes, error) {
	var scopes Scopes
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			scope := strings.ToLower(strings.TrimSpace(part))
			if scope == "" {
				continue
			}
			scopes.Enabled = true
			switch scope {
			case "all":
				scopes.Request = true
				scopes.Response = true
				scopes.Stream = true
				scopes.Events = true
			case "request", "requests":
				scopes.Request = true
			case "response", "responses":
				scopes.Response = true
			case "stream", "streams":
				scopes.Stream = true
			case "event", "events":
				scopes.Events = true
			default:
				return Scopes{}, fmt.Errorf("invalid debug scope %q; valid scopes: all, request, response, stream, events", scope)
			}
		}
	}
	return scopes, nil
}

func (s Scopes) TransportEnabled() bool {
	return s.Request || s.Response || s.Stream
}

type TransportOptions struct {
	Inner        transport.ByteStreamTransport
	Writer       io.Writer
	Scopes       Scopes
	Mode         string
	MaxBodyBytes int
}

func NewHTTPTransport(w io.Writer, scopes Scopes) *DebugTransport {
	return NewTransport(TransportOptions{
		Inner:  transport.NewHTTPByteStreamTransport(transport.HTTPTransportConfig{FrameFormat: transport.FrameFormatSSE}),
		Writer: w,
		Scopes: scopes,
		Mode:   ModeHTTPSSE,
	})
}

func NewWebSocketTransport(w io.Writer, scopes Scopes) *DebugTransport {
	return NewTransport(TransportOptions{
		Inner: transport.NewWebSocketByteStreamTransport(transport.WebSocketTransportConfig{
			EnableCompression: true,
			ForceIPv4:         true,
		}),
		Writer: w,
		Scopes: scopes,
		Mode:   ModeWebSocket,
	})
}

func NewTransport(opts TransportOptions) *DebugTransport {
	w := opts.Writer
	if w == nil {
		w = io.Discard
	}
	maxBodyBytes := opts.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultMaxBodyBytes
	}
	return &DebugTransport{
		inner:        opts.Inner,
		w:            w,
		scopes:       opts.Scopes,
		mode:         opts.Mode,
		maxBodyBytes: maxBodyBytes,
	}
}

type DebugTransport struct {
	inner        transport.ByteStreamTransport
	w            io.Writer
	scopes       Scopes
	mode         string
	maxBodyBytes int
	seq          atomic.Uint64
}

func (t *DebugTransport) Open(ctx context.Context, req *transport.Request) (transport.ByteStream, error) {
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
	if t.scopes.Request {
		t.logRequest(id, req, body)
	}
	stream, err := t.inner.Open(ctx, reqForOpen)
	if err != nil {
		if t.scopes.Response {
			t.logResponseError(id, err)
		}
		return nil, err
	}
	if t.scopes.Response {
		t.logResponse(id, stream)
	}
	if t.scopes.Stream {
		stream = &debugByteStream{id: id, inner: stream, w: t.w, mode: t.mode}
	}
	return stream, nil
}

func (t *DebugTransport) logRequest(id uint64, req *transport.Request, body []byte) {
	t.logf(id, "transport: %s\n", t.mode)
	t.logf(id, ">>> request %s %s\n", req.Method, req.URL)
	WriteHeaders(t.w, id, req.Header)
	if len(body) > 0 {
		contentType := req.Header.Get("Content-Type")
		if t.mode == ModeWebSocket {
			t.logBody(id, ">>> websocket frame", contentType, body)
		} else {
			t.logBody(id, ">>> request body", contentType, body)
		}
	}
}

func (t *DebugTransport) logResponse(id uint64, stream transport.ByteStream) {
	t.logf(id, "<<< response headers\n")
	if carrier, ok := stream.(transport.HeaderCarrier); ok {
		WriteHeaders(t.w, id, carrier.Header())
		return
	}
	t.logf(id, "  headers unavailable\n")
}

func (t *DebugTransport) logResponseError(id uint64, err error) {
	t.logf(id, "!!! response error: %v\n", err)
	var apiErr *unified.APIError
	if errors.As(err, &apiErr) && len(apiErr.ProviderRaw) > 0 {
		t.logBody(id, "!!! response error body", "application/json", apiErr.ProviderRaw)
	}
}

func (t *DebugTransport) logBody(id uint64, label string, contentType string, body []byte) {
	truncated := false
	if len(body) > t.maxBodyBytes {
		body = body[:t.maxBodyBytes]
		truncated = true
	}
	text := FormatBody(body, contentType)
	if truncated {
		text += "\n... truncated"
	}
	t.logf(id, "%s:\n%s\n", label, IndentBlock(text))
}

func (t *DebugTransport) logf(id uint64, format string, args ...any) {
	fmt.Fprintf(t.w, "[debug #%d] "+format, append([]any{id}, args...)...)
}

type debugByteStream struct {
	id    uint64
	inner transport.ByteStream
	w     io.Writer
	mode  string
}

func (s *debugByteStream) Recv(ctx context.Context) ([]byte, error) {
	frame, err := s.inner.Recv(ctx)
	if len(frame) > 0 {
		fmt.Fprintf(s.w, "[debug #%d] <<< %s frame:\n%s\n", s.id, s.mode, IndentBlock(FormatFrame(frame)))
	}
	if err != nil && err != io.EOF {
		fmt.Fprintf(s.w, "[debug #%d] !!! stream error: %v\n", s.id, err)
	}
	return frame, err
}

func (s *debugByteStream) Write(ctx context.Context, frame []byte) error {
	writer, ok := s.inner.(transport.ByteStreamWriter)
	if !ok {
		return fmt.Errorf("%s stream does not support writes", s.mode)
	}
	if len(frame) > 0 {
		fmt.Fprintf(s.w, "[debug #%d] >>> %s frame:\n%s\n", s.id, s.mode, IndentBlock(FormatFrame(frame)))
	}
	return writer.Write(ctx, frame)
}

func (s *debugByteStream) Close() error {
	return s.inner.Close()
}

func (s *debugByteStream) Header() http.Header {
	if carrier, ok := s.inner.(transport.HeaderCarrier); ok {
		return carrier.Header()
	}
	return nil
}

func WriteHeaders(w io.Writer, id uint64, headers http.Header) {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		values := headers.Values(key)
		if IsSensitiveKey(key) {
			values = []string{"[redacted]"}
		}
		fmt.Fprintf(w, "[debug #%d]   %s: %s\n", id, key, strings.Join(values, ", "))
	}
}

func WriteEvent(w io.Writer, ev unified.Event) {
	if w == nil {
		return
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		fmt.Fprintf(w, "[debug event] %T: %v\n", ev, ev)
		return
	}
	fmt.Fprintf(w, "[debug event] %T:\n%s\n", ev, IndentBlock(FormatFrame(payload)))
}

func WriteMode(w io.Writer, source string, transportKind unified.TransportKind, continuation unified.ContinuationMode) {
	if w == nil || transportKind == "" {
		return
	}
	if continuation != "" {
		fmt.Fprintf(w, "[debug mode] %s transport=%s internal_continuation=%s\n", source, transportKind, continuation)
		return
	}
	fmt.Fprintf(w, "[debug mode] %s transport=%s\n", source, transportKind)
}

func FormatBody(data []byte, contentType string) string {
	if len(data) == 0 {
		return ""
	}
	contentType = strings.ToLower(contentType)
	if strings.Contains(contentType, "json") || json.Valid(data) {
		var v any
		if err := json.Unmarshal(data, &v); err == nil {
			pretty, err := json.MarshalIndent(RedactJSON(v), "", "  ")
			if err == nil {
				return string(pretty)
			}
		}
	}
	return RedactText(string(data))
}

func FormatFrame(frame []byte) string {
	if len(frame) == 0 {
		return ""
	}
	if json.Valid(frame) {
		var v any
		if err := json.Unmarshal(frame, &v); err == nil {
			pretty, err := json.MarshalIndent(RedactJSON(v), "", "  ")
			if err == nil {
				return string(pretty)
			}
		}
	}
	return RedactText(string(frame))
}

func RedactJSON(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			if IsSensitiveKey(key) {
				out[key] = "[redacted]"
				continue
			}
			out[key] = RedactJSON(value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = RedactJSON(value)
		}
		return out
	default:
		return v
	}
}

func RedactText(text string) string {
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), defaultMaxBodyBytes)
	var out strings.Builder
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		if !first {
			out.WriteByte('\n')
		}
		first = false
		out.WriteString(redactHeaderLikeLine(line))
	}
	if err := scanner.Err(); err != nil {
		return text
	}
	if first {
		return text
	}
	return out.String()
}

func redactHeaderLikeLine(line string) string {
	name, value, ok := strings.Cut(line, ":")
	if !ok || !IsSensitiveKey(name) {
		return line
	}
	if strings.TrimSpace(value) == "" {
		return name + ":"
	}
	return name + ": [redacted]"
}

func IsSensitiveKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return false
	}
	normalized := strings.NewReplacer("-", "", "_", "", " ", "").Replace(k)
	for _, part := range []string{
		"authorization",
		"api-key",
		"apikey",
		"access-token",
		"accesstoken",
		"refresh-token",
		"refreshtoken",
		"id-token",
		"idtoken",
		"user-id",
		"userid",
		"organization",
		"organization-id",
		"organizationid",
		"project",
		"project-id",
		"projectid",
		"client-request-id",
		"clientrequestid",
		"installation",
		"installation-id",
		"installationid",
		"safety-identifier",
		"safetyidentifier",
		"window-id",
		"windowid",
		"cookie",
		"secret",
		"password",
		"session",
		"account",
	} {
		if strings.Contains(k, part) || strings.Contains(normalized, part) {
			return true
		}
	}
	return false
}

func IndentBlock(text string) string {
	if text == "" {
		return "  "
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}
