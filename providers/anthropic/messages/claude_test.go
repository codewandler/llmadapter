package messages

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

func TestClaudeCodeOptionsShapeRequest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"userID":"device-1","oauthAccount":{"accountUuid":"acct-1"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	fake := &transport.FakeByteStreamTransport{}
	client, err := NewClient(
		WithBaseURL("https://anthropic.test"),
		WithTransport(fake),
		WithBearerTokenProvider(NewStaticTokenProvider(NewStaticBearerToken("oauth-token"))),
		WithClaudeHeaders(),
		WithClaudeCodePreflight(),
	)
	if err != nil {
		t.Fatal(err)
	}

	maxTokens := 64
	events, err := client.Request(context.Background(), unified.Request{
		Model:           "claude-test",
		MaxOutputTokens: &maxTokens,
		Instructions: []unified.Instruction{{
			Kind:    unified.InstructionSystem,
			Content: []unified.ContentPart{unified.TextPart{Text: "be concise"}},
		}},
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
		Stream: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for range events {
	}

	if len(fake.Seen) != 1 {
		t.Fatalf("requests = %d, want 1", len(fake.Seen))
	}
	seen := fake.Seen[0]
	u, err := url.Parse(seen.URL)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("beta") != "true" {
		t.Fatalf("missing beta query in %q", seen.URL)
	}
	if seen.Header.Get("Authorization") != "Bearer oauth-token" || seen.Header.Get("x-api-key") != "" {
		t.Fatalf("unexpected auth headers: %v", seen.Header)
	}
	if seen.Header.Get("User-Agent") != claudeUserAgent || !strings.Contains(seen.Header.Get("Anthropic-Beta"), "oauth-2025-04-20") {
		t.Fatalf("unexpected Claude headers: %v", seen.Header)
	}
	if seen.Header.Get("X-App") != "cli" || seen.Method != http.MethodPost {
		t.Fatalf("unexpected transport request: %+v", seen)
	}
	if seen.Header.Get("Accept-Encoding") != transport.ExtendedAcceptEncoding {
		t.Fatalf("unexpected Accept-Encoding: %v", seen.Header)
	}

	body, err := io.ReadAll(seen.Body)
	if err != nil {
		t.Fatal(err)
	}
	var wire MessageRequest
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	system := wire.System.Text()
	if !strings.Contains(system, claudeBillingHeader) || !strings.Contains(system, claudeSystemCore) || !strings.Contains(system, "be concise") {
		t.Fatalf("unexpected system preflight: %q", system)
	}
	rawUserID, ok := wire.Metadata["user_id"].(string)
	if !ok {
		t.Fatalf("missing user metadata: %+v", wire.Metadata)
	}
	var userID map[string]string
	if err := json.Unmarshal([]byte(rawUserID), &userID); err != nil {
		t.Fatal(err)
	}
	if userID["device_id"] != "device-1" || userID["account_uuid"] != "acct-1" || userID["session_id"] == "" {
		t.Fatalf("unexpected user metadata: %+v", userID)
	}
}

func TestClaudeHeadersSetQuotaProvider(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{
		Header: http.Header{
			"Anthropic-Ratelimit-Requests-Limit":     []string{"10"},
			"Anthropic-Ratelimit-Requests-Remaining": []string{"5"},
		},
		Frames: [][]byte{
			[]byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg","role":"assistant","model":"claude-test"}}`),
			[]byte(`event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`),
			[]byte(`event: message_stop
data: {"type":"message_stop"}`),
		},
	}
	client, err := NewClient(
		WithBaseURL("https://anthropic.test"),
		WithTransport(fake),
		WithBearerTokenProvider(NewStaticTokenProvider(NewStaticBearerToken("oauth-token"))),
		WithClaudeHeaders(),
	)
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 64
	events, err := client.Request(context.Background(), unified.Request{
		Model:           "claude-test",
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
	if len(resp.Quotas) != 1 || resp.Quotas[0].Provider != "claude" {
		t.Fatalf("quotas = %+v, want Claude quota provider", resp.Quotas)
	}
}

func TestSystemCacheControlProcessor(t *testing.T) {
	req := MessageRequest{System: NewSystemContent(
		ContentBlock{Type: "text", Text: "first"},
		ContentBlock{Type: "text", Text: "last"},
	)}
	if err := (systemCacheControlProcessor{ttl: "5m"}).ProcessProviderRequest(context.Background(), &req); err != nil {
		t.Fatal(err)
	}
	if req.System.Blocks[0].Cache != nil {
		t.Fatalf("expected first block to remain uncached: %+v", req.System.Blocks)
	}
	if got := req.System.Blocks[1].Cache; got == nil || got.Type != "ephemeral" || got.TTL != "5m" {
		t.Fatalf("unexpected cache control: %+v", req.System.Blocks)
	}
}
