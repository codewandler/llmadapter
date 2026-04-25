package codex

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

func TestNewClientRequiresAuth(t *testing.T) {
	t.Setenv(EnvAuthPath, t.TempDir()+"/missing.json")
	_, err := NewClient()
	if err == nil || !strings.Contains(err.Error(), "load auth") {
		t.Fatalf("expected auth error, got %v", err)
	}
}

func TestClientMutatesCodexRequest(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-5.4","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.content_part.added","response_id":"resp_1","output_index":0,"content_index":0,"part":{"type":"output_text","text":""}}`),
		[]byte(`data: {"type":"response.output_text.delta","response_id":"resp_1","output_index":0,"content_index":0,"delta":"ok"}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"gpt-5.4","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithInstallationID("install-1"),
		WithTransport(fake),
	)
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 8
	req := unified.Request{
		Model:           "codex",
		MaxOutputTokens: &maxTokens,
		CachePolicy:     unified.CachePolicyOn,
		CacheKey:        "session-1",
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
		Stream: true,
	}
	events, err := client.Request(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}

	if len(fake.Seen) != 1 {
		t.Fatalf("seen requests = %d", len(fake.Seen))
	}
	seen := fake.Seen[0]
	if seen.URL != "https://example.invalid/backend-api/codex/responses" {
		t.Fatalf("URL = %q", seen.URL)
	}
	if got := seen.Header.Get("Authorization"); got != "Bearer token" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := seen.Header.Get(HeaderCodexInstallationID); got != "install-1" {
		t.Fatalf("%s = %q", HeaderCodexInstallationID, got)
	}
	if got := seen.Header.Get(HeaderSessionID); got != "session-1" {
		t.Fatalf("%s = %q", HeaderSessionID, got)
	}
	if got := seen.Header.Get(HeaderCodexWindowID); got != "session-1:0" {
		t.Fatalf("%s = %q", HeaderCodexWindowID, got)
	}

	var body map[string]any
	if err := json.NewDecoder(seen.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["model"] != DefaultModel {
		t.Fatalf("model = %v", body["model"])
	}
	if body["instructions"] != defaultInstructions {
		t.Fatalf("instructions = %v", body["instructions"])
	}
	if body["store"] != false {
		t.Fatalf("store = %v", body["store"])
	}
	if _, ok := body["max_output_tokens"]; ok {
		t.Fatalf("max_output_tokens was not removed: %#v", body)
	}
}

func TestClientAppliesCodexExtensions(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-5.4","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"gpt-5.4","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithInstallationID("install-1"),
		WithTransport(fake),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "codex", Stream: true}
	if err := unified.SetCodexExtensions(&req.Extensions, unified.CodexExtensions{
		SessionID:            "sess",
		WindowID:             "sess:3",
		TurnState:            "sticky",
		TurnMetadata:         `{"turn":1}`,
		ParentThreadID:       "thread",
		Subagent:             true,
		MemgenRequest:        true,
		IncludeTimingMetrics: true,
	}); err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}

	if len(fake.Seen) != 1 {
		t.Fatalf("seen requests = %d", len(fake.Seen))
	}
	header := fake.Seen[0].Header
	checkHeader := func(key, want string) {
		t.Helper()
		if got := header.Get(key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	checkHeader(HeaderSessionID, "sess")
	checkHeader(HeaderCodexWindowID, "sess:3")
	checkHeader(HeaderCodexTurnState, "sticky")
	checkHeader(HeaderCodexTurnMetadata, `{"turn":1}`)
	checkHeader(HeaderCodexParentThreadID, "thread")
	checkHeader(HeaderOpenAISubagent, "true")
	checkHeader(HeaderOpenAIMemgenRequest, "true")
	checkHeader(HeaderTimingMetrics, "true")
}

func TestClientRejectsInvalidCodexExtensions(t *testing.T) {
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(&transport.FakeByteStreamTransport{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "codex", Stream: true}
	if err := req.Extensions.Set(unified.ExtCodexSessionID, " "); err != nil {
		t.Fatal(err)
	}
	_, err = client.Request(context.Background(), req)
	if err == nil {
		t.Fatal("expected invalid codex extension error")
	}
	if !strings.Contains(err.Error(), "invalid extensions") {
		t.Fatalf("unexpected error: %v", err)
	}
}
