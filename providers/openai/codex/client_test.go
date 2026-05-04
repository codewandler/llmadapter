package codex

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

func TestDefaultHTTPTransportRetriesTransient503(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Error(w, `{"error":{"message":"temporary"}}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-5.4","status":"in_progress"}}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"type":"response.output_text.delta","response_id":"resp_1","output_index":0,"content_index":0,"delta":"ok"}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.4","status":"completed"}}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL(server.URL),
		WithPath("/v1/responses"),
		WithWebSocketEnabled(false),
	)
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 8
	events, err := client.Request(context.Background(), unified.Request{
		Model:           "codex",
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
		WithWebSocketEnabled(false),
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

	if len(fake.Seen) < 1 {
		t.Fatalf("seen requests = %d", len(fake.Seen))
	}
	seen := fake.Seen[len(fake.Seen)-1]
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
	if got := seen.Header.Get(HeaderOriginator); got != CodexCLIOriginator {
		t.Fatalf("%s = %q", HeaderOriginator, got)
	}
	if got := seen.Header.Get(HeaderVersion); got != CodexCLIVersion {
		t.Fatalf("%s = %q", HeaderVersion, got)
	}
	if got := seen.Header.Get("User-Agent"); !strings.HasPrefix(got, CodexCLIOriginator+"/"+CodexCLIVersion+" ") {
		t.Fatalf("User-Agent = %q", got)
	}

	var body map[string]any
	if err := json.NewDecoder(seen.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["model"] != "codex" {
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

func TestClientDropsUnsupportedPreviousResponseID(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-5.4","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"gpt-5.4","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(fake),
		WithWebSocketEnabled(false),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "codex", Stream: true}
	if err := req.Extensions.Set(unified.ExtOpenAIPreviousResponseID, "resp_prev"); err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Warnings) != 1 || resp.Warnings[0].Source != "codex.responses" || resp.Warnings[0].Code != "unsupported_field_dropped" {
		t.Fatalf("warnings = %+v", resp.Warnings)
	}

	var body map[string]any
	if err := json.NewDecoder(fake.Seen[len(fake.Seen)-1].Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["previous_response_id"]; ok {
		t.Fatalf("previous_response_id was not removed: %#v", body)
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
		WithWebSocketEnabled(false),
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

	if len(fake.Seen) < 1 {
		t.Fatalf("seen requests = %d", len(fake.Seen))
	}
	header := fake.Seen[len(fake.Seen)-1].Header
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

func TestClientCapturesHTTPSSETurnStateAndQuotaHeaders(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{
		Header: http.Header{
			"X-Codex-Turn-State":               []string{"sticky-http"},
			"X-Codex-Primary-Used-Percent":     []string{"12.5"},
			"X-Codex-Primary-Window-Minutes":   []string{"15"},
			"X-Codex-Secondary-Used-Percent":   []string{"40"},
			"X-Codex-Secondary-Window-Minutes": []string{"60"},
		},
		Frames: [][]byte{
			[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-5.4","status":"in_progress"}}`),
			[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"gpt-5.4","status":"completed"}}`),
			[]byte(`data: [DONE]`),
		},
	}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(fake),
		WithWebSocketEnabled(false),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "codex", Stream: true}
	if err := unified.SetCodexExtensions(&req.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
		SessionID:       "sess",
	}); err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Quotas) != 1 {
		t.Fatalf("quotas = %+v, want one quota event", resp.Quotas)
	}
	limit := resp.Quotas[0].Limits[0]
	if limit.ID != "codex" || len(limit.Windows) != 2 {
		t.Fatalf("quota limit = %+v", limit)
	}
	if limit.Windows[0].Name != "primary" || limit.Windows[0].UsedPercent != 12.5 {
		t.Fatalf("primary quota = %+v", limit.Windows[0])
	}
	if limit.Windows[1].Name != "secondary" || limit.Windows[1].UsedPercent != 40 {
		t.Fatalf("secondary quota = %+v", limit.Windows[1])
	}

	events, err = client.Request(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	if len(fake.Seen) != 2 {
		t.Fatalf("seen requests = %d, want 2", len(fake.Seen))
	}
	if got := fake.Seen[1].Header.Get(HeaderCodexTurnState); got != "sticky-http" {
		t.Fatalf("%s on second request = %q, want sticky-http", HeaderCodexTurnState, got)
	}
}

func TestClientMapsWebSocketRateLimitEventToQuota(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{}
	wsFake := &reusableWebSocketTransport{
		turnFrames: [][][]byte{{
			[]byte(`{"type":"codex.rate_limits","plan_type":"plus","rate_limits":{"primary":{"used_percent":55,"window_minutes":15},"secondary":{"used_percent":20,"window_minutes":60}},"credits":{"has_credits":true,"unlimited":false,"balance":"10"}}`),
			[]byte(`{"type":"response.done","response":{"id":"resp_ws","model":"gpt-5.4","status":"completed"}}`),
		}},
	}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "codex", Stream: true}
	if err := unified.SetCodexExtensions(&req.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
		SessionID:       "sess",
	}); err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Quotas) != 1 {
		t.Fatalf("quotas = %+v, want one quota event", resp.Quotas)
	}
	quota := resp.Quotas[0]
	if quota.Provider != ProviderName || quota.Plan != "plus" {
		t.Fatalf("quota metadata = %+v", quota)
	}
	limit := quota.Limits[0]
	if len(limit.Windows) != 2 || limit.Windows[0].UsedPercent != 55 || limit.Windows[1].UsedPercent != 20 {
		t.Fatalf("quota windows = %+v", limit.Windows)
	}
	if limit.Credits == nil || limit.Credits.HasCredits == nil || !*limit.Credits.HasCredits || limit.Credits.Balance != "10" {
		t.Fatalf("quota credits = %+v", limit.Credits)
	}
	if len(httpFake.Seen) != 0 {
		t.Fatalf("http fallback requests = %d, want 0", len(httpFake.Seen))
	}
}

func TestClientRejectsInvalidCodexExtensions(t *testing.T) {
	tests := []struct {
		name string
		key  string
		val  any
	}{
		{name: "unsafe session header", key: unified.ExtCodexSessionID, val: " "},
		{name: "invalid turn metadata", key: unified.ExtCodexTurnMetadata, val: "not-json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(
				WithAccessToken("token"),
				WithBaseURL("https://example.invalid/backend-api"),
				WithTransport(&transport.FakeByteStreamTransport{}),
				WithWebSocketEnabled(false),
			)
			if err != nil {
				t.Fatal(err)
			}
			req := unified.Request{Model: "codex", Stream: true}
			if err := req.Extensions.Set(tt.key, tt.val); err != nil {
				t.Fatal(err)
			}
			_, err = client.Request(context.Background(), req)
			if err == nil {
				t.Fatal("expected invalid codex extension error")
			}
			if !strings.Contains(err.Error(), "invalid extensions") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestClientUsesWebSocketForSessionInteraction(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{}
	wsFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`{"type":"response.created","response":{"id":"resp_ws","model":"gpt-5.4","status":"in_progress"}}`),
		[]byte(`{"type":"response.output_text.delta","response_id":"resp_ws","output_index":0,"content_index":0,"delta":"ws ok"}`),
		[]byte(`{"type":"response.done","response":{"id":"resp_ws","model":"gpt-5.4","status":"completed"}}`),
	}}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "codex", Stream: true}
	if err := unified.SetCodexExtensions(&req.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
		SessionID:       "sess",
	}); err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if got := responseText(resp); got != "ws ok" {
		t.Fatalf("text = %q, want ws ok", got)
	}
	if len(wsFake.Seen) != 1 || len(httpFake.Seen) != 0 {
		t.Fatalf("ws seen=%d http seen=%d, want ws only", len(wsFake.Seen), len(httpFake.Seen))
	}
	if got := wsFake.Seen[0].URL; got != "wss://example.invalid/backend-api/codex/responses" {
		t.Fatalf("ws URL = %q", got)
	}
	if got := wsFake.Seen[0].Header.Get(HeaderOpenAIBeta); got != WebSocketBetaValue {
		t.Fatalf("ws beta header = %q, want %q", got, WebSocketBetaValue)
	}
	if got := wsFake.Seen[0].Header.Get("Content-Type"); got != "" {
		t.Fatalf("ws Content-Type = %q, want empty", got)
	}
	if got := wsFake.Seen[0].Header.Get("User-Agent"); got != "" {
		t.Fatalf("ws User-Agent = %q, want empty", got)
	}
	if got := wsFake.Seen[0].Header.Get(HeaderCodexInstallationID); got != "" {
		t.Fatalf("ws %s = %q, want empty", HeaderCodexInstallationID, got)
	}
	if got := wsFake.Seen[0].Header.Get("x-client-request-id"); got != "sess" {
		t.Fatalf("ws x-client-request-id = %q, want sess", got)
	}
	var body map[string]any
	if err := json.NewDecoder(wsFake.Seen[0].Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["type"] != "response.create" {
		t.Fatalf("ws body type = %v", body["type"])
	}
	if body["model"] != "codex" {
		t.Fatalf("ws body model = %v", body["model"])
	}
	if body["tool_choice"] != "auto" || body["parallel_tool_calls"] != true {
		t.Fatalf("ws defaults missing: %#v", body)
	}
	if _, ok := body["tools"].([]any); !ok {
		t.Fatalf("ws tools = %#v", body["tools"])
	}
	if _, ok := body["reasoning"]; !ok {
		t.Fatalf("ws reasoning field missing: %#v", body)
	}
	metadata, ok := body["client_metadata"].(map[string]any)
	if !ok {
		t.Fatalf("ws client_metadata = %#v", body["client_metadata"])
	}
	if metadata[HeaderCodexWindowID] != "sess:0" {
		t.Fatalf("ws window metadata = %#v", metadata)
	}
}

func TestClientAssemblesFragmentedWebSocketJSONEvents(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{}
	wsFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`{`),
		[]byte(`"type":"response.created","response":{"id":"resp_ws","model":"gpt-5.4","status":"in_progress"}}`),
		[]byte(`{"type":"response.output_text.delta","response_id":"resp_ws","output_index":0,"content_index":0,"delta":"ws `),
		[]byte(`ok"}`),
		[]byte(`{"type":"response.done","response":{"id":"resp_ws","model":"gpt-5.4","status":"completed"}}`),
	}}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "codex", Stream: true}
	if err := unified.SetCodexExtensions(&req.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
		SessionID:       "sess",
	}); err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if got := responseText(resp); got != "ws ok" {
		t.Fatalf("text = %q, want ws ok", got)
	}
	if len(wsFake.Seen) != 1 || len(httpFake.Seen) != 0 {
		t.Fatalf("ws seen=%d http seen=%d, want ws only", len(wsFake.Seen), len(httpFake.Seen))
	}
}

