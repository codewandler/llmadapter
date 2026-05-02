package main

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

func TestParseInferDebugScopes(t *testing.T) {
	scopes, err := parseInferDebugScopes([]string{"request,response", "stream"})
	if err != nil {
		t.Fatal(err)
	}
	if !scopes.enabled || !scopes.request || !scopes.response || !scopes.stream || scopes.events {
		t.Fatalf("unexpected scopes: %+v", scopes)
	}
	scopes, err = parseInferDebugScopes([]string{"all"})
	if err != nil {
		t.Fatal(err)
	}
	if !scopes.request || !scopes.response || !scopes.stream || !scopes.events {
		t.Fatalf("all did not enable every scope: %+v", scopes)
	}
	if _, err := parseInferDebugScopes([]string{"wire"}); err == nil {
		t.Fatalf("expected invalid scope error")
	}
}

func TestInferDebugWebSocketTransportLogsModeAndFrames(t *testing.T) {
	fake := &writableFakeTransport{
		header: http.Header{"Openai-Project": []string{"proj_secret"}},
		frames: [][]byte{
			[]byte(`{"type":"response.output_text.delta","delta":"ok"}`),
		},
	}
	var out bytes.Buffer
	debug := newInferDebugWebSocketTransport(&out, inferDebugScopes{request: true, response: true, stream: true})
	debug.inner = fake
	stream, err := debug.Open(context.Background(), &transport.Request{
		Method: http.MethodGet,
		URL:    "wss://example.test/v1/responses",
		Header: http.Header{
			"Authorization": []string{"Bearer secret"},
			"Content-Type":  []string{"application/json"},
		},
		Body: strings.NewReader(`{"type":"response.create","access_token":"secret"}`),
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
	for _, want := range []string{"transport: websocket", ">>> websocket frame", "<<< websocket frame", "Openai-Project: [redacted]", `"access_token": "[redacted]"`, `"api_key": "[redacted]"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("debug output missing %q:\n%s", want, got)
		}
	}
	for _, leaked := range []string{"Bearer secret", "proj_secret", `"access_token": "secret"`, `"api_key": "secret"`} {
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

func (t *writableFakeTransport) Open(ctx context.Context, req *transport.Request) (transport.ByteStream, error) {
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

func (s *writableFakeStream) Write(ctx context.Context, frame []byte) error {
	s.owner.mu.Lock()
	defer s.owner.mu.Unlock()
	s.owner.writes = append(s.owner.writes, append([]byte(nil), frame...))
	return nil
}

func (s *writableFakeStream) Recv(ctx context.Context) ([]byte, error) {
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

func TestNormalizeInferArgsAcceptsSeparatedDebugScopes(t *testing.T) {
	params := inferParams{debugScopes: []string{"all"}}
	args, err := normalizeInferArgs(&params, []string{"request,response", "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 1 || args[0] != "hello" {
		t.Fatalf("unexpected args: %+v", args)
	}
	if len(params.debugScopes) != 1 || params.debugScopes[0] != "request,response" {
		t.Fatalf("debug scopes not rewritten: %+v", params.debugScopes)
	}
}

func TestInferDebugTransportRedactsHeadersAndBody(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{
		Header: http.Header{
			"Openai-Project": []string{"proj_secret"},
			"Set-Cookie":     []string{"session=secret"},
			"X-Request":      []string{"req-1"},
		},
		Frames: [][]byte{[]byte(`{"delta":"ok","access_token":"secret"}`)},
	}
	var out bytes.Buffer
	debug := newInferDebugTransport(&out, inferDebugScopes{request: true, response: true, stream: true})
	debug.inner = fake
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
