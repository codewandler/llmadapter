package e2e

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	"github.com/codewandler/llmadapter/unified"
)

type smokeProvider struct {
	name      string
	apiKeyEnv string
	modelEnv  string
	model     string
	newClient func(apiKey string) (unified.Client, error)
}

func TestSmokeTextStream(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}

	providers := []smokeProvider{
		{
			name:      "anthropic",
			apiKeyEnv: "ANTHROPIC_API_KEY",
			modelEnv:  "ANTHROPIC_MODEL",
			model:     "claude-haiku-4-5-20251001",
			newClient: func(apiKey string) (unified.Client, error) {
				return anthropic.NewClient(anthropic.WithAPIKey(apiKey))
			},
		},
	}

	for _, provider := range providers {
		t.Run(provider.name, func(t *testing.T) {
			apiKey := os.Getenv(provider.apiKeyEnv)
			if apiKey == "" {
				t.Skipf("set %s to run %s e2e smoke test", provider.apiKeyEnv, provider.name)
			}

			model := provider.model
			if fromEnv := os.Getenv(provider.modelEnv); fromEnv != "" {
				model = fromEnv
			}

			client, err := provider.newClient(apiKey)
			if err != nil {
				t.Fatalf("new client: %v", err)
			}

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