func TestClientEmitsWebSocketExecutionMetadataBeforeDoneOnlyResponse(t *testing.T) {
	wsFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`{"type":"response.done","response":{"id":"resp_ws","model":"gpt-5.4","status":"completed"}}`),
	}}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(&transport.FakeByteStreamTransport{}),
		WithWebSocketTransport(wsFake),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "codex", Stream: true}
	if err := unified.SetCodexExtensions(&req.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
		SessionID:       "sess",
	}); err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	first := <-events
	metadata, ok := first.(unified.ProviderExecutionEvent)
	if !ok {
		t.Fatalf("first event = %T, want ProviderExecutionEvent", first)
	}
	if metadata.Transport != unified.TransportWebSocket || metadata.InternalContinuation != unified.ContinuationReplay {
		t.Fatalf("metadata = %+v", metadata)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}
}

func TestClientUsesInternalPreviousResponseIDForAppendOnlyWebSocketSession(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{}
	wsFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`{"type":"response.created","response":{"id":"resp_1","model":"gpt-5.4","status":"in_progress"}}`),
		[]byte(`{"type":"response.done","response":{"id":"resp_1","model":"gpt-5.4","status":"completed"}}`),
	}}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
	)
	if err != nil {
		t.Fatal(err)
	}
	first := unified.Request{
		Model:  "codex",
		Stream: true,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "one"}},
		}},
	}
	if err := unified.SetCodexExtensions(&first.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
		SessionID:       "sess",
		BranchID:        "main",
	}); err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}

	wsFake.Frames = [][]byte{
		[]byte(`{"type":"response.created","response":{"id":"resp_2","model":"gpt-5.4","status":"in_progress"}}`),
		[]byte(`{"type":"response.done","response":{"id":"resp_2","model":"gpt-5.4","status":"completed"}}`),
	}
	second := first
	second.Messages = append(second.Messages, unified.Message{
		Role:    unified.RoleAssistant,
		Content: []unified.ContentPart{unified.TextPart{Text: "two"}},
	})
	events, err = client.Request(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	if len(wsFake.Seen) != 2 {
		t.Fatalf("ws seen = %d, want 2", len(wsFake.Seen))
	}
	var firstBody, secondBody map[string]any
	if err := json.NewDecoder(wsFake.Seen[0].Body).Decode(&firstBody); err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(wsFake.Seen[1].Body).Decode(&secondBody); err != nil {
		t.Fatal(err)
	}
	if _, ok := firstBody["previous_response_id"]; ok {
		t.Fatalf("first body should not include previous_response_id: %#v", firstBody)
	}
	if secondBody["previous_response_id"] != "resp_1" {
		t.Fatalf("second previous_response_id = %v, want resp_1; body=%#v", secondBody["previous_response_id"], secondBody)
	}
}

