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
	responsesendpoint "github.com/codewandler/llmadapter/endpoints/openairesponses"
	"github.com/codewandler/llmadapter/gateway"
	codex "github.com/codewandler/llmadapter/providers/openai/codex"
	openairesponses "github.com/codewandler/llmadapter/providers/openai/responses"
	openrouterresponses "github.com/codewandler/llmadapter/providers/openrouter/responses"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
)

func TestResponsesGatewaySmokeNonStreaming(t *testing.T) {
	for _, provider := range responsesGatewayProviders() {
		t.Run(provider.name, func(t *testing.T) {
			handler, model := newResponsesGateway(t, provider)
			body := `{
				"model":` + jsonQuote(model) + `,
				"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"Reply with exactly: llmadapter responses gateway smoke ok"}]}],
				"max_output_tokens":64
			}`

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body)))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
			}
			var resp struct {
				Object string `json:"object"`
				Output []struct {
					Type    string `json:"type"`
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				} `json:"output"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}
			if resp.Object != "response" {
				t.Fatalf("unexpected response: %+v", resp)
			}
			if !strings.Contains(strings.ToLower(responsesOutputText(resp.Output)), "llmadapter responses gateway smoke ok") {
				t.Fatalf("unexpected output: %+v", resp.Output)
			}
		})
	}
}

func TestResponsesGatewaySmokeStreaming(t *testing.T) {
	for _, provider := range responsesGatewayProviders() {
		t.Run(provider.name, func(t *testing.T) {
			handler, model := newResponsesGateway(t, provider)
			body := `{
				"model":` + jsonQuote(model) + `,
				"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"Reply with exactly: llmadapter responses gateway stream ok"}]}],
				"max_output_tokens":64,
				"stream":true
			}`

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body)))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
			}
			text, done, err := collectResponsesStreamText(w.Body.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if !done {
				t.Fatalf("stream did not include response.done: %s", w.Body.String())
			}
			if !strings.Contains(strings.ToLower(text), "llmadapter responses gateway stream ok") {
				t.Fatalf("unexpected stream text: %q", text)
			}
		})
	}
}

type responsesGatewayProvider struct {
	name            string
	apiKind         adapt.ApiKind
	apiKeyEnv       []string
	localCodexOAuth bool
	modelEnv        string
	model           string
	newClient       func(apiKey string) (unified.Client, error)
}

func responsesGatewayProviders() []responsesGatewayProvider {
	return []responsesGatewayProvider{
		{
			name:      "openai_responses",
			apiKind:   adapt.ApiOpenAIResponses,
			apiKeyEnv: []string{"OPENAI_API_KEY", "OPENAI_KEY"},
			modelEnv:  "OPENAI_RESPONSES_MODEL",
			model:     "gpt-4.1-mini",
			newClient: func(apiKey string) (unified.Client, error) {
				return openairesponses.NewClient(openairesponses.WithAPIKey(apiKey))
			},
		},
		{
			name:            "codex_responses",
			apiKind:         adapt.ApiCodexResponses,
			apiKeyEnv:       []string{codex.EnvAccessToken, codex.EnvOAuthToken},
			localCodexOAuth: true,
			modelEnv:        codex.EnvModel,
			model:           codex.DefaultModel,
			newClient: func(apiKey string) (unified.Client, error) {
				if apiKey == "" {
					return codex.NewClient()
				}
				return codex.NewClient(codex.WithAccessToken(apiKey))
			},
		},
		{
			name:      "openrouter_responses",
			apiKind:   adapt.ApiOpenRouterResponses,
			apiKeyEnv: []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
			modelEnv:  "OPENROUTER_RESPONSES_MODEL",
			model:     "openai/gpt-4.1-mini",
			newClient: func(apiKey string) (unified.Client, error) {
				return openrouterresponses.NewClient(openrouterresponses.WithAPIKey(apiKey))
			},
		},
	}
}

func newResponsesGateway(t *testing.T, provider responsesGatewayProvider) (http.Handler, string) {
	t.Helper()
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}
	apiKey := firstSetEnv(provider.apiKeyEnv...)
	if apiKey == "" && !(provider.localCodexOAuth && codex.LocalAvailable()) {
		t.Skipf("set one of %s to run %s Responses gateway e2e smoke tests", strings.Join(provider.apiKeyEnv, ","), provider.name)
	}
	model := provider.model
	if fromEnv := os.Getenv(provider.modelEnv); fromEnv != "" {
		model = fromEnv
	}
	client, err := provider.newClient(apiKey)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return timeoutHandler{
		ctx: ctx,
		handler: gateway.Handler{
			Endpoint: responsesendpoint.Codec{},
			Router: router.NewStaticRouter(router.StaticRoute{
				SourceAPI: adapt.ApiOpenAIResponses,
				Endpoint: router.ProviderEndpoint{
					ProviderName: provider.name,
					APIKind:      provider.apiKind,
					Family:       adapt.FamilyOpenAIResponses,
					Client:       client,
					Capabilities: router.CapabilitySet{Streaming: true, Tools: true, Reasoning: true},
				},
			}),
		},
	}, model
}

func responsesOutputText(output []struct {
	Type    string `json:"type"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}) string {
	var out strings.Builder
	for _, item := range output {
		for _, part := range item.Content {
			if part.Type == "output_text" {
				out.WriteString(part.Text)
			}
		}
	}
	return out.String()
}

func collectResponsesStreamText(body []byte) (string, bool, error) {
	var text strings.Builder
	done := false
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event struct {
			Type  string `json:"type"`
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			return "", false, err
		}
		if event.Type == "response.done" {
			done = true
		}
		if event.Type == "response.output_text.delta" {
			text.WriteString(event.Delta)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", false, err
	}
	return text.String(), done, nil
}
