package providers_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

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

func TestProviderClientsReturnTransportAPIErrorUnwrapped(t *testing.T) {
	apiErr := &unified.APIError{
		StatusCode:  http.StatusTooManyRequests,
		Type:        "rate_limit_error",
		Code:        "rate_limited",
		Message:     "slow down",
		RetryAfter:  3 * time.Second,
		ProviderRaw: []byte(`{"error":{"message":"slow down"}}`),
	}
	cases := []struct {
		name      string
		newClient func(transport.ByteStreamTransport) (unified.Client, error)
	}{
		{
			name: "anthropic_messages",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return anthropic.NewClient(anthropic.WithAPIKey("test"), anthropic.WithTransport(t))
			},
		},
		{
			name: "openai_chat",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openaichat.NewClient(openaichat.WithAPIKey("test"), openaichat.WithTransport(t))
			},
		},
		{
			name: "openai_responses",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openairesponses.NewClient(openairesponses.WithAPIKey("test"), openairesponses.WithTransport(t))
			},
		},
		{
			name: "codex_responses",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return codex.NewClient(codex.WithAccessToken("test"), codex.WithTransport(t))
			},
		},
		{
			name: "openrouter_chat",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openrouterchat.NewClient(openrouterchat.WithAPIKey("test"), openrouterchat.WithTransport(t))
			},
		},
		{
			name: "openrouter_responses",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openrouterresponses.NewClient(openrouterresponses.WithAPIKey("test"), openrouterresponses.WithTransport(t))
			},
		},
		{
			name: "openrouter_messages",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openroutermessages.NewClient(openroutermessages.WithAPIKey("test"), openroutermessages.WithTransport(t))
			},
		},
		{
			name: "minimax_chat",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return minimaxchat.NewClient(minimaxchat.WithAPIKey("test"), minimaxchat.WithTransport(t))
			},
		},
		{
			name: "minimax_messages",
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return minimaxmessages.NewClient(minimaxmessages.WithAPIKey("test"), minimaxmessages.WithTransport(t))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := tc.newClient(&transport.FakeByteStreamTransport{OpenErr: apiErr})
			if err != nil {
				t.Fatal(err)
			}
			events, err := client.Request(context.Background(), errorSmokeRequest())
			if err == nil {
				_, err = unified.Collect(context.Background(), events)
			}
			var got *unified.APIError
			if !errors.As(err, &got) {
				t.Fatalf("error = %T %v, want APIError", err, err)
			}
			if got != apiErr {
				t.Fatalf("APIError was wrapped or replaced: got=%+v want=%+v", got, apiErr)
			}
		})
	}
}

func errorSmokeRequest() unified.Request {
	maxTokens := 8
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