func TestClientReusesWebSocketConnectionForInternalPreviousResponseID(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{}
	wsFake := &reusableWebSocketTransport{turnFrames: [][][]byte{
		{
			[]byte(`{"type":"response.created","response":{"id":"resp_1","model":"gpt-5.4","status":"in_progress"}}`),
			[]byte(`{"type":"response.done","response":{"id":"resp_1","model":"gpt-5.4","status":"completed"}}`),
		},
		{
			[]byte(`{"type":"response.created","response":{"id":"resp_2","model":"gpt-5.4","status":"in_progress"}}`),
			[]byte(`{"type":"response.done","response":{"id":"resp_2","model":"gpt-5.4","status":"completed"}}`),
		},
	}}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
	)
	if err != nil {
		t.Fatal(err)
	}
	first := unified.Request{
		Model:  "codex",
		Stream: true,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "one"}},
		}},
	}
	if err := unified.SetCodexExtensions(&first.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
		SessionID:       "sess",
		BranchID:        "main",
	}); err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}

	second := first
	second.Messages = append(second.Messages,
		unified.Message{Role: unified.RoleAssistant, Content: []unified.ContentPart{unified.TextPart{Text: "answer"}}},
		unified.Message{Role: unified.RoleUser, Content: []unified.ContentPart{unified.TextPart{Text: "two"}}},
	)
	events, err = client.Request(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	if len(wsFake.seen) != 1 {
		t.Fatalf("websocket opens = %d, want 1", len(wsFake.seen))
	}
	if len(httpFake.Seen) != 0 {
		t.Fatalf("http fallback opens = %d, want 0", len(httpFake.Seen))
	}
	if len(wsFake.bodies) != 2 {
		t.Fatalf("websocket writes = %d, want 2", len(wsFake.bodies))
	}
	var firstBody, secondBody map[string]any
	if err := json.Unmarshal(wsFake.bodies[0], &firstBody); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(wsFake.bodies[1], &secondBody); err != nil {
		t.Fatal(err)
	}
	if _, ok := firstBody["previous_response_id"]; ok {
		t.Fatalf("first body should not include previous_response_id: %#v", firstBody)
	}
	if secondBody["previous_response_id"] != "resp_1" {
		t.Fatalf("second previous_response_id = %v, want resp_1; body=%#v", secondBody["previous_response_id"], secondBody)
	}
}

