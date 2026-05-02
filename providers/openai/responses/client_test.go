package responses

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

func TestNewClientRequiresAPIKey(t *testing.T) {
	_, err := NewClient()
	if err == nil {
		t.Fatalf("expected missing API key error")
	}
}

func TestClientUsesOpenAIResponsesEndpoint(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-test","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.output_text.delta","response_id":"resp_1","output_index":0,"content_index":0,"delta":"hello"}`),
		[]byte(`data: {"type":"response.output_item.done","item":{"id":"msg_1","type":"message","role":"assistant","status":"completed","phase":"final_answer","content":[{"type":"output_text","text":"hello"}]}}`),
		[]byte(`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-test","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{
		Model: "gpt-test",
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
		Stream: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != "resp_1" || resp.Phase != unified.MessagePhaseFinalAnswer || len(resp.Content) != 1 || resp.Content[0].(unified.TextPart).Text != "hello" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if len(fake.Seen) != 1 || fake.Seen[0].URL != "https://api.openai.com/v1/responses" {
		t.Fatalf("unexpected request: %+v", fake.Seen)
	}
	if fake.Seen[0].Header.Get("Authorization") != "Bearer key" {
		t.Fatalf("missing authorization header: %+v", fake.Seen[0].Header)
	}
}

func TestClientRequestBodyIncludesAssistantMessagePhase(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-test","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{
		Model: "gpt-test",
		Messages: []unified.Message{
			{
				Role:    unified.RoleUser,
				Phase:   unified.MessagePhaseCommentary,
				Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
			},
			{
				Role:    unified.RoleAssistant,
				Phase:   unified.MessagePhaseFinalAnswer,
				Content: []unified.ContentPart{unified.TextPart{Text: "done"}},
			},
		},
		Stream: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	if len(fake.Seen) != 1 {
		t.Fatalf("requests seen = %d, want 1", len(fake.Seen))
	}
	var body struct {
		Input []struct {
			Role  string `json:"role"`
			Phase string `json:"phase"`
		} `json:"input"`
	}
	if err := json.NewDecoder(fake.Seen[0].Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Input) != 2 {
		t.Fatalf("input items = %d, want 2", len(body.Input))
	}
	if body.Input[0].Phase != "" {
		t.Fatalf("user phase = %q, want omitted", body.Input[0].Phase)
	}
	if body.Input[1].Role != "assistant" || body.Input[1].Phase != "final_answer" {
		t.Fatalf("assistant item = %+v, want phase final_answer", body.Input[1])
	}
}

func TestClientDefaultHTTPTransportRetriesTransient503(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Error(w, `{"error":{"message":"temporary"}}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-test","status":"in_progress"}}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"type":"response.output_text.delta","response_id":"resp_1","output_index":0,"content_index":0,"delta":"ok"}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-test","status":"completed"}}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client, err := NewClient(
		WithAPIKey("key"),
		WithBaseURL(server.URL),
		WithWebSocketMode(WebSocketModeDisabled),
	)
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 8
	events, err := client.Request(context.Background(), unified.Request{
		Model:           "gpt-test",
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
		Stream: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if len(resp.Content) != 1 || resp.Content[0].(unified.TextPart).Text != "ok" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestClientKeepsWebSocketDefaultOnHTTPSSE(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_http","model":"gpt-test","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.completed","response":{"id":"resp_http","model":"gpt-test","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	wsFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`{"type":"response.created","response":{"id":"resp_ws","model":"gpt-test","status":"in_progress"}}`),
		[]byte(`{"type":"response.completed","response":{"id":"resp_ws","model":"gpt-test","status":"completed"}}`),
	}}
	client, err := NewClient(
		WithAPIKey("key"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
	)
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{Model: "gpt-test", Stream: true})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != "resp_http" {
		t.Fatalf("response id = %q, want resp_http", resp.ID)
	}
	if len(httpFake.Seen) != 1 || len(wsFake.Seen) != 0 {
		t.Fatalf("http seen=%d ws seen=%d, want http only", len(httpFake.Seen), len(wsFake.Seen))
	}
}

