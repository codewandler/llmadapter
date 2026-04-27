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
	chat "github.com/codewandler/llmadapter/endpoints/openaichatcompletions"
	"github.com/codewandler/llmadapter/gateway"
	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	minimax "github.com/codewandler/llmadapter/providers/minimax/chatcompletions"
	minimaxmessages "github.com/codewandler/llmadapter/providers/minimax/messages"
	openai "github.com/codewandler/llmadapter/providers/openai/chatcompletions"
	codex "github.com/codewandler/llmadapter/providers/openai/codex"
	openairesponses "github.com/codewandler/llmadapter/providers/openai/responses"
	openrouter "github.com/codewandler/llmadapter/providers/openrouter/chatcompletions"
	openroutermessages "github.com/codewandler/llmadapter/providers/openrouter/messages"
	openrouterresponses "github.com/codewandler/llmadapter/providers/openrouter/responses"
	"github.com/codewandler/llmadapter/router"
	"github.com/codewandler/llmadapter/unified"
)

type gatewayProvider struct {
	name             string
	apiKind          adapt.ApiKind
	family           adapt.ApiFamily
	capabilities     router.CapabilitySet
	apiKeyEnv        []string
	localClaudeOAuth bool
	localCodexOAuth  bool
	modelEnv         string
	model            string
	maxOutputTokens  int
	newClient        func(apiKey string) (unified.Client, error)
}

func TestGatewaySmokeNonStreaming(t *testing.T) {
	for _, provider := range gatewayProviders() {
		t.Run(provider.name, func(t *testing.T) {
			t.Parallel()
			handler, model := newGateway(t, provider)
			body := `{
				"model":` + jsonQuote(model) + `,
				"messages":[{"role":"user","content":"Reply with exactly: llmadapter gateway smoke ok"}],
				"max_tokens":` + jsonInt(provider.maxTokens(64)) + `
			}`

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
			}
			var resp struct {
				Object  string `json:"object"`
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}
			if resp.Object != "chat.completion" || len(resp.Choices) != 1 {
				t.Fatalf("unexpected response: %+v", resp)
			}
			if !strings.Contains(strings.ToLower(resp.Choices[0].Message.Content), "llmadapter gateway smoke ok") {
				t.Fatalf("unexpected content: %q", resp.Choices[0].Message.Content)
			}
		})
	}
}

func TestGatewaySmokeStreaming(t *testing.T) {
	for _, provider := range gatewayProviders() {
		t.Run(provider.name, func(t *testing.T) {
			t.Parallel()
			handler, model := newGateway(t, provider)
			body := `{
				"model":` + jsonQuote(model) + `,
				"messages":[{"role":"user","content":"Reply with exactly: llmadapter gateway stream ok"}],
				"max_tokens":` + jsonInt(provider.maxTokens(64)) + `,
				"stream":true
			}`

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body)))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
			}
			text, done, err := collectOpenAIStreamText(w.Body.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if !done {
				t.Fatalf("stream did not include [DONE]: %s", w.Body.String())
			}
			if !strings.Contains(strings.ToLower(text), "llmadapter gateway stream ok") {
				t.Fatalf("unexpected stream text: %q", text)
			}
		})
	}
}