func TestClientInvalidatesWebSocketContinuationAfterMidStreamDisconnect(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{}
	wsFake := &reusableWebSocketTransport{
		turnFrames: [][][]byte{
			{
				[]byte(`{"type":"response.created","response":{"id":"resp_1","model":"gpt-5.4","status":"in_progress"}}`),
				[]byte(`{"type":"response.done","response":{"id":"resp_1","model":"gpt-5.4","status":"completed"}}`),
			},
			{
				[]byte(`{"type":"response.output_text.delta","response_id":"resp_2","output_index":0,"content_index":0,"delta":"partial"}`),
			},
			{
				[]byte(`{"type":"response.created","response":{"id":"resp_3","model":"gpt-5.4","status":"in_progress"}}`),
				[]byte(`{"type":"response.done","response":{"id":"resp_3","model":"gpt-5.4","status":"completed"}}`),
			},
		},
		turnErrs: []error{nil, io.ErrUnexpectedEOF, nil},
	}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
	)
	if err != nil {
		t.Fatal(err)
	}
	first := unified.Request{
		Model:  "codex",
		Stream: true,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "one"}},
		}},
	}
	if err := unified.SetCodexExtensions(&first.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
		SessionID:       "sess",
		BranchID:        "main",
	}); err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}

	second := first
	second.Messages = append(second.Messages,
		unified.Message{Role: unified.RoleAssistant, Content: []unified.ContentPart{unified.TextPart{Text: "answer"}}},
		unified.Message{Role: unified.RoleUser, Content: []unified.ContentPart{unified.TextPart{Text: "two"}}},
	)
	events, err = client.Request(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err == nil {
		t.Fatal("expected mid-stream websocket failure")
	}

	third := second
	third.Messages = append(third.Messages,
		unified.Message{Role: unified.RoleAssistant, Content: []unified.ContentPart{unified.TextPart{Text: "failed partial should not commit"}}},
		unified.Message{Role: unified.RoleUser, Content: []unified.ContentPart{unified.TextPart{Text: "three"}}},
	)
	events, err = client.Request(context.Background(), third)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	if len(wsFake.seen) != 2 {
		t.Fatalf("websocket opens = %d, want 2 after failed connection is discarded", len(wsFake.seen))
	}
	if len(wsFake.bodies) != 3 {
		t.Fatalf("websocket request bodies = %d, want 3", len(wsFake.bodies))
	}
	var secondBody, thirdBody map[string]any
	if err := json.Unmarshal(wsFake.bodies[1], &secondBody); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(wsFake.bodies[2], &thirdBody); err != nil {
		t.Fatal(err)
	}
	if secondBody["previous_response_id"] != "resp_1" {
		t.Fatalf("second previous_response_id = %v, want resp_1", secondBody["previous_response_id"])
	}
	if _, ok := thirdBody["previous_response_id"]; ok {
		t.Fatalf("third body should replay after failed websocket session: %#v", thirdBody)
	}
}

