package providers_test

import (
	"context"
	"errors"
	"testing"

	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	minimaxchat "github.com/codewandler/llmadapter/providers/minimax/chatcompletions"
	minimaxmessages "github.com/codewandler/llmadapter/providers/minimax/messages"
	openaichat "github.com/codewandler/llmadapter/providers/openai/chatcompletions"
	codex "github.com/codewandler/llmadapter/providers/openai/codex"
	openairesponses "github.com/codewandler/llmadapter/providers/openai/responses"
	openrouterchat "github.com/codewandler/llmadapter/providers/openrouter/chatcompletions"
	openroutermessages "github.com/codewandler/llmadapter/providers/openrouter/messages"
	openrouterresponses "github.com/codewandler/llmadapter/providers/openrouter/responses"
	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

func TestProviderMidStreamErrorConformance(t *testing.T) {
	cases := []struct {
		name      string
		frames    [][]byte
		newClient func(transport.ByteStreamTransport) (unified.Client, error)
		wantType  string
		wantCode  string
		wantMsg   string
	}{
		{
			name:     "anthropic_messages",
			frames:   anthropicErrorFrames(),
			wantType: "overloaded_error",
			wantMsg:  "try again",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return anthropic.NewClient(anthropic.WithAPIKey("key"), anthropic.WithTransport(t))
			},
		},
		{
			name:     "minimax_messages",
			frames:   anthropicErrorFrames(),
			wantType: "overloaded_error",
			wantMsg:  "try again",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return minimaxmessages.NewClient(minimaxmessages.WithAPIKey("key"), minimaxmessages.WithTransport(t))
			},
		},
		{
			name:     "openrouter_messages",
			frames:   anthropicErrorFrames(),
			wantType: "overloaded_error",
			wantMsg:  "try again",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openroutermessages.NewClient(openroutermessages.WithAPIKey("key"), openroutermessages.WithTransport(t))
			},
		},
		{
			name:     "openai_chat",
			frames:   chatErrorFrames(),
			wantType: "server_error",
			wantCode: "upstream_failed",
			wantMsg:  "upstream failed",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openaichat.NewClient(openaichat.WithAPIKey("key"), openaichat.WithTransport(t))
			},
		},
		{
			name:     "openrouter_chat",
			frames:   chatErrorFrames(),
			wantType: "server_error",
			wantCode: "upstream_failed",
			wantMsg:  "upstream failed",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openrouterchat.NewClient(openrouterchat.WithAPIKey("key"), openrouterchat.WithTransport(t))
			},
		},
		{
			name:     "minimax_chat",
			frames:   chatErrorFrames(),
			wantType: "server_error",
			wantCode: "upstream_failed",
			wantMsg:  "upstream failed",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return minimaxchat.NewClient(minimaxchat.WithAPIKey("key"), minimaxchat.WithTransport(t))
			},
		},
		{
			name:     "openai_responses",
			frames:   responsesErrorFrames(),
			wantType: "server_error",
			wantCode: "upstream_failed",
			wantMsg:  "upstream failed",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openairesponses.NewClient(openairesponses.WithAPIKey("key"), openairesponses.WithTransport(t))
			},
		},
		{
			name:     "openrouter_responses",
			frames:   responsesErrorFrames(),
			wantType: "server_error",
			wantCode: "upstream_failed",
			wantMsg:  "upstream failed",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openrouterresponses.NewClient(openrouterresponses.WithAPIKey("key"), openrouterresponses.WithTransport(t))
			},
		},
		{
			name:     "codex_responses",
			frames:   responsesErrorFrames(),
			wantType: "server_error",
			wantCode: "upstream_failed",
			wantMsg:  "upstream failed",
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
			events, err := client.Request(context.Background(), errorSmokeRequest())
			if err != nil {
				t.Fatal(err)
			}
			_, err = unified.Collect(context.Background(), events)
			var apiErr *unified.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %T %v, want APIError", err, err)
			}
			if apiErr.Type != tc.wantType || apiErr.Code != tc.wantCode || apiErr.Message != tc.wantMsg {
				t.Fatalf("unexpected APIError: %+v", apiErr)
			}
			if len(apiErr.ProviderRaw) == 0 {
				t.Fatalf("missing provider raw error payload: %+v", apiErr)
			}
		})
	}
}

func TestProviderResponseObjectErrorConformance(t *testing.T) {
	cases := []struct {
		name      string
		newClient func(transport.ByteStreamTransport) (unified.Client, error)
	}{
		{
			name: "openai_responses",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openairesponses.NewClient(openairesponses.WithAPIKey("key"), openairesponses.WithTransport(t))
			},
		},
		{
			name: "openrouter_responses",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openrouterresponses.NewClient(openrouterresponses.WithAPIKey("key"), openrouterresponses.WithTransport(t))
			},
		},
		{
			name: "codex_responses",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return codex.NewClient(codex.WithAccessToken("token"), codex.WithTransport(t))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := tc.newClient(&transport.FakeByteStreamTransport{Frames: responsesObjectErrorFrames()})
			if err != nil {
				t.Fatal(err)
			}
			events, err := client.Request(context.Background(), errorSmokeRequest())
			if err != nil {
				t.Fatal(err)
			}
			_, err = unified.Collect(context.Background(), events)
			var apiErr *unified.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %T %v, want APIError", err, err)
			}
			if apiErr.Type != "invalid_request_error" || apiErr.Code != "bad_model" || apiErr.Message != "unknown model" {
				t.Fatalf("unexpected APIError: %+v", apiErr)
			}
			if len(apiErr.ProviderRaw) == 0 {
				t.Fatalf("missing provider raw error payload: %+v", apiErr)
			}
		})
	}
}

func anthropicErrorFrames() [][]byte {
	return [][]byte{
		[]byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-test","content":[]}}`),
		[]byte(`event: error
data: {"type":"error","error":{"type":"overloaded_error","message":"try again"}}`),
	}
}

func chatErrorFrames() [][]byte {
	return [][]byte{
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant"}}]}`),
		[]byte(`data: {"error":{"type":"server_error","code":"upstream_failed","message":"upstream failed"}}`),
	}
}

func responsesErrorFrames() [][]byte {
	return [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-test","status":"in_progress"}}`),
		[]byte(`data: {"type":"error","error":{"type":"server_error","code":"upstream_failed","message":"upstream failed"}}`),
	}
}

func responsesObjectErrorFrames() [][]byte {
	return [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-test","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.failed","response":{"id":"resp_1","model":"gpt-test","status":"failed","error":{"type":"invalid_request_error","code":"bad_model","message":"unknown model"}}}`),
	}
}