func gatewayProviders() []gatewayProvider {
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
			name:             "claude",
			apiKind:          adapt.ApiAnthropicMessages,
			family:           adapt.FamilyAnthropicMessages,
			capabilities:     router.CapabilitySet{Streaming: true, Tools: true},
			apiKeyEnv:        nil,
			localClaudeOAuth: true,
			modelEnv:         "CLAUDE_MODEL",
			model:            "claude-haiku-4-5-20251001",
			newClient: func(apiKey string) (unified.Client, error) {
				if apiKey == "" {
					return anthropic.NewClient(
						anthropic.WithClaudeCode(),
						anthropic.WithRequestProcessor(requestProcessorFunc(func(ctx context.Context, req *adapt.Request) error {
							req.Unified.Stream = true
							return nil
						})),
					)
				}
				return anthropic.NewClient(
					anthropic.WithBearerTokenProvider(anthropic.NewStaticTokenProvider(anthropic.NewStaticBearerToken(apiKey))),
					anthropic.WithClaudeHeaders(),
					anthropic.WithClaudeCodePreflight(),
					anthropic.WithSystemCacheControl("1h"),
					anthropic.WithRequestProcessor(requestProcessorFunc(func(ctx context.Context, req *adapt.Request) error {
						req.Unified.Stream = true
						return nil
					})),
				)
			},
		},
		{
			name:         "openai_chat",
			apiKind:      adapt.ApiOpenAIChatCompletions,
			family:       adapt.FamilyOpenAIChatCompletions,
			capabilities: router.CapabilitySet{Streaming: true, Tools: true},
			apiKeyEnv:    []string{"OPENAI_API_KEY", "OPENAI_KEY"},
			modelEnv:     "OPENAI_MODEL",
			model:        "gpt-4.1-mini",
			newClient: func(apiKey string) (unified.Client, error) {
				return openai.NewClient(openai.WithAPIKey(apiKey))
			},
		},
		{
			name:         "openai_responses",
			apiKind:      adapt.ApiOpenAIResponses,
			family:       adapt.FamilyOpenAIResponses,
			capabilities: router.CapabilitySet{Streaming: true, Tools: true},
			apiKeyEnv:    []string{"OPENAI_API_KEY", "OPENAI_KEY"},
			modelEnv:     "OPENAI_RESPONSES_MODEL",
			model:        "gpt-4.1-mini",
			newClient: func(apiKey string) (unified.Client, error) {
				return openairesponses.NewClient(openairesponses.WithAPIKey(apiKey))
			},
		},
		{
			name:            "codex_responses",
			apiKind:         adapt.ApiCodexResponses,
			family:          adapt.FamilyOpenAIResponses,
			capabilities:    router.CapabilitySet{Streaming: true, Tools: true, Reasoning: true, PromptCaching: true},
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
			name:         "openrouter_chat",
			apiKind:      adapt.ApiOpenRouterChatCompletions,
			family:       adapt.FamilyOpenAIChatCompletions,
			capabilities: router.CapabilitySet{Streaming: true, Tools: true},
			apiKeyEnv:    []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
			modelEnv:     "OPENROUTER_MODEL",
			model:        "openai/gpt-4.1-mini",
			newClient: func(apiKey string) (unified.Client, error) {
				return openrouter.NewClient(openrouter.WithAPIKey(apiKey))
			},
		},
		{
			name:         "minimax_chat",
			apiKind:      adapt.ApiMiniMaxChatCompletions,
			family:       adapt.FamilyOpenAIChatCompletions,
			capabilities: router.CapabilitySet{Streaming: true},
			apiKeyEnv:    []string{"MINIMAX_API_KEY", "MINIMAX_KEY"},
			modelEnv:     "MINIMAX_MODEL",
			model:        "MiniMax-M2.7",
			newClient: func(apiKey string) (unified.Client, error) {
				return minimax.NewClient(minimax.WithAPIKey(apiKey))
			},
		},
		{
			name:         "minimax_messages",
			apiKind:      adapt.ApiMiniMaxAnthropicMessages,
			family:       adapt.FamilyAnthropicMessages,
			capabilities: router.CapabilitySet{Streaming: true, Tools: true},
			apiKeyEnv:    []string{"MINIMAX_API_KEY", "MINIMAX_KEY"},
			modelEnv:     "MINIMAX_MESSAGES_MODEL",
			model:        "MiniMax-M2.7",
			// MiniMax emits reasoning before final text on the Anthropic-compatible surface.
			maxOutputTokens: 512,
			newClient: func(apiKey string) (unified.Client, error) {
				return minimaxmessages.NewClient(minimaxmessages.WithAPIKey(apiKey))
			},
		},
		{
			name:         "openrouter_responses",
			apiKind:      adapt.ApiOpenRouterResponses,
			family:       adapt.FamilyOpenAIResponses,
			capabilities: router.CapabilitySet{Streaming: true, Tools: true},
			apiKeyEnv:    []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
			modelEnv:     "OPENROUTER_RESPONSES_MODEL",
			model:        "openai/gpt-4.1-mini",
			newClient: func(apiKey string) (unified.Client, error) {
				return openrouterresponses.NewClient(openrouterresponses.WithAPIKey(apiKey))
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
	}
}

func (p gatewayProvider) maxTokens(defaultValue int) int {
	if p.maxOutputTokens > 0 {
		return p.maxOutputTokens
	}
	return defaultValue
}

func newGateway(t *testing.T, provider gatewayProvider) (http.Handler, string) {
	t.Helper()
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}
	apiKey := firstSetEnv(provider.apiKeyEnv...)
	if apiKey == "" && !(provider.localClaudeOAuth && anthropic.LocalTokenStoreAvailable()) && !(provider.localCodexOAuth && codex.LocalAvailable()) {
		if len(provider.apiKeyEnv) == 0 && provider.localClaudeOAuth {
			t.Skipf("Claude provider %s requires local Claude credentials, expected ~/.claude/.credentials.json or CLAUDE_CONFIG_DIR to point to a token file", provider.name)
		}
		t.Skipf("set one of %s to run %s gateway e2e smoke tests", strings.Join(provider.apiKeyEnv, ","), provider.name)
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
			Endpoint: chat.Codec{},
			Router: router.NewStaticRouter(router.StaticRoute{
				SourceAPI: adapt.ApiOpenAIChatCompletions,
				Endpoint: router.ProviderEndpoint{
					ProviderName: provider.name,
					APIKind:      provider.apiKind,
					Family:       provider.family,
					Client:       client,
					Capabilities: provider.capabilities,
				},
			}),
		},
	}, model
}

type timeoutHandler struct {
	ctx     context.Context
	handler http.Handler
}

func (h timeoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handler.ServeHTTP(w, r.WithContext(h.ctx))
}

func collectOpenAIStreamText(body []byte) (string, bool, error) {
	var text strings.Builder
	done := false
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			done = true
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return "", false, err
		}
		for _, choice := range chunk.Choices {
			text.WriteString(choice.Delta.Content)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", false, err
	}
	return text.String(), done, nil
}

func jsonQuote(value string) string {
	b, _ := json.Marshal(value)
	return string(b)
}

func jsonInt(value int) string {
	b, _ := json.Marshal(value)
	return string(b)
}

type requestProcessorFunc func(context.Context, *adapt.Request) error

func (f requestProcessorFunc) ProcessRequest(ctx context.Context, req *adapt.Request) error {
	return f(ctx, req)
}