func TestClientRetriesFreshWebSocketAfterPreOutputAbnormalClose(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{}
	wsFake := &reusableWebSocketTransport{
		turnFrames: [][][]byte{
			{
				[]byte(`{"type":"response.created","response":{"id":"resp_1","model":"gpt-5.4","status":"in_progress"}}`),
				[]byte(`{"type":"response.done","response":{"id":"resp_1","model":"gpt-5.4","status":"completed"}}`),
			},
			nil,
			{
				[]byte(`{"type":"response.created","response":{"id":"resp_2","model":"gpt-5.4","status":"in_progress"}}`),
				[]byte(`{"type":"response.output_text.delta","response_id":"resp_2","output_index":0,"content_index":0,"delta":"fresh ws ok"}`),
				[]byte(`{"type":"response.done","response":{"id":"resp_2","model":"gpt-5.4","status":"completed"}}`),
			},
		},
		turnErrs: []error{nil, &transport.WebSocketCloseError{Code: 1006, Text: "unexpected EOF"}, nil},
	}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
	)
	if err != nil {
		t.Fatal(err)
	}
	first := unified.Request{
		Model:  "codex",
		Stream: true,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "one"}},
		}},
	}
	if err := unified.SetCodexExtensions(&first.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
		SessionID:       "sess",
		BranchID:        "main",
	}); err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}

	second := first
	second.Messages = append(second.Messages,
		unified.Message{Role: unified.RoleAssistant, Content: []unified.ContentPart{unified.TextPart{Text: "answer"}}},
		unified.Message{Role: unified.RoleUser, Content: []unified.ContentPart{unified.TextPart{Text: "two"}}},
	)
	events, err = client.Request(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if got := responseText(resp); got != "fresh ws ok" {
		t.Fatalf("text = %q, want fresh ws ok", got)
	}
	if len(wsFake.seen) != 2 {
		t.Fatalf("websocket opens = %d, want initial plus fresh retry", len(wsFake.seen))
	}
	if len(httpFake.Seen) != 0 {
		t.Fatalf("http fallback opens = %d, want 0", len(httpFake.Seen))
	}
	if len(wsFake.bodies) != 3 {
		t.Fatalf("websocket request bodies = %d, want initial, failed reuse, fresh retry", len(wsFake.bodies))
	}
	var reusedBody, freshBody map[string]any
	if err := json.Unmarshal(wsFake.bodies[1], &reusedBody); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(wsFake.bodies[2], &freshBody); err != nil {
		t.Fatal(err)
	}
	if reusedBody["previous_response_id"] != "resp_1" {
		t.Fatalf("reused previous_response_id = %v, want resp_1", reusedBody["previous_response_id"])
	}
	if _, ok := freshBody["previous_response_id"]; ok {
		t.Fatalf("fresh websocket retry should replay without previous_response_id: %#v", freshBody)
	}
}

