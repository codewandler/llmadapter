package providers_test

import (
	"context"
	"strings"
	"testing"

	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	bedrockmessages "github.com/codewandler/llmadapter/providers/bedrock/messages"
	bedrockresponses "github.com/codewandler/llmadapter/providers/bedrock/responses"
	minimaxmessages "github.com/codewandler/llmadapter/providers/minimax/messages"
	openaichat "github.com/codewandler/llmadapter/providers/openai/chatcompletions"
	codex "github.com/codewandler/llmadapter/providers/openai/codex"
	openairesponses "github.com/codewandler/llmadapter/providers/openai/responses"
	openroutermessages "github.com/codewandler/llmadapter/providers/openrouter/messages"
	openrouterresponses "github.com/codewandler/llmadapter/providers/openrouter/responses"
	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

func TestProviderUsageRawConformance(t *testing.T) {
	cases := []struct {
		name      string
		frames    [][]byte
		newClient func(transport.ByteStreamTransport) (unified.Client, error)
		wantRaw   string
	}{
		{
			name:    "openai_chat",
			frames:  chatUsageFrames("gpt-test"),
			wantRaw: `"prompt_tokens":7`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openaichat.NewClient(openaichat.WithAPIKey("key"), openaichat.WithTransport(t))
			},
		},
		{
			name:    "openai_responses",
			frames:  responsesUsageFrames("gpt-test"),
			wantRaw: `"input_tokens":7`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openairesponses.NewClient(openairesponses.WithAPIKey("key"), openairesponses.WithTransport(t))
			},
		},
		{
			name:    "openrouter_responses",
			frames:  responsesUsageFrames("openai/gpt-test"),
			wantRaw: `"input_tokens":7`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openrouterresponses.NewClient(openrouterresponses.WithAPIKey("key"), openrouterresponses.WithTransport(t))
			},
		},
		{
			name:    "codex_responses",
			frames:  responsesUsageFrames("gpt-5.4"),
			wantRaw: `"input_tokens":7`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return codex.NewClient(codex.WithAccessToken("token"), codex.WithTransport(t))
			},
		},
		{
			name:    "bedrock_responses",
			frames:  responsesUsageFrames("openai.gpt-oss-120b"),
			wantRaw: `"input_tokens":7`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return bedrockresponses.NewClient(bedrockresponses.WithAPIKey("key"), bedrockresponses.WithTransport(t))
			},
		},
		{
			name:    "anthropic_messages",
			frames:  anthropicUsageFrames("claude-test"),
			wantRaw: `"cache_read_input_tokens":3`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return anthropic.NewClient(anthropic.WithAPIKey("key"), anthropic.WithTransport(t))
			},
		},
		{
			name:    "bedrock_messages",
			frames:  anthropicUsageFrames("anthropic.claude-opus-4-7"),
			wantRaw: `"cache_read_input_tokens":3`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return bedrockmessages.NewClient(bedrockmessages.WithAPIKey("key"), bedrockmessages.WithTransport(t))
			},
		},
		{
			name:    "claude",
			frames:  anthropicUsageFrames("claude-test"),
			wantRaw: `"cache_read_input_tokens":3`,
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
			name:    "openrouter_messages",
			frames:  anthropicUsageFrames("anthropic/claude-sonnet-4.5"),
			wantRaw: `"cache_read_input_tokens":3`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openroutermessages.NewClient(openroutermessages.WithAPIKey("key"), openroutermessages.WithTransport(t))
			},
		},
		{
			name:    "minimax_messages",
			frames:  anthropicUsageFrames("MiniMax-M2.7"),
			wantRaw: `"cache_read_input_tokens":3`,
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
			resp := collectMetadataSmoke(t, client)
			if !strings.Contains(string(resp.Usage.ProviderRaw), tc.wantRaw) {
				t.Fatalf("provider raw usage = %s, want containing %s; usage=%+v", resp.Usage.ProviderRaw, tc.wantRaw, resp.Usage)
			}
		})
	}
}

