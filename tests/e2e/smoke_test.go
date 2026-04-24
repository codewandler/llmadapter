package e2e

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	openai "github.com/codewandler/llmadapter/providers/openai/chatcompletions"
	openrouter "github.com/codewandler/llmadapter/providers/openrouter/chatcompletions"
	openroutermessages "github.com/codewandler/llmadapter/providers/openrouter/messages"
	openrouterresponses "github.com/codewandler/llmadapter/providers/openrouter/responses"
	"github.com/codewandler/llmadapter/unified"
)

type smokeProvider struct {
	name      string
	apiKeyEnv []string
	modelEnv  string
	model     string
	tools     bool
	newClient func(apiKey string) (unified.Client, error)
}

func TestSmokeTextStream(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}

	for _, provider := range smokeProviders() {
		t.Run(provider.name, func(t *testing.T) {
			client, model := newSmokeClient(t, provider)
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()

			maxTokens := 64
			events, err := client.Request(ctx, unified.Request{
				Model:           model,
				MaxOutputTokens: &maxTokens,
				Messages: []unified.Message{
					{
						Role: unified.RoleUser,
						Content: []unified.ContentPart{
							unified.TextPart{Text: "Reply with exactly: llmadapter smoke ok"},
						},
					},
				},
				Stream: true,
			})
			if err != nil {
				t.Fatalf("request: %v", err)
			}

			resp, err := unified.Collect(ctx, events)
			if err != nil {
				t.Fatalf("collect: %v", err)
			}

			text := responseText(resp)
			if text == "" {
				t.Fatalf("empty response content: %+v", resp)
			}
			if !strings.Contains(strings.ToLower(text), "llmadapter smoke ok") {
				t.Fatalf("response text %q does not contain expected smoke marker", text)
			}
			if resp.FinishReason == "" {
				t.Fatalf("missing finish reason in response: %+v", resp)
			}
			if resp.Usage.TotalTokens == 0 && resp.Usage.InputTokens == 0 && resp.Usage.OutputTokens == 0 {
				t.Fatalf("missing usage in response: %+v", resp)
			}
		})
	}
}

func TestSmokeToolUse(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}

	for _, provider := range smokeProviders() {
		t.Run(provider.name, func(t *testing.T) {
			if !provider.tools {
				t.Skipf("%s does not advertise tool smoke support in this slice", provider.name)
			}
			client, model := newSmokeClient(t, provider)
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()

			maxTokens := 128
			events, err := client.Request(ctx, unified.Request{
				Model:           model,
				MaxOutputTokens: &maxTokens,
				Messages: []unified.Message{
					{
						Role: unified.RoleUser,
						Content: []unified.ContentPart{
							unified.TextPart{Text: "Use the lookup_city tool with city set to Berlin. Do not answer directly."},
						},
					},
				},
				Tools: []unified.Tool{
					{
						Kind:        unified.ToolKindFunction,
						Name:        "lookup_city",
						Description: "Looks up a city by name.",
						InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"],"additionalProperties":false}`),
					},
				},
				ToolChoice: &unified.ToolChoice{Mode: unified.ToolChoiceTool, Name: "lookup_city"},
				Stream:     true,
			})
			if err != nil {
				t.Fatalf("request: %v", err)
			}

			resp, err := unified.Collect(ctx, events)
			if err != nil {
				t.Fatalf("collect: %v", err)
			}
			if resp.FinishReason != unified.FinishReasonToolCall {
				t.Fatalf("finish reason = %q, want %q; response=%+v", resp.FinishReason, unified.FinishReasonToolCall, resp)
			}
			if len(resp.ToolCalls) != 1 {
				t.Fatalf("tool calls = %+v", resp.ToolCalls)
			}
			if resp.ToolCalls[0].Name != "lookup_city" {
				t.Fatalf("tool name = %q", resp.ToolCalls[0].Name)
			}
			if !strings.Contains(strings.ToLower(string(resp.ToolCalls[0].Arguments)), "berlin") {
				t.Fatalf("tool args = %s", resp.ToolCalls[0].Arguments)
			}
		})
	}
}