func TestClientRetriesFreshWebSocketAfterPreOutputWriteFailure(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{}
	wsFake := &reusableWebSocketTransport{
		turnFrames: [][][]byte{
			{
				[]byte(`{"type":"response.created","response":{"id":"resp_1","model":"gpt-5.4","status":"in_progress"}}`),
				[]byte(`{"type":"response.done","response":{"id":"resp_1","model":"gpt-5.4","status":"completed"}}`),
			},
			{
				[]byte(`{"type":"response.created","response":{"id":"resp_2","model":"gpt-5.4","status":"in_progress"}}`),
				[]byte(`{"type":"response.output_text.delta","response_id":"resp_2","output_index":0,"content_index":0,"delta":"write retry ok"}`),
				[]byte(`{"type":"response.done","response":{"id":"resp_2","model":"gpt-5.4","status":"completed"}}`),
			},
		},
		writeErrs: []error{nil, &transport.WebSocketCloseError{Code: 1006, Text: "unexpected EOF"}},
	}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
	)
	if err != nil {
		t.Fatal(err)
	}
	first := unified.Request{
		Model:  "codex",
		Stream: true,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "one"}},
		}},
	}
	if err := unified.SetCodexExtensions(&first.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
		SessionID:       "sess",
		BranchID:        "main",
	}); err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}

	second := first
	second.Messages = append(second.Messages,
		unified.Message{Role: unified.RoleAssistant, Content: []unified.ContentPart{unified.TextPart{Text: "answer"}}},
		unified.Message{Role: unified.RoleUser, Content: []unified.ContentPart{unified.TextPart{Text: "two"}}},
	)
	events, err = client.Request(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if got := responseText(resp); got != "write retry ok" {
		t.Fatalf("text = %q, want write retry ok", got)
	}
	if len(wsFake.seen) != 2 {
		t.Fatalf("websocket opens = %d, want initial plus fresh retry", len(wsFake.seen))
	}
	if len(httpFake.Seen) != 0 {
		t.Fatalf("http fallback opens = %d, want 0", len(httpFake.Seen))
	}
	if len(wsFake.bodies) != 3 {
		t.Fatalf("websocket request bodies = %d, want initial, failed write, fresh retry", len(wsFake.bodies))
	}
	var freshBody map[string]any
	if err := json.Unmarshal(wsFake.bodies[2], &freshBody); err != nil {
		t.Fatal(err)
	}
	if _, ok := freshBody["previous_response_id"]; ok {
		t.Fatalf("fresh websocket retry should replay without previous_response_id: %#v", freshBody)
	}
}