func TestProviderUnknownRawEventConformance(t *testing.T) {
	cases := []struct {
		name        string
		frames      [][]byte
		newClient   func(transport.ByteStreamTransport) (unified.Client, error)
		wantAPIKind string
		wantType    string
		wantJSON    string
	}{
		{
			name:        "openai_responses",
			frames:      responsesUnknownEventFrames("gpt-test"),
			wantAPIKind: "openai.responses",
			wantType:    "response.web_search_call.in_progress",
			wantJSON:    `"id":"ws_1"`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openairesponses.NewClient(openairesponses.WithAPIKey("key"), openairesponses.WithTransport(t))
			},
		},
		{
			name:        "openrouter_responses",
			frames:      responsesUnknownEventFrames("openai/gpt-test"),
			wantAPIKind: "openrouter.responses",
			wantType:    "response.web_search_call.in_progress",
			wantJSON:    `"id":"ws_1"`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openrouterresponses.NewClient(openrouterresponses.WithAPIKey("key"), openrouterresponses.WithTransport(t))
			},
		},
		{
			name:        "codex_responses",
			frames:      responsesUnknownEventFrames("gpt-5.4"),
			wantAPIKind: "codex.responses",
			wantType:    "response.web_search_call.in_progress",
			wantJSON:    `"id":"ws_1"`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return codex.NewClient(codex.WithAccessToken("token"), codex.WithTransport(t))
			},
		},
		{
			name:        "bedrock_responses",
			frames:      responsesUnknownEventFrames("openai.gpt-oss-120b"),
			wantAPIKind: "bedrock.responses",
			wantType:    "response.web_search_call.in_progress",
			wantJSON:    `"id":"ws_1"`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return bedrockresponses.NewClient(bedrockresponses.WithAPIKey("key"), bedrockresponses.WithTransport(t))
			},
		},
		{
			name:        "anthropic_messages",
			frames:      anthropicUnknownContentFrames("claude-test"),
			wantAPIKind: "anthropic.messages",
			wantType:    "server_tool_use",
			wantJSON:    `"id":"srv_1"`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return anthropic.NewClient(anthropic.WithAPIKey("key"), anthropic.WithTransport(t))
			},
		},
		{
			name:        "bedrock_messages",
			frames:      anthropicUnknownContentFrames("anthropic.claude-opus-4-7"),
			wantAPIKind: "bedrock.anthropic_messages",
			wantType:    "server_tool_use",
			wantJSON:    `"id":"srv_1"`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return bedrockmessages.NewClient(bedrockmessages.WithAPIKey("key"), bedrockmessages.WithTransport(t))
			},
		},
		{
			name:        "claude",
			frames:      anthropicUnknownContentFrames("claude-test"),
			wantAPIKind: "anthropic.messages",
			wantType:    "server_tool_use",
			wantJSON:    `"id":"srv_1"`,
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
			name:        "openrouter_messages",
			frames:      anthropicUnknownContentFrames("anthropic/claude-sonnet-4.5"),
			wantAPIKind: "openrouter.anthropic_messages",
			wantType:    "server_tool_use",
			wantJSON:    `"id":"srv_1"`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openroutermessages.NewClient(openroutermessages.WithAPIKey("key"), openroutermessages.WithTransport(t))
			},
		},
		{
			name:        "minimax_messages",
			frames:      anthropicUnknownContentFrames("MiniMax-M2.7"),
			wantAPIKind: "minimax.anthropic_messages",
			wantType:    "server_tool_use",
			wantJSON:    `"id":"srv_1"`,
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
			resp := collectMetadataSmoke(t, client)
			raw := firstRawEvent(resp, tc.wantType)
			if raw.APIKind != tc.wantAPIKind || raw.Type != tc.wantType || !strings.Contains(string(raw.JSON), tc.wantJSON) {
				t.Fatalf("raw event = %+v, want api=%s type=%s json containing %s; response=%+v", raw, tc.wantAPIKind, tc.wantType, tc.wantJSON, resp)
			}
		})
	}
}

func collectMetadataSmoke(t *testing.T, client unified.Client) unified.Response {
	t.Helper()
	events, err := client.Request(context.Background(), metadataSmokeRequest())
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func metadataSmokeRequest() unified.Request {
	maxTokens := 16
	return unified.Request{
		Model:           "test-model",
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
		Stream: true,
	}
}

func firstRawEvent(resp unified.Response, eventType string) unified.RawEvent {
	for _, raw := range resp.Raw {
		if raw.Type == eventType {
			return raw
		}
	}
	return unified.RawEvent{}
}

func chatUsageFrames(model string) [][]byte {
	return [][]byte{
		[]byte(`data: {"id":"chatcmpl_1","model":"` + model + `","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":5,"total_tokens":12,"prompt_tokens_details":{"cached_tokens":3},"completion_tokens_details":{"reasoning_tokens":2}}}`),
		[]byte(`data: [DONE]`),
	}
}

func responsesUsageFrames(model string) [][]byte {
	return [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"` + model + `","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.output_text.delta","response_id":"resp_1","output_index":0,"delta":"ok"}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"` + model + `","status":"completed","usage":{"input_tokens":7,"output_tokens":5,"total_tokens":12,"input_tokens_details":{"cached_tokens":3},"output_tokens_details":{"reasoning_tokens":2}}}}`),
		[]byte(`data: [DONE]`),
	}
}

func anthropicUsageFrames(model string) [][]byte {
	return [][]byte{
		[]byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"` + model + `","content":[]}}`),
		[]byte(`event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
		[]byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}`),
		[]byte(`event: content_block_stop
data: {"type":"content_block_stop","index":0}`),
		[]byte(`event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":7,"cache_read_input_tokens":3,"cache_creation_input_tokens":2,"output_tokens":5}}`),
		[]byte(`event: message_stop
data: {"type":"message_stop"}`),
	}
}

func responsesUnknownEventFrames(model string) [][]byte {
	return [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"` + model + `","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.web_search_call.in_progress","response_id":"resp_1","output_index":0,"id":"ws_1","provider":"native"}`),
		[]byte(`data: {"type":"response.output_text.delta","response_id":"resp_1","output_index":1,"delta":"ok"}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"` + model + `","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}
}

func anthropicUnknownContentFrames(model string) [][]byte {
	return [][]byte{
		[]byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"` + model + `","content":[]}}`),
		[]byte(`event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"server_tool_use","id":"srv_1","name":"web_search","input":{"query":"docs"}}}`),
		[]byte(`event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`),
		[]byte(`event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"ok"}}`),
		[]byte(`event: content_block_stop
data: {"type":"content_block_stop","index":1}`),
		[]byte(`event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`),
		[]byte(`event: message_stop
data: {"type":"message_stop"}`),
	}
}
