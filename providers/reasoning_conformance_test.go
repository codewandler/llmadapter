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

func TestReasoningStreamConformance(t *testing.T) {
	cases := []struct {
		name          string
		frames        [][]byte
		newClient     func(transport.ByteStreamTransport) (unified.Client, error)
		wantText      string
		wantSignature string
	}{
		{
			name:          "anthropic_messages",
			frames:        anthropicReasoningFrames("claude-test"),
			wantText:      "think",
			wantSignature: "sig",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return anthropic.NewClient(anthropic.WithAPIKey("key"), anthropic.WithTransport(t))
			},
		},
		{
			name:          "minimax_messages",
			frames:        anthropicReasoningFrames("MiniMax-M2.7"),
			wantText:      "think",
			wantSignature: "sig",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return minimaxmessages.NewClient(minimaxmessages.WithAPIKey("key"), minimaxmessages.WithTransport(t))
			},
		},
		{
			name:          "openrouter_messages",
			frames:        anthropicReasoningFrames("anthropic/claude-sonnet-4.5"),
			wantText:      "think",
			wantSignature: "sig",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openroutermessages.NewClient(openroutermessages.WithAPIKey("key"), openroutermessages.WithTransport(t))
			},
		},
		{
			name:     "openai_responses",
			frames:   responsesReasoningFrames("gpt-test"),
			wantText: "thinking",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openairesponses.NewClient(openairesponses.WithAPIKey("key"), openairesponses.WithTransport(t))
			},
		},
		{
			name:     "openrouter_responses",
			frames:   responsesReasoningFrames("openai/gpt-test"),
			wantText: "thinking",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openrouterresponses.NewClient(openrouterresponses.WithAPIKey("key"), openrouterresponses.WithTransport(t))
			},
		},
		{
			name:     "codex_responses",
			frames:   responsesReasoningFrames("gpt-5.4"),
			wantText: "thinking",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return codex.NewClient(codex.WithAccessToken("token"), codex.WithTransport(t))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := tc.newClient(&transport.FakeByteStreamTransport{Frames: tc.frames})
			if err != nil {
				t.Fatal(err)
			}
			events, err := client.Request(context.Background(), reasoningSmokeRequest())
			if err != nil {
				t.Fatal(err)
			}
			resp, err := unified.Collect(context.Background(), events)
			if err != nil {
				t.Fatal(err)
			}
			part := firstReasoningPart(resp)
			if part.Text != tc.wantText {
				t.Fatalf("reasoning text = %q, want %q; response=%+v", part.Text, tc.wantText, resp)
			}
			if tc.wantSignature != "" && part.Signature != tc.wantSignature {
				t.Fatalf("reasoning signature = %q, want %q; response=%+v", part.Signature, tc.wantSignature, resp)
			}
		})
	}
}

func reasoningSmokeRequest() unified.Request {
	maxTokens := 4096
	budget := 1024
	return unified.Request{
		Model:           "test-model",
		MaxOutputTokens: &maxTokens,
		Reasoning:       &unified.ReasoningConfig{Effort: unified.ReasoningEffortHigh, MaxTokens: &budget, Expose: true},
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "think then answer"}},
		}},
		Stream: true,
	}
}

func firstReasoningPart(resp unified.Response) unified.ReasoningPart {
	for _, part := range resp.Content {
		if reasoning, ok := part.(unified.ReasoningPart); ok {
			return reasoning
		}
	}
	return unified.ReasoningPart{}
}

func anthropicReasoningFrames(model string) [][]byte {
	return [][]byte{
		[]byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"` + model + `","content":[]}}`),
		[]byte(`event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`),
		[]byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"think"}}`),
		[]byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig"}}`),
		[]byte(`event: content_block_stop
data: {"type":"content_block_stop","index":0}`),
		[]byte(`event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`),
		[]byte(`event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"answer"}}`),
		[]byte(`event: content_block_stop
data: {"type":"content_block_stop","index":1}`),
		[]byte(`event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}`),
		[]byte(`event: message_stop
data: {"type":"message_stop"}`),
	}
}

func responsesReasoningFrames(model string) [][]byte {
	return [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"` + model + `","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.reasoning_summary_text.delta","response_id":"resp_1","output_index":0,"delta":"thinking"}`),
		[]byte(`data: {"type":"response.reasoning_summary_text.done","response_id":"resp_1","output_index":0}`),
		[]byte(`data: {"type":"response.output_text.delta","response_id":"resp_1","output_index":1,"delta":"answer"}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"` + model + `","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}
}
