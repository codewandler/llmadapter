package e2e

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	minimax "github.com/codewandler/llmadapter/providers/minimax/chatcompletions"
	minimaxmessages "github.com/codewandler/llmadapter/providers/minimax/messages"
	openai "github.com/codewandler/llmadapter/providers/openai/chatcompletions"
	codex "github.com/codewandler/llmadapter/providers/openai/codex"
	openairesponses "github.com/codewandler/llmadapter/providers/openai/responses"
	openrouter "github.com/codewandler/llmadapter/providers/openrouter/chatcompletions"
	openroutermessages "github.com/codewandler/llmadapter/providers/openrouter/messages"
	openrouterresponses "github.com/codewandler/llmadapter/providers/openrouter/responses"
	"github.com/codewandler/llmadapter/unified"
)

type smokeProvider struct {
	name                  string
	apiKeyEnv             []string
	localClaudeOAuth      bool
	localCodexOAuth       bool
	modelEnv              string
	model                 string
	tools                 bool
	reasoning             bool
	promptCache           bool
	cacheWarmupNoUsage    bool
	cachePrefixRepeat     int
	responsesContinuation bool
	maxOutputTokens       int
	newClient             func(apiKey string) (unified.Client, error)
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

			maxTokens := provider.maxTokens(64)
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
			if resp.Usage.TotalTokens() == 0 && resp.Usage.InputTokens() == 0 && resp.Usage.OutputTokens() == 0 {
				t.Fatalf("missing usage in response: %+v", resp)
			}
		})
	}
}