func TestClientUsesWebSocketWhenEnabled(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{}
	wsFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`{"type":"response.created","response":{"id":"resp_ws","model":"gpt-test","status":"in_progress"}}`),
		[]byte(`{"type":"response.output_text.delta","response_id":"resp_ws","output_index":0,"content_index":0,"delta":"ws"}`),
		[]byte(`{"type":"response.completed","response":{"id":"resp_ws","model":"gpt-test","status":"completed"}}`),
	}}
	client, err := NewClient(
		WithAPIKey("key"),
		WithBaseURL("https://example.invalid"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
		WithWebSocketMode(WebSocketModeEnabled),
	)
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{Model: "gpt-test", Stream: true})
	if err != nil {
		t.Fatal(err)
	}
	first := <-events
	metadata, ok := first.(unified.ProviderExecutionEvent)
	if !ok {
		t.Fatalf("first event = %T, want ProviderExecutionEvent", first)
	}
	if metadata.Transport != unified.TransportWebSocket {
		t.Fatalf("metadata = %+v, want websocket transport", metadata)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != "resp_ws" || resp.Content[0].(unified.TextPart).Text != "ws" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if len(httpFake.Seen) != 0 || len(wsFake.Seen) != 1 {
		t.Fatalf("http seen=%d ws seen=%d, want ws only", len(httpFake.Seen), len(wsFake.Seen))
	}
	if got := wsFake.Seen[0].URL; got != "wss://example.invalid/v1/responses" {
		t.Fatalf("ws URL = %q", got)
	}
	if got := wsFake.Seen[0].Header.Get("OpenAI-Beta"); got != webSocketBetaValue {
		t.Fatalf("OpenAI-Beta = %q, want %q", got, webSocketBetaValue)
	}
	var body map[string]any
	if err := json.NewDecoder(wsFake.Seen[0].Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["type"] != "response.create" || body["stream"] != true {
		t.Fatalf("ws body missing response.create/stream=true: %#v", body)
	}
}

func TestClientUsesWebSocketAutoWithCacheKey(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{}
	wsFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`{"type":"response.created","response":{"id":"resp_ws","model":"gpt-test","status":"in_progress"}}`),
		[]byte(`{"type":"response.completed","response":{"id":"resp_ws","model":"gpt-test","status":"completed"}}`),
	}}
	client, err := NewClient(
		WithAPIKey("key"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
		WithWebSocketMode(WebSocketModeAuto),
	)
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{Model: "gpt-test", Stream: true, CacheKey: "sess"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	if len(httpFake.Seen) != 0 || len(wsFake.Seen) != 1 {
		t.Fatalf("http seen=%d ws seen=%d, want ws only", len(httpFake.Seen), len(wsFake.Seen))
	}
	if got := wsFake.Seen[0].Header.Get("x-client-request-id"); got != "sess" {
		t.Fatalf("x-client-request-id = %q, want sess", got)
	}
}

func TestClientFallsBackFromWebSocketAutoOpenFailure(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_http","model":"gpt-test","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.completed","response":{"id":"resp_http","model":"gpt-test","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	wsFake := &transport.FakeByteStreamTransport{OpenErr: context.Canceled}
	client, err := NewClient(
		WithAPIKey("key"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
		WithWebSocketMode(WebSocketModeAuto),
	)
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{Model: "gpt-test", Stream: true, CacheKey: "sess"})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != "resp_http" {
		t.Fatalf("response id = %q, want resp_http", resp.ID)
	}
	if len(httpFake.Seen) != 1 || len(wsFake.Seen) != 1 {
		t.Fatalf("http seen=%d ws seen=%d, want both", len(httpFake.Seen), len(wsFake.Seen))
	}
}

func TestClientDoesNotFallbackWhenWebSocketRequestBodyIsInvalid(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{}
	wsFake := &transport.FakeByteStreamTransport{}
	client, err := NewClient(
		WithAPIKey("key"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
		WithWebSocketMode(WebSocketModeAuto),
		WithBodyMutator(func(unified.Request, []byte) ([]byte, []unified.WarningEvent, error) {
			return []byte(`{`), nil, nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Request(context.Background(), unified.Request{Model: "gpt-test", Stream: true, CacheKey: "sess"})
	if err == nil {
		t.Fatal("expected invalid websocket request body error")
	}
	if !errors.Is(err, errWebSocketRequestBody) {
		t.Fatalf("error = %v, want websocket request body error", err)
	}
	if len(httpFake.Seen) != 0 || len(wsFake.Seen) != 0 {
		t.Fatalf("http seen=%d ws seen=%d, want no fallback/open", len(httpFake.Seen), len(wsFake.Seen))
	}
}

func TestClientInvalidatesWebSocketSessionAfterReadError(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{}
	wsFake := &reusableWebSocketTransport{turnFrames: [][][]byte{
		{
			[]byte(`{"type":"response.created","response":{"id":"resp_1","model":"gpt-test","status":"in_progress"}}`),
		},
		{
			[]byte(`{"type":"response.created","response":{"id":"resp_2","model":"gpt-test","status":"in_progress"}}`),
			[]byte(`{"type":"response.completed","response":{"id":"resp_2","model":"gpt-test","status":"completed"}}`),
		},
	}, turnErrs: []error{io.ErrUnexpectedEOF, nil}}
	client, err := NewClient(
		WithAPIKey("key"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
		WithWebSocketMode(WebSocketModeAuto),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "gpt-test", Stream: true, CacheKey: "sess"}
	events, err := client.Request(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err == nil {
		t.Fatal("expected websocket read error")
	}
	events, err = client.Request(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != "resp_2" {
		t.Fatalf("response id = %q, want resp_2", resp.ID)
	}
	if len(wsFake.seen) != 2 {
		t.Fatalf("websocket opens = %d, want fresh open after read error", len(wsFake.seen))
	}
	if len(wsFake.bodies) != 2 {
		t.Fatalf("websocket request bodies = %d, want 2", len(wsFake.bodies))
	}
	if len(httpFake.Seen) != 0 {
		t.Fatalf("http fallback opens = %d, want 0", len(httpFake.Seen))
	}
}

func TestClientDecodesProviderExecutionMetadata(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"llmadapter.provider_execution","transport":"http_sse","internal_continuation":"previous_response_id"}`),
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-test","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-test","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{Model: "gpt-test", Stream: true})
	if err != nil {
		t.Fatal(err)
	}
	first := <-events
	metadata, ok := first.(unified.ProviderExecutionEvent)
	if !ok {
		t.Fatalf("first event = %T, want ProviderExecutionEvent", first)
	}
	if metadata.Transport != unified.TransportHTTPSSE || metadata.InternalContinuation != unified.ContinuationPreviousResponseID {
		t.Fatalf("metadata = %+v", metadata)
	}
}

func TestClientAssemblesFragmentedStreamJSONEvents(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {`),
		[]byte(`data: "type":"response.created","response":{"id":"resp_1","model":"gpt-test","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.output_text.delta","response_id":"resp_1","output_index":0,"content_index":0,"delta":"hel`),
		[]byte(`data: lo"}`),
		[]byte(`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-test","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{Model: "gpt-test", Stream: true})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != "resp_1" || len(resp.Content) != 1 || resp.Content[0].(unified.TextPart).Text != "hello" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestClientUsesOpenAIWarningSource(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-test","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-test","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{
		Model: "gpt-test",
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.ReasoningPart{Text: "unsupported"}},
		}},
		Stream: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Warnings) != 1 || resp.Warnings[0].Source != "openai.responses" {
		t.Fatalf("warnings = %+v", resp.Warnings)
	}
}

type reusableWebSocketTransport struct {
	mu         sync.Mutex
	next       int
	turnFrames [][][]byte
	turnErrs   []error
	seen       []*transport.Request
	bodies     [][]byte
	stream     *reusableWebSocketStream
}

func (t *reusableWebSocketTransport) Open(ctx context.Context, req *transport.Request) (transport.ByteStream, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.seen = append(t.seen, req)
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	t.bodies = append(t.bodies, body)
	frames, streamErr := t.nextTurnLocked()
	t.stream = &reusableWebSocketStream{owner: t, frames: frames, err: streamErr}
	return t.stream, nil
}

func (t *reusableWebSocketTransport) nextTurnLocked() ([][]byte, error) {
	index := t.next
	t.next++
	var frames [][]byte
	if index < len(t.turnFrames) {
		frames = cloneFrames(t.turnFrames[index])
	}
	var err error
	if index < len(t.turnErrs) {
		err = t.turnErrs[index]
	}
	return frames, err
}

type reusableWebSocketStream struct {
	owner  *reusableWebSocketTransport
	frames [][]byte
	err    error
	index  int
}

func (s *reusableWebSocketStream) Header() http.Header {
	return http.Header{}
}

func (s *reusableWebSocketStream) Write(ctx context.Context, frame []byte) error {
	s.owner.mu.Lock()
	defer s.owner.mu.Unlock()
	s.owner.bodies = append(s.owner.bodies, append([]byte(nil), frame...))
	frames, streamErr := s.owner.nextTurnLocked()
	s.frames = frames
	s.err = streamErr
	s.index = 0
	return nil
}

func (s *reusableWebSocketStream) Recv(ctx context.Context) ([]byte, error) {
	if s.index >= len(s.frames) {
		if s.err != nil {
			return nil, s.err
		}
		return nil, io.EOF
	}
	frame := append([]byte(nil), s.frames[s.index]...)
	s.index++
	return frame, nil
}

func (s *reusableWebSocketStream) Close() error {
	return nil
}

func cloneFrames(frames [][]byte) [][]byte {
	out := make([][]byte, len(frames))
	for i := range frames {
		out[i] = append([]byte(nil), frames[i]...)
	}
	return out
}
