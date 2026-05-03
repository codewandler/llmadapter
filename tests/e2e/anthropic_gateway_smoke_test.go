package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/codewandler/llmadapter/adapt"
	anthropicendpoint "github.com/codewandler/llmadapter/endpoints/anthropicmessages"
	"github.com/codewandler/llmadapter/gateway"
	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	bedrockconverse "github.com/codewandler/llmadapter/providers/bedrock/converse"
	bedrockmessages "github.com/codewandler/llmadapter/providers/bedrock/messages"
	minimaxmessages "github.com/codewandler/llmadapter/providers/minimax/messages"
	openroutermessages "github.com/codewandler/llmadapter/providers/openrouter/messages"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
)

func TestAnthropicMessagesGatewaySmokeNonStreaming(t *testing.T) {
	for _, provider := range anthropicMessagesGatewayProviders() {
		t.Run(provider.name, func(t *testing.T) {
			handler, model, maxTokens := newAnthropicMessagesGateway(t, provider)
			body := `{
				"model":` + jsonQuote(model) + `,
				"messages":[{"role":"user","content":[{"type":"text","text":"Reply with exactly: llmadapter messages gateway smoke ok"}]}],
				"max_tokens":` + jsonInt(maxTokens) + `
			}`

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
			}
			var resp struct {
				Type    string `json:"type"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}
			if resp.Type != "message" {
				t.Fatalf("unexpected response: %+v", resp)
			}
			if !strings.Contains(strings.ToLower(anthropicContentText(resp.Content)), "llmadapter messages gateway smoke ok") {
				t.Fatalf("unexpected content: %+v", resp.Content)
			}
		})
	}
}

func TestAnthropicMessagesGatewaySmokeStreaming(t *testing.T) {
	for _, provider := range anthropicMessagesGatewayProviders() {
		t.Run(provider.name, func(t *testing.T) {
			handler, model, maxTokens := newAnthropicMessagesGateway(t, provider)
			body := `{
				"model":` + jsonQuote(model) + `,
				"messages":[{"role":"user","content":[{"type":"text","text":"Reply with exactly: llmadapter messages gateway stream ok"}]}],
				"max_tokens":` + jsonInt(maxTokens) + `,
				"stream":true
			}`

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
			}
			text, stopped, err := collectAnthropicStreamText(w.Body.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if !stopped {
				t.Fatalf("stream did not include message_stop: %s", w.Body.String())
			}
			if !strings.Contains(strings.ToLower(text), "llmadapter messages gateway stream ok") {
				t.Fatalf("unexpected stream text: %q", text)
			}
		})
	}
}

func anthropicMessagesGatewayProviders() []gatewayProvider {
	return []gatewayProvider{
		{
			name:         "anthropic",
			apiKind:      adapt.ApiAnthropicMessages,
			family:       adapt.FamilyAnthropicMessages,
			capabilities: router.CapabilitySet{Streaming: true, Tools: true},
			apiKeyEnv:    []string{"ANTHROPIC_API_KEY"},
			modelEnv:     "ANTHROPIC_MODEL",
			model:        "claude-haiku-4-5-20251001",
			newClient: func(apiKey string) (unified.Client, error) {
				return anthropic.NewClient(
					anthropic.WithAPIKey(apiKey),
					anthropic.WithRequestProcessor(requestProcessorFunc(func(ctx context.Context, req *adapt.Request) error {
						req.Unified.Stream = true
						return nil
					})),
				)
			},
		},
		{
			name:         "openrouter_messages",
			apiKind:      adapt.ApiOpenRouterAnthropicMessages,
			family:       adapt.FamilyAnthropicMessages,
			capabilities: router.CapabilitySet{Streaming: true, Tools: true},
			apiKeyEnv:    []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
			modelEnv:     "OPENROUTER_MESSAGES_MODEL",
			model:        "anthropic/claude-sonnet-4",
			newClient: func(apiKey string) (unified.Client, error) {
				return openroutermessages.NewClient(openroutermessages.WithAPIKey(apiKey))
			},
		},
		{
			name:            "minimax_messages",
			apiKind:         adapt.ApiMiniMaxAnthropicMessages,
			family:          adapt.FamilyAnthropicMessages,
			capabilities:    router.CapabilitySet{Streaming: true, Tools: true},
			apiKeyEnv:       []string{"MINIMAX_API_KEY", "MINIMAX_KEY"},
			modelEnv:        "MINIMAX_MESSAGES_MODEL",
			model:           "MiniMax-M2.7",
			maxOutputTokens: 512,
			newClient: func(apiKey string) (unified.Client, error) {
				return minimaxmessages.NewClient(minimaxmessages.WithAPIKey(apiKey))
			},
		},
		{
			name:           "bedrock_messages",
			apiKind:        adapt.ApiBedrockAnthropicMessages,
			family:         adapt.FamilyAnthropicMessages,
			capabilities:   router.CapabilitySet{Streaming: true, Tools: true},
			apiKeyEnv:      []string{bedrockmessages.EnvAPIKey, bedrockmessages.EnvBearerToken},
			awsProfileAuth: true,
			modelEnv:       bedrockmessages.EnvModel,
			model:          bedrockmessages.DefaultModel,
			newClient: func(apiKey string) (unified.Client, error) {
				if apiKey == "" {
					return bedrockmessages.NewClient()
				}
				return bedrockmessages.NewClient(bedrockmessages.WithAPIKey(apiKey))
			},
		},
		{
			name:            "bedrock_converse",
			apiKind:         adapt.ApiBedrockConverse,
			family:          adapt.FamilyBedrockConverse,
			capabilities:    router.CapabilitySet{Streaming: true, Tools: true, Reasoning: true},
			awsProfileAuth:  true,
			modelEnv:        bedrockconverse.EnvModel,
			model:           bedrockconverse.DefaultModel,
			maxOutputTokens: 512,
			newClient: func(apiKey string) (unified.Client, error) {
				return bedrockconverse.NewClient()
			},
		},
	}
}

func newAnthropicMessagesGateway(t *testing.T, provider gatewayProvider) (http.Handler, string, int) {
	t.Helper()
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}
	apiKey := firstSetEnv(provider.apiKeyEnv...)
	if apiKey == "" && !provider.awsProfileAuth {
		t.Skipf("set one of %s to run %s Anthropic Messages gateway e2e smoke tests", strings.Join(provider.apiKeyEnv, ","), provider.name)
	}
	model := provider.model
	if fromEnv := os.Getenv(provider.modelEnv); fromEnv != "" {
		model = fromEnv
	}
	client, err := provider.newClient(apiKey)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	return timeoutHandler{
		ctx: ctx,
		handler: gateway.Handler{
			Endpoint: anthropicendpoint.Codec{},
			Router: router.NewStaticRouter(router.StaticRoute{
				SourceAPI: adapt.ApiAnthropicMessages,
				Endpoint: router.ProviderEndpoint{
					ProviderName: provider.name,
					APIKind:      provider.apiKind,
					Family:       provider.family,
					Client:       client,
					Capabilities: provider.capabilities,
				},
			}),
		},
	}, model, provider.maxTokens(128)
}

func anthropicContentText(content []struct {
	Type string `json:"type"`
	Text string `json:"text"`
}) string {
	var out strings.Builder
	for _, part := range content {
		if part.Type == "text" {
			out.WriteString(part.Text)
		}
	}
	return out.String()
}

func collectAnthropicStreamText(body []byte) (string, bool, error) {
	var text strings.Builder
	stopped := false
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			return "", false, err
		}
		if event.Type == "message_stop" {
			stopped = true
		}
		text.WriteString(event.Delta.Text)
	}
	if err := scanner.Err(); err != nil {
		return "", false, err
	}
	return text.String(), stopped, nil
}