func TestSmokeReasoningStream(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}

	for _, provider := range smokeProviders() {
		t.Run(provider.name, func(t *testing.T) {
			if !provider.reasoning {
				t.Skipf("%s does not advertise reasoning smoke support in this slice", provider.name)
			}
			client, model := newSmokeClient(t, provider)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			maxTokens := provider.maxTokens(2048)
			if maxTokens < 2048 {
				maxTokens = 2048
			}
			budget := 1024
			events, err := client.Request(ctx, unified.Request{
				Model:           model,
				MaxOutputTokens: &maxTokens,
				Reasoning:       &unified.ReasoningConfig{Effort: unified.ReasoningEffortHigh, MaxTokens: &budget, Expose: true},
				Messages: []unified.Message{{
					Role: unified.RoleUser,
					Content: []unified.ContentPart{
						unified.TextPart{Text: "What is 1+1? After thinking, answer with exactly: 2"},
					},
				}},
				Stream: true,
			})
			if err != nil {
				t.Fatalf("request: %v", err)
			}

			resp, err := unified.Collect(ctx, events)
			if err != nil {
				t.Fatalf("collect: %v", err)
			}
			if !strings.Contains(responseText(resp), "2") {
				t.Fatalf("response text %q does not contain expected answer; response=%+v", responseText(resp), resp)
			}
			if responseReasoningText(resp) == "" && resp.Usage.ReasoningTokens() == 0 {
				t.Fatalf("missing reasoning evidence: response=%+v usage=%+v", resp, resp.Usage)
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

			maxTokens := provider.maxTokens(128)
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

			maxTokens := provider.maxTokens(128)
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

func TestSmokePromptCache(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}

	for _, provider := range smokeProviders() {
		t.Run(provider.name, func(t *testing.T) {
			if !provider.promptCache {
				t.Skipf("%s does not advertise prompt cache smoke support in this slice", provider.name)
			}
			client, model := newSmokeClient(t, provider)
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()

			maxTokens := provider.maxTokens(16)
			req := unified.Request{
				Model:           model,
				MaxOutputTokens: &maxTokens,
				CachePolicy:     unified.CachePolicyOn,
				CacheKey:        cacheSmokeKey(provider.name),
				Instructions: []unified.Instruction{{
					Kind: unified.InstructionSystem,
					Content: []unified.ContentPart{unified.TextPart{
						Text:         cacheSmokePrefix(provider.cachePrefixRepeat),
						CacheControl: unified.EphemeralCache(""),
					}},
				}},
				Messages: []unified.Message{{
					Role:    unified.RoleUser,
					Content: []unified.ContentPart{unified.TextPart{Text: "Reply with exactly: cache smoke ok"}},
				}},
				Stream: true,
			}

			first, err := collectSmokeResponse(ctx, client, req)
			if err != nil {
				t.Fatalf("first request: %v", err)
			}
			if first.Usage.CacheWriteTokens() == 0 && first.Usage.CacheReadTokens() == 0 {
				if !provider.cacheWarmupNoUsage {
					t.Fatalf("first request did not report cache usage: %+v", first.Usage)
				}
			}
			if first.Usage.CacheReadTokens() > 0 {
				return
			}
			if first.Usage.CacheWriteTokens() == 0 && provider.cacheWarmupNoUsage {
				t.Logf("first request did not report cache write usage; checking follow-up cache reads: %+v", first.Usage)
			}

			var last unified.Response
			for attempt := 0; attempt < 4; attempt++ {
				time.Sleep(time.Duration(attempt+1) * time.Second)
				last, err = collectSmokeResponse(ctx, client, req)
				if err != nil {
					t.Fatalf("cache read attempt %d: %v", attempt+1, err)
				}
				if last.Usage.CacheReadTokens() > 0 {
					return
				}
			}
			t.Fatalf("repeated requests did not report cache read usage; first=%+v last=%+v", first.Usage, last.Usage)
		})
	}
}

func TestSmokeResponsesContinuation(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}

	for _, provider := range smokeProviders() {
		t.Run(provider.name, func(t *testing.T) {
			if !provider.responsesContinuation {
				t.Skipf("%s does not advertise Responses previous_response_id smoke support in this slice", provider.name)
			}
			client, model := newSmokeClient(t, provider)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			maxTokens := provider.maxTokens(32)
			firstReq := unified.Request{
				Model:           model,
				MaxOutputTokens: &maxTokens,
				Messages: []unified.Message{{
					Role: unified.RoleUser,
					Content: []unified.ContentPart{
						unified.TextPart{Text: "Remember the marker orchid-731. Reply with exactly: remembered"},
					},
				}},
				Stream: true,
			}
			if err := firstReq.Extensions.Set(unified.ExtOpenAIStore, true); err != nil {
				t.Fatal(err)
			}

			first, err := collectSmokeResponse(ctx, client, firstReq)
			if err != nil {
				t.Fatalf("first request: %v", err)
			}
			if first.ID == "" {
				t.Fatalf("first response did not expose a response id: %+v", first)
			}
			if !strings.Contains(strings.ToLower(responseText(first)), "remembered") {
				t.Fatalf("first response text = %q", responseText(first))
			}

			secondReq := unified.Request{
				Model:           model,
				MaxOutputTokens: &maxTokens,
				Messages: []unified.Message{{
					Role: unified.RoleUser,
					Content: []unified.ContentPart{
						unified.TextPart{Text: "What marker did I ask you to remember? Reply with only the marker."},
					},
				}},
				Stream: true,
			}
			if err := secondReq.Extensions.Set(unified.ExtOpenAIPreviousResponseID, first.ID); err != nil {
				t.Fatal(err)
			}

			second, err := collectSmokeResponse(ctx, client, secondReq)
			if err != nil {
				t.Fatalf("continuation request: %v", err)
			}
			if !strings.Contains(strings.ToLower(responseText(second)), "orchid-731") {
				t.Fatalf("continuation text %q does not contain remembered marker; first id=%s response=%+v", responseText(second), first.ID, second)
			}
			if second.ID == "" {
				t.Fatalf("continuation response did not expose a response id: %+v", second)
			}
		})
	}
}

func smokeProviders() []smokeProvider {
	return []smokeProvider{
		{
			name:        "anthropic",
			apiKeyEnv:   []string{"ANTHROPIC_API_KEY"},
			modelEnv:    "ANTHROPIC_MODEL",
			model:       "claude-haiku-4-5-20251001",
			tools:       true,
			reasoning:   true,
			promptCache: true,
			newClient: func(apiKey string) (unified.Client, error) {
				return anthropic.NewClient(anthropic.WithAPIKey(apiKey))
			},
		},
		{
			name:             "claude",
			apiKeyEnv:        nil,
			localClaudeOAuth: true,
			modelEnv:         "CLAUDE_MODEL",
			model:            "claude-haiku-4-5-20251001",
			tools:            true,
			reasoning:        true,
			promptCache:      true,
			newClient:        newClaudeSmokeClient,
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
			name:                  "openai_responses",
			apiKeyEnv:             []string{"OPENAI_API_KEY", "OPENAI_KEY"},
			modelEnv:              "OPENAI_RESPONSES_MODEL",
			model:                 "gpt-4.1-mini",
			tools:                 true,
			responsesContinuation: true,
			newClient: func(apiKey string) (unified.Client, error) {
				return openairesponses.NewClient(openairesponses.WithAPIKey(apiKey))
			},
		},
		{
			name:               "codex_responses",
			apiKeyEnv:          []string{codex.EnvAccessToken, codex.EnvOAuthToken},
			localCodexOAuth:    true,
			modelEnv:           codex.EnvModel,
			model:              codex.DefaultModel,
			tools:              true,
			reasoning:          true,
			promptCache:        true,
			cacheWarmupNoUsage: true,
			cachePrefixRepeat:  1200,
			newClient:          newCodexSmokeClient,
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
			name:      "minimax_chat",
			apiKeyEnv: []string{"MINIMAX_API_KEY", "MINIMAX_KEY"},
			modelEnv:  "MINIMAX_MODEL",
			model:     "MiniMax-M2.7",
			tools:     true,
			newClient: func(apiKey string) (unified.Client, error) {
				return minimax.NewClient(minimax.WithAPIKey(apiKey))
			},
		},
		{
			name:        "minimax_messages",
			apiKeyEnv:   []string{"MINIMAX_API_KEY", "MINIMAX_KEY"},
			modelEnv:    "MINIMAX_MESSAGES_MODEL",
			model:       "MiniMax-M2.7",
			tools:       true,
			reasoning:   true,
			promptCache: true,
			// MiniMax emits reasoning before final text on the Anthropic-compatible surface.
			maxOutputTokens: 512,
			newClient: func(apiKey string) (unified.Client, error) {
				return minimaxmessages.NewClient(minimaxmessages.WithAPIKey(apiKey))
			},
		},
		{
			name:      "openrouter_responses",
			apiKeyEnv: []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
			modelEnv:  "OPENROUTER_RESPONSES_MODEL",
			model:     "openai/gpt-4.1-mini",
			tools:     true,
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
			reasoning: true,
			newClient: func(apiKey string) (unified.Client, error) {
				return openroutermessages.NewClient(openroutermessages.WithAPIKey(apiKey))
			},
		},
	}
}

func collectSmokeResponse(ctx context.Context, client unified.Client, req unified.Request) (unified.Response, error) {
	events, err := client.Request(ctx, req)
	if err != nil {
		return unified.Response{}, err
	}
	return unified.Collect(ctx, events)
}

func cacheSmokePrefix(repeat int) string {
	if repeat <= 0 {
		repeat = 320
	}
	const sentence = "This stable llmadapter prompt-cache smoke prefix is intentionally repetitive so Anthropic can store it as reusable context. "
	var b strings.Builder
	for i := 0; i < repeat; i++ {
		b.WriteString(sentence)
	}
	return b.String()
}

func cacheSmokeKey(providerName string) string {
	replacer := strings.NewReplacer("_", "-", ".", "-")
	return "llmadapter-smoke-" + replacer.Replace(strings.ToLower(providerName))
}

func (p smokeProvider) maxTokens(defaultValue int) int {
	if p.maxOutputTokens > 0 {
		return p.maxOutputTokens
	}
	return defaultValue
}

func newSmokeClient(t *testing.T, provider smokeProvider) (unified.Client, string) {
	t.Helper()
	apiKey := firstSetEnv(provider.apiKeyEnv...)
	if apiKey == "" && !(provider.localClaudeOAuth && anthropic.LocalTokenStoreAvailable()) && !(provider.localCodexOAuth && codex.LocalAvailable()) {
		if len(provider.apiKeyEnv) == 0 && provider.localClaudeOAuth {
			t.Skipf("Claude provider %s requires local Claude credentials, expected ~/.claude/.credentials.json or CLAUDE_CONFIG_DIR to point to a token file", provider.name)
		}
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

func newCodexSmokeClient(token string) (unified.Client, error) {
	if token == "" {
		return codex.NewClient()
	}
	return codex.NewClient(codex.WithAccessToken(token))
}

func newClaudeSmokeClient(token string) (unified.Client, error) {
	if token == "" {
		return anthropic.NewClient(anthropic.WithClaudeCode())
	}
	return anthropic.NewClient(
		anthropic.WithBearerTokenProvider(anthropic.NewStaticTokenProvider(anthropic.NewStaticBearerToken(token))),
		anthropic.WithClaudeHeaders(),
		anthropic.WithClaudeCodePreflight(),
		anthropic.WithSystemCacheControl(""),
	)
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

func responseReasoningText(resp unified.Response) string {
	var b strings.Builder
	for _, part := range resp.Content {
		text, ok := part.(unified.ReasoningPart)
		if !ok {
			continue
		}
		b.WriteString(text.Text)
	}
	return b.String()
}
