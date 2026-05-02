package diagnostics

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/codewandler/llmadapter/transport"
)

func TestParseScopes(t *testing.T) {
	scopes, err := ParseScopes([]string{"request,response", "stream"})
	if err != nil {
		t.Fatal(err)
	}
	if !scopes.Enabled || !scopes.Request || !scopes.Response || !scopes.Stream || scopes.Events {
		t.Fatalf("unexpected scopes: %+v", scopes)
	}
	scopes, err = ParseScopes([]string{"all"})
	if err != nil {
		t.Fatal(err)
	}
	if !scopes.Request || !scopes.Response || !scopes.Stream || !scopes.Events {
		t.Fatalf("all did not enable every scope: %+v", scopes)
	}
	if _, err := ParseScopes([]string{"wire"}); err == nil {
		t.Fatalf("expected invalid scope error")
	}
}

func TestDebugTransportRedactsHeadersAndBody(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{
		Header: http.Header{
			"Openai-Project": []string{"proj_secret"},
			"Set-Cookie":     []string{"session=secret"},
			"X-Request":      []string{"req-1"},
		},
		Frames: [][]byte{[]byte(`{"delta":"ok","access_token":"secret"}`)},
	}
	var out bytes.Buffer
	debug := NewTransport(TransportOptions{
		Inner:  fake,
		Writer: &out,
		Scopes: Scopes{Request: true, Response: true, Stream: true},
		Mode:   ModeHTTPSSE,
	})
	stream, err := debug.Open(context.Background(), &transport.Request{
		Method: http.MethodPost,
		URL:    "https://example.test/v1/messages",
		Header: http.Header{
			"Authorization": []string{"Bearer secret"},
			"Content-Type":  []string{"application/json"},
			"X-Trace":       []string{"trace-1"},
		},
		Body: strings.NewReader(`{"message":"hello","api_key":"secret"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(context.Background()); err != io.EOF {
		t.Fatalf("expected eof, got %v", err)
	}
	got := out.String()
	for _, want := range []string{"Authorization: [redacted]", "Openai-Project: [redacted]", "Set-Cookie: [redacted]", `"api_key": "[redacted]"`, `"access_token": "[redacted]"`, "X-Trace: trace-1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("debug output missing %q:\n%s", want, got)
		}
	}
	for _, leaked := range []string{"Bearer secret", "proj_secret", "session=secret", `"api_key": "secret"`, `"access_token": "secret"`} {
		if strings.Contains(got, leaked) {
			t.Fatalf("debug output leaked %q:\n%s", leaked, got)
		}
	}
}

func TestDebugWebSocketTransportLogsModeAndFrames(t *testing.T) {
	fake := &writableFakeTransport{
		header: http.Header{"Openai-Project": []string{"proj_secret"}},
		frames: [][]byte{
			[]byte(`{"type":"response.output_text.delta","delta":"ok","safety_identifier":"user-secret"}`),
		},
	}
	var out bytes.Buffer
	debug := NewTransport(TransportOptions{
		Inner:  fake,
		Writer: &out,
		Scopes: Scopes{Request: true, Response: true, Stream: true},
		Mode:   ModeWebSocket,
	})
	stream, err := debug.Open(context.Background(), &transport.Request{
		Method: http.MethodGet,
		URL:    "wss://example.test/v1/responses",
		Header: http.Header{
			"Authorization":        []string{"Bearer secret"},
			"Content-Type":         []string{"application/json"},
			"X-Client-Request-Id":  []string{"session-secret"},
			"X-Codex-Window-Id":    []string{"window-secret"},
			"X-Codex-Installation": []string{"install-secret"},
		},
		Body: strings.NewReader(`{"type":"response.create","access_token":"secret","client_metadata":{"x-codex-installation-id":"install-secret","x-codex-window-id":"window-secret"}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	writer, ok := stream.(transport.ByteStreamWriter)
	if !ok {
		t.Fatalf("debug websocket stream does not implement writer")
	}
	if err := writer.Write(context.Background(), []byte(`{"type":"response.create","api_key":"secret"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"transport: websocket", ">>> websocket frame", "<<< websocket frame", "Openai-Project: [redacted]", "X-Client-Request-Id: [redacted]", "X-Codex-Window-Id: [redacted]", "X-Codex-Installation: [redacted]", `"access_token": "[redacted]"`, `"api_key": "[redacted]"`, `"x-codex-installation-id": "[redacted]"`, `"x-codex-window-id": "[redacted]"`, `"safety_identifier": "[redacted]"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("debug output missing %q:\n%s", want, got)
		}
	}
	for _, leaked := range []string{"Bearer secret", "proj_secret", "session-secret", "window-secret", "install-secret", "user-secret", `"access_token": "secret"`, `"api_key": "secret"`} {
		if strings.Contains(got, leaked) {
			t.Fatalf("debug output leaked %q:\n%s", leaked, got)
		}
	}
}

type writableFakeTransport struct {
	header http.Header
	frames [][]byte

	mu     sync.Mutex
	writes [][]byte
}

func (t *writableFakeTransport) Open(_ context.Context, req *transport.Request) (transport.ByteStream, error) {
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		t.mu.Lock()
		t.writes = append(t.writes, body)
		t.mu.Unlock()
	}
	return &writableFakeStream{owner: t, header: t.header.Clone(), frames: append([][]byte(nil), t.frames...)}, nil
}

type writableFakeStream struct {
	owner  *writableFakeTransport
	header http.Header
	frames [][]byte
	index  int
}

func (s *writableFakeStream) Header() http.Header {
	return s.header.Clone()
}

func (s *writableFakeStream) Write(_ context.Context, frame []byte) error {
	s.owner.mu.Lock()
	defer s.owner.mu.Unlock()
	s.owner.writes = append(s.owner.writes, append([]byte(nil), frame...))
	return nil
}

func (s *writableFakeStream) Recv(_ context.Context) ([]byte, error) {
	if s.index >= len(s.frames) {
		return nil, io.EOF
	}
	frame := append([]byte(nil), s.frames[s.index]...)
	s.index++
	return frame, nil
}

func (s *writableFakeStream) Close() error {
	return nil
}
