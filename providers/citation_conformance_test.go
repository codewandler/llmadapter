package providers_test

import (
	"context"
	"testing"

	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	minimaxmessages "github.com/codewandler/llmadapter/providers/minimax/messages"
	codex "github.com/codewandler/llmadapter/providers/openai/codex"
	openairesponses "github.com/codewandler/llmadapter/providers/openai/responses"
	openroutermessages "github.com/codewandler/llmadapter/providers/openrouter/messages"
	openrouterresponses "github.com/codewandler/llmadapter/providers/openrouter/responses"
	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

func TestProviderCitationConformance(t *testing.T) {
	cases := []struct {
		name      string
		frames    [][]byte
		newClient func(transport.ByteStreamTransport) (unified.Client, error)
		want      unified.Citation
		wantMeta  string
	}{
		{
			name:     "openai_responses",
			frames:   responsesCitationFrames("gpt-test"),
			want:     responsesCitation(),
			wantMeta: "web",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openairesponses.NewClient(openairesponses.WithAPIKey("key"), openairesponses.WithTransport(t))
			},
		},
		{
			name:     "openrouter_responses",
			frames:   responsesCitationFrames("openai/gpt-test"),
			want:     responsesCitation(),
			wantMeta: "web",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openrouterresponses.NewClient(openrouterresponses.WithAPIKey("key"), openrouterresponses.WithTransport(t))
			},
		},
		{
			name:     "codex_responses",
			frames:   responsesCitationFrames("gpt-5.4"),
			want:     responsesCitation(),
			wantMeta: "web",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return codex.NewClient(codex.WithAccessToken("token"), codex.WithTransport(t))
			},
		},
		{
			name:     "anthropic_messages",
			frames:   anthropicCitationFrames("claude-test"),
			want:     anthropicCitation(),
			wantMeta: "pdf",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return anthropic.NewClient(anthropic.WithAPIKey("key"), anthropic.WithTransport(t))
			},
		},
		{
			name:     "claude",
			frames:   anthropicCitationFrames("claude-test"),
			want:     anthropicCitation(),
			wantMeta: "pdf",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return anthropic.NewClient(
					anthropic.WithBearerTokenProvider(anthropic.NewStaticTokenProvider(anthropic.NewStaticBearerToken("token"))),
					anthropic.WithClaudeHeaders(),
					anthropic.WithClaudeCodePreflight(),
					anthropic.WithTransport(t),
				)
			},
		},
		{
			name:     "openrouter_messages",
			frames:   anthropicCitationFrames("anthropic/claude-sonnet-4.5"),
			want:     anthropicCitation(),
			wantMeta: "pdf",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openroutermessages.NewClient(openroutermessages.WithAPIKey("key"), openroutermessages.WithTransport(t))
			},
		},
		{
			name:     "minimax_messages",
			frames:   anthropicCitationFrames("MiniMax-M2.7"),
			want:     anthropicCitation(),
			wantMeta: "pdf",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return minimaxmessages.NewClient(minimaxmessages.WithAPIKey("key"), minimaxmessages.WithTransport(t))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := tc.newClient(&transport.FakeByteStreamTransport{Frames: tc.frames})
			if err != nil {
				t.Fatal(err)
			}
			events, err := client.Request(context.Background(), citationSmokeRequest())
			if err != nil {
				t.Fatal(err)
			}
			resp, err := unified.Collect(context.Background(), events)
			if err != nil {
				t.Fatal(err)
			}
			if len(resp.Citations) != 1 {
				t.Fatalf("citations = %+v, response=%+v", resp.Citations, resp)
			}
			got := resp.Citations[0]
			if got.Index != 0 {
				t.Fatalf("citation index = %d, want 0", got.Index)
			}
			assertCitation(t, got.Citation, tc.want, tc.wantMeta)
		})
	}
}

func citationSmokeRequest() unified.Request {
	maxTokens := 16
	return unified.Request{
		Model:           "test-model",
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "answer with citation"}},
		}},
		Stream: true,
	}
}

func assertCitation(t *testing.T, got, want unified.Citation, wantMeta string) {
	t.Helper()
	if got.Type != want.Type || got.Text != want.Text || got.URL != want.URL || got.Title != want.Title || got.DocumentID != want.DocumentID || got.StartOffset != want.StartOffset || got.EndOffset != want.EndOffset {
		t.Fatalf("citation = %+v, want %+v", got, want)
	}
	if got.Meta["source"] != wantMeta {
		t.Fatalf("citation meta = %+v, want source=%s", got.Meta, wantMeta)
	}
}

func responsesCitation() unified.Citation {
	return unified.Citation{
		Type:        "url_citation",
		Text:        "quoted text",
		URL:         "https://example.test/doc",
		Title:       "Example",
		StartOffset: 2,
		EndOffset:   8,
	}
}

func anthropicCitation() unified.Citation {
	return unified.Citation{
		Type:        "char_location",
		Text:        "quoted text",
		Title:       "Manual",
		DocumentID:  "doc_1",
		StartOffset: 2,
		EndOffset:   8,
	}
}

func responsesCitationFrames(model string) [][]byte {
	return [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"` + model + `","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.output_text.delta","response_id":"resp_1","output_index":0,"delta":"answer"}`),
		[]byte(`data: {"type":"response.output_text.annotation.added","response_id":"resp_1","output_index":0,"annotation":{"type":"url_citation","url":"https://example.test/doc","title":"Example","text":"quoted text","start_index":2,"end_index":8,"source":"web"}}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"` + model + `","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}
}

func anthropicCitationFrames(model string) [][]byte {
	return [][]byte{
		[]byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"` + model + `","content":[]}}`),
		[]byte(`event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":"","citations":[{"type":"char_location","cited_text":"quoted text","document_title":"Manual","document_id":"doc_1","start_char_index":2,"end_char_index":8,"source":"pdf"}]}}`),
		[]byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"answer"}}`),
		[]byte(`event: content_block_stop
data: {"type":"content_block_stop","index":0}`),
		[]byte(`event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`),
		[]byte(`event: message_stop
data: {"type":"message_stop"}`),
	}
}