func TestClientDoesNotUseInternalPreviousResponseIDAcrossBranches(t *testing.T) {
	wsFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`{"type":"response.created","response":{"id":"resp_1","model":"gpt-5.4","status":"in_progress"}}`),
		[]byte(`{"type":"response.done","response":{"id":"resp_1","model":"gpt-5.4","status":"completed"}}`),
	}}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(&transport.FakeByteStreamTransport{}),
		WithWebSocketTransport(wsFake),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{
		Model:  "codex",
		Stream: true,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "one"}},
		}},
	}
	if err := unified.SetCodexExtensions(&req.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
		SessionID:       "sess",
		BranchID:        "main",
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

	wsFake.Frames = [][]byte{
		[]byte(`{"type":"response.created","response":{"id":"resp_2","model":"gpt-5.4","status":"in_progress"}}`),
		[]byte(`{"type":"response.done","response":{"id":"resp_2","model":"gpt-5.4","status":"completed"}}`),
	}
	if err := unified.SetCodexExtensions(&req.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
		SessionID:       "sess",
		BranchID:        "other",
	}); err != nil {
		t.Fatal(err)
	}
	events, err = client.Request(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	var secondBody map[string]any
	if err := json.NewDecoder(wsFake.Seen[1].Body).Decode(&secondBody); err != nil {
		t.Fatal(err)
	}
	if _, ok := secondBody["previous_response_id"]; ok {
		t.Fatalf("branch switch should not include previous_response_id: %#v", secondBody)
	}
}

func TestClientFallsBackToHTTPSSEWhenWebSocketOpenFails(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_http","model":"gpt-5.4","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.output_text.delta","response_id":"resp_http","output_index":0,"content_index":0,"delta":"http ok"}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_http","model":"gpt-5.4","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	wsFake := &transport.FakeByteStreamTransport{OpenErr: context.Canceled}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "codex", Stream: true}
	if err := unified.SetCodexExtensions(&req.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
		SessionID:       "sess",
	}); err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if got := responseText(resp); got != "http ok" {
		t.Fatalf("text = %q, want http ok", got)
	}
	if len(wsFake.Seen) != 2 || len(httpFake.Seen) != 1 {
		t.Fatalf("ws seen=%d http seen=%d, want both", len(wsFake.Seen), len(httpFake.Seen))
	}
}

func TestClientFallsBackToHTTPSSEWhenWebSocketClosesBeforeResponse(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_http","model":"gpt-5.4","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.output_text.delta","response_id":"resp_http","output_index":0,"content_index":0,"delta":"http fallback ok"}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_http","model":"gpt-5.4","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	wsFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`{"type":"codex.rate_limits","rate_limits":[]}`),
	}}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "codex", Stream: true}
	if err := unified.SetCodexExtensions(&req.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
		SessionID:       "sess",
	}); err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if got := responseText(resp); got != "http fallback ok" {
		t.Fatalf("text = %q, want http fallback ok", got)
	}
	if len(wsFake.Seen) != 2 || len(httpFake.Seen) != 1 {
		t.Fatalf("ws seen=%d http seen=%d, want both", len(wsFake.Seen), len(httpFake.Seen))
	}
}