func TestSmokeToolResultContinuation(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}

	for _, provider := range smokeProviders() {
		t.Run(provider.name, func(t *testing.T) {
			if !provider.tools {
				t.Skipf("%s does not advertise tool smoke support in this slice", provider.name)
			}
			client, model := newSmokeClient(t, provider)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			maxTokens := 128
			userMessage := unified.Message{
				Role: unified.RoleUser,
				Content: []unified.ContentPart{
					unified.TextPart{Text: "Use the lookup_city tool with city set to Berlin. Do not answer directly."},
				},
			}
			tool := unified.Tool{
				Kind:        unified.ToolKindFunction,
				Name:        "lookup_city",
				Description: "Looks up a city by name.",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"],"additionalProperties":false}`),
			}

			events, err := client.Request(ctx, unified.Request{
				Model:           model,
				MaxOutputTokens: &maxTokens,
				Messages:        []unified.Message{userMessage},
				Tools:           []unified.Tool{tool},
				ToolChoice:      &unified.ToolChoice{Mode: unified.ToolChoiceTool, Name: "lookup_city"},
				Stream:          true,
			})
			if err != nil {
				t.Fatalf("tool request: %v", err)
			}

			toolResp, err := unified.Collect(ctx, events)
			if err != nil {
				t.Fatalf("collect tool response: %v", err)
			}
			if toolResp.FinishReason != unified.FinishReasonToolCall || len(toolResp.ToolCalls) != 1 {
				t.Fatalf("tool response = %+v", toolResp)
			}

			toolCall := toolResp.ToolCalls[0]
			events, err = client.Request(ctx, unified.Request{
				Model:           model,
				MaxOutputTokens: &maxTokens,
				Messages: []unified.Message{
					userMessage,
					{
						Role:      unified.RoleAssistant,
						ToolCalls: []unified.ToolCall{toolCall},
					},
					{
						Role: unified.RoleTool,
						ToolResults: []unified.ToolResult{{
							ToolCallID: toolCall.ID,
							Name:       toolCall.Name,
							Content: []unified.ContentPart{
								unified.TextPart{Text: "lookup_city result: Berlin marker is llmadapter tool loop ok. Reply with that exact marker."},
							},
						}},
					},
				},
				Stream: true,
			})
			if err != nil {
				t.Fatalf("continuation request: %v", err)
			}

			finalResp, err := unified.Collect(ctx, events)
			if err != nil {
				t.Fatalf("collect continuation response: %v", err)
			}
			text := strings.ToLower(responseText(finalResp))
			if !strings.Contains(text, "llmadapter tool loop ok") {
				t.Fatalf("continuation text %q does not contain expected marker; response=%+v", text, finalResp)
			}
			if finalResp.FinishReason == "" || finalResp.FinishReason == unified.FinishReasonToolCall {
				t.Fatalf("unexpected continuation finish reason: %+v", finalResp)
			}
		})
	}
}

func smokeProviders() []smokeProvider {
	return []smokeProvider{
		{
			name:      "anthropic",
			apiKeyEnv: []string{"ANTHROPIC_API_KEY"},
			modelEnv:  "ANTHROPIC_MODEL",
			model:     "claude-haiku-4-5-20251001",
			tools:     true,
			newClient: func(apiKey string) (unified.Client, error) {
				return anthropic.NewClient(anthropic.WithAPIKey(apiKey))
			},
		},
		{
			name:      "openai_chat",
			apiKeyEnv: []string{"OPENAI_API_KEY", "OPENAI_KEY"},
			modelEnv:  "OPENAI_MODEL",
			model:     "gpt-4.1-mini",
			tools:     true,
			newClient: func(apiKey string) (unified.Client, error) {
				return openai.NewClient(openai.WithAPIKey(apiKey))
			},
		},
		{
			name:      "openrouter_chat",
			apiKeyEnv: []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
			modelEnv:  "OPENROUTER_MODEL",
			model:     "openai/gpt-4.1-mini",
			tools:     true,
			newClient: func(apiKey string) (unified.Client, error) {
				return openrouter.NewClient(openrouter.WithAPIKey(apiKey))
			},
		},
		{
			name:      "openrouter_responses",
			apiKeyEnv: []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
			modelEnv:  "OPENROUTER_RESPONSES_MODEL",
			model:     "openai/gpt-4.1-mini",
			newClient: func(apiKey string) (unified.Client, error) {
				return openrouterresponses.NewClient(openrouterresponses.WithAPIKey(apiKey))
			},
		},
		{
			name:      "openrouter_messages",
			apiKeyEnv: []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
			modelEnv:  "OPENROUTER_MESSAGES_MODEL",
			model:     "anthropic/claude-sonnet-4",
			tools:     true,
			newClient: func(apiKey string) (unified.Client, error) {
				return openroutermessages.NewClient(openroutermessages.WithAPIKey(apiKey))
			},
		},
	}
}

func newSmokeClient(t *testing.T, provider smokeProvider) (unified.Client, string) {
	t.Helper()
	apiKey := firstSetEnv(provider.apiKeyEnv...)
	if apiKey == "" {
		t.Skipf("set one of %s to run %s e2e smoke test", strings.Join(provider.apiKeyEnv, ","), provider.name)
	}
	model := provider.model
	if fromEnv := os.Getenv(provider.modelEnv); fromEnv != "" {
		model = fromEnv
	}
	client, err := provider.newClient(apiKey)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client, model
}

func firstSetEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func responseText(resp unified.Response) string {
	var b strings.Builder
	for _, part := range resp.Content {
		text, ok := part.(unified.TextPart)
		if !ok {
			continue
		}
		b.WriteString(text.Text)
	}
	return b.String()
}