func TestClientRetriesWebSocketAfterPreStreamFallback(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_http","model":"gpt-5.4","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_http","model":"gpt-5.4","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	wsFake := &reusableWebSocketTransport{
		turnFrames: [][][]byte{
			{
				[]byte(`{"type":"codex.rate_limits","rate_limits":[]}`),
			},
			{
				[]byte(`{"type":"codex.rate_limits","rate_limits":[]}`),
			},
			{
				[]byte(`{"type":"response.created","response":{"id":"resp_ws","model":"gpt-5.4","status":"in_progress"}}`),
				[]byte(`{"type":"response.done","response":{"id":"resp_ws","model":"gpt-5.4","status":"completed"}}`),
			},
		},
	}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "codex", Stream: true}
	if err := unified.SetCodexExtensions(&req.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
		SessionID:       "sess",
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
	events, err = client.Request(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != "resp_ws" {
		t.Fatalf("second response id = %q, want resp_ws", resp.ID)
	}
	if len(wsFake.seen) != 3 {
		t.Fatalf("websocket opens = %d, want retry after fallback", len(wsFake.seen))
	}
	if len(httpFake.Seen) != 1 {
		t.Fatalf("http opens = %d, want only first-turn fallback", len(httpFake.Seen))
	}
}

func TestClientKeepsOneShotOnHTTPSSE(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_http","model":"gpt-5.4","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_http","model":"gpt-5.4","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	wsFake := &transport.FakeByteStreamTransport{}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "codex", Stream: true}
	if err := unified.SetCodexExtensions(&req.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionOneShot,
		SessionID:       "sess",
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
	if len(wsFake.Seen) != 0 || len(httpFake.Seen) != 1 {
		t.Fatalf("ws seen=%d http seen=%d, want http only", len(wsFake.Seen), len(httpFake.Seen))
	}
}

func TestClientKeepsSessionOnHTTPSSEWhenWebSocketDisabled(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_http","model":"gpt-5.4","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_http","model":"gpt-5.4","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	wsFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`{"type":"response.created","response":{"id":"resp_ws","model":"gpt-5.4","status":"in_progress"}}`),
		[]byte(`{"type":"response.done","response":{"id":"resp_ws","model":"gpt-5.4","status":"completed"}}`),
	}}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
		WithWebSocketEnabled(false),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "codex", Stream: true}
	if err := unified.SetCodexExtensions(&req.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
		SessionID:       "sess",
	}); err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	first := <-events
	metadata, ok := first.(unified.ProviderExecutionEvent)
	if !ok {
		t.Fatalf("first event = %T, want ProviderExecutionEvent", first)
	}
	if metadata.Transport != unified.TransportHTTPSSE || metadata.InternalContinuation != unified.ContinuationReplay {
		t.Fatalf("metadata = %+v", metadata)
	}
	if _, err := unified.Collect(context.Background(), events); err != nil {
		t.Fatal(err)
	}
	if len(wsFake.Seen) != 0 || len(httpFake.Seen) != 1 {
		t.Fatalf("ws seen=%d http seen=%d, want http only", len(wsFake.Seen), len(httpFake.Seen))
	}
}

func TestClientKeepsSessionWithoutStableIDOnHTTPSSE(t *testing.T) {
	httpFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_http","model":"gpt-5.4","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_http","model":"gpt-5.4","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}}
	wsFake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`{"type":"response.created","response":{"id":"resp_ws","model":"gpt-5.4","status":"in_progress"}}`),
		[]byte(`{"type":"response.done","response":{"id":"resp_ws","model":"gpt-5.4","status":"completed"}}`),
	}}
	client, err := NewClient(
		WithAccessToken("token"),
		WithBaseURL("https://example.invalid/backend-api"),
		WithTransport(httpFake),
		WithWebSocketTransport(wsFake),
	)
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{Model: "codex", Stream: true}
	if err := unified.SetCodexExtensions(&req.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
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
	if len(wsFake.Seen) != 0 || len(httpFake.Seen) != 1 {
		t.Fatalf("ws seen=%d http seen=%d, want http only without stable session id", len(wsFake.Seen), len(httpFake.Seen))
	}
}

func responseText(resp unified.Response) string {
	var b strings.Builder
	for _, part := range resp.Content {
		if text, ok := part.(unified.TextPart); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

type reusableWebSocketTransport struct {
	mu         sync.Mutex
	next       int
	turnFrames [][][]byte
	turnErrs   []error
	writeErrs  []error
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
	if s.owner.next < len(s.owner.writeErrs) && s.owner.writeErrs[s.owner.next] != nil {
		return s.owner.writeErrs[s.owner.next]
	}
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
