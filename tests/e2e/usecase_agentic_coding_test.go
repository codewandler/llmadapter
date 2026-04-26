package e2e

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/adapterconfig"
	"github.com/codewandler/llmadapter/providers/anthropic/messages"
	codex "github.com/codewandler/llmadapter/providers/openai/codex"
	"github.com/codewandler/llmadapter/unified"
)

type agenticCodingCandidate struct {
	name                  string
	publicModel           string
	nativeModel           string
	modelEnv              string
	providerType          string
	providerAPI           adapt.ApiKind
	sourceAPI             adapt.ApiKind
	modelDBService        string
	apiKeyEnv             []string
	localClaudeOAuth      bool
	localCodexOAuth       bool
	maxOutputTokens       int
	cacheWarmupNoUsage    bool
	cachePrefixRepeat     int
	responseFormatSchema  bool
	skipReasoningText     bool
	skipCacheAccountingIf string
}

func TestUseCaseAgenticCoding(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run use-case compatibility e2e tests")
	}

	for _, candidate := range agenticCodingCandidates() {
		t.Run(candidate.name, func(t *testing.T) {
			started := time.Now()
			client, model := newAgenticCodingClient(t, candidate)
			result := agenticCodingResult{Candidate: candidate.name, Model: model}

			result.Text = checkAgenticText(t, client, model, candidate)
			result.Tools = checkAgenticToolContinuation(t, client, model, candidate)
			result.StructuredOutput = checkAgenticStructuredOutput(t, client, model, candidate)
			result.Reasoning = checkAgenticReasoning(t, client, model, candidate)
			result.PromptCaching, result.CacheAccounting = checkAgenticPromptCache(t, client, model, candidate)
			result.DurationSeconds = time.Since(started).Seconds()

			t.Logf("agentic_coding result: candidate=%s model=%s duration_seconds=%.2f text=%s tools=%s structured_output=%s reasoning=%s prompt_caching=%s cache_accounting=%s",
				result.Candidate,
				result.Model,
				result.DurationSeconds,
				result.Text,
				result.Tools,
				result.StructuredOutput,
				result.Reasoning,
				result.PromptCaching,
				result.CacheAccounting,
			)
		})
	}
}

type agenticCodingResult struct {
	Candidate        string
	Model            string
	Text             string
	Tools            string
	StructuredOutput string
	Reasoning        string
	PromptCaching    string
	CacheAccounting  string
	DurationSeconds  float64
}

func agenticCodingCandidates() []agenticCodingCandidate {
	return []agenticCodingCandidate{
		openAIResponsesAgenticCandidate("openai_gpt_5_5", "gpt-5.5", "OPENAI_AGENTIC_GPT55_MODEL"),
		codexAgenticCandidate("codex_gpt_5_5", "gpt-5.5", "CODEX_AGENTIC_GPT55_MODEL"),
		openRouterResponsesAgenticCandidate("openrouter_gpt_5_5", "gpt-5.5", "openai/gpt-5.5", "OPENROUTER_AGENTIC_GPT55_MODEL"),
		openAIResponsesAgenticCandidate("openai_gpt_5_4", "gpt-5.4", "OPENAI_AGENTIC_GPT54_MODEL"),
		codexAgenticCandidate("codex_gpt_5_4", "gpt-5.4", "CODEX_AGENTIC_GPT54_MODEL"),
		openRouterResponsesAgenticCandidate("openrouter_gpt_5_4", "gpt-5.4", "openai/gpt-5.4", "OPENROUTER_AGENTIC_GPT54_MODEL"),
		openRouterResponsesAgenticCandidate("openrouter_kimi_k2_6", "kimi-k2.6", "moonshotai/kimi-k2.6", "OPENROUTER_AGENTIC_KIMI_MODEL"),
		claudeFamilyAgenticCandidate("claude_haiku", "haiku", "claude-haiku-4-5-20251001", "CLAUDE_AGENTIC_HAIKU_MODEL", "claude", nil, true),
		anthropicAgenticCandidate("anthropic_haiku", "haiku", "claude-haiku-4-5-20251001", "ANTHROPIC_AGENTIC_HAIKU_MODEL"),
		openRouterMessagesAgenticCandidate("openrouter_haiku", "haiku", "anthropic/claude-haiku-4.5", "OPENROUTER_AGENTIC_HAIKU_MODEL"),
		claudeFamilyAgenticCandidate("claude_sonnet", "sonnet", "claude-sonnet-4-6", "CLAUDE_AGENTIC_SONNET_MODEL", "claude", nil, true),
		anthropicAgenticCandidate("anthropic_sonnet", "sonnet", "claude-sonnet-4-6", "ANTHROPIC_AGENTIC_SONNET_MODEL"),
		openRouterMessagesAgenticCandidate("openrouter_sonnet", "sonnet", "anthropic/claude-sonnet-4.6", "OPENROUTER_AGENTIC_SONNET_MODEL"),
		claudeFamilyAgenticCandidate("claude_opus", "opus", "claude-opus-4-6", "CLAUDE_AGENTIC_OPUS_MODEL", "claude", nil, true),
		anthropicAgenticCandidate("anthropic_opus", "opus", "claude-opus-4-6", "ANTHROPIC_AGENTIC_OPUS_MODEL"),
		openRouterMessagesAgenticCandidate("openrouter_opus", "opus", "anthropic/claude-opus-4.6", "OPENROUTER_AGENTIC_OPUS_MODEL"),
		minimaxMessagesAgenticCandidate(),
	}
}

func openAIResponsesAgenticCandidate(name, model, env string) agenticCodingCandidate {
	return agenticCodingCandidate{
		name:                 name,
		publicModel:          model,
		nativeModel:          model,
		modelEnv:             env,
		providerType:         "openai_responses",
		providerAPI:          adapt.ApiOpenAIResponses,
		sourceAPI:            adapt.ApiOpenAIResponses,
		modelDBService:       "openai",
		apiKeyEnv:            []string{"OPENAI_API_KEY", "OPENAI_KEY"},
		maxOutputTokens:      2048,
		cacheWarmupNoUsage:   true,
		cachePrefixRepeat:    1200,
		responseFormatSchema: true,
	}
}

func codexAgenticCandidate(name, model, env string) agenticCodingCandidate {
	return agenticCodingCandidate{
		name:               name,
		publicModel:        model,
		nativeModel:        model,
		modelEnv:           env,
		providerType:       "codex_responses",
		providerAPI:        adapt.ApiCodexResponses,
		sourceAPI:          adapt.ApiOpenAIResponses,
		modelDBService:     "codex",
		apiKeyEnv:          []string{codex.EnvAccessToken, codex.EnvOAuthToken},
		localCodexOAuth:    true,
		maxOutputTokens:    2048,
		cacheWarmupNoUsage: true,
		cachePrefixRepeat:  1200,
	}
}

func openRouterResponsesAgenticCandidate(name, publicModel, nativeModel, env string) agenticCodingCandidate {
	return agenticCodingCandidate{
		name:                 name,
		publicModel:          publicModel,
		nativeModel:          nativeModel,
		modelEnv:             env,
		providerType:         "openrouter_responses",
		providerAPI:          adapt.ApiOpenRouterResponses,
		sourceAPI:            adapt.ApiOpenAIResponses,
		modelDBService:       "openrouter",
		apiKeyEnv:            []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
		maxOutputTokens:      2048,
		cacheWarmupNoUsage:   true,
		cachePrefixRepeat:    1200,
		responseFormatSchema: true,
	}
}

func anthropicAgenticCandidate(name, publicModel, nativeModel, env string) agenticCodingCandidate {
	return claudeFamilyAgenticCandidate(name, publicModel, nativeModel, env, "anthropic", []string{"ANTHROPIC_API_KEY"}, false)
}

func claudeFamilyAgenticCandidate(name, publicModel, nativeModel, env, providerType string, apiKeyEnv []string, localClaude bool) agenticCodingCandidate {
	return agenticCodingCandidate{
		name:               name,
		publicModel:        publicModel,
		nativeModel:        nativeModel,
		modelEnv:           env,
		providerType:       providerType,
		providerAPI:        adapt.ApiAnthropicMessages,
		sourceAPI:          adapt.ApiAnthropicMessages,
		modelDBService:     "anthropic",
		apiKeyEnv:          apiKeyEnv,
		localClaudeOAuth:   localClaude,
		maxOutputTokens:    2048,
		cachePrefixRepeat:  360,
		skipReasoningText:  false,
		cacheWarmupNoUsage: false,
	}
}

func openRouterMessagesAgenticCandidate(name, publicModel, nativeModel, env string) agenticCodingCandidate {
	return agenticCodingCandidate{
		name:               name,
		publicModel:        publicModel,
		nativeModel:        nativeModel,
		modelEnv:           env,
		providerType:       "openrouter_messages",
		providerAPI:        adapt.ApiOpenRouterAnthropicMessages,
		sourceAPI:          adapt.ApiAnthropicMessages,
		modelDBService:     "openrouter",
		apiKeyEnv:          []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
		maxOutputTokens:    2048,
		cacheWarmupNoUsage: true,
		cachePrefixRepeat:  1200,
	}
}

func minimaxMessagesAgenticCandidate() agenticCodingCandidate {
	return agenticCodingCandidate{
		name:               "minimax_latest",
		publicModel:        "minimax-latest",
		nativeModel:        "MiniMax-M2.7",
		modelEnv:           "MINIMAX_AGENTIC_MODEL",
		providerType:       "minimax_messages",
		providerAPI:        adapt.ApiMiniMaxAnthropicMessages,
		sourceAPI:          adapt.ApiAnthropicMessages,
		apiKeyEnv:          []string{"MINIMAX_API_KEY", "MINIMAX_KEY"},
		maxOutputTokens:    1024,
		cacheWarmupNoUsage: true,
		cachePrefixRepeat:  1200,
		skipReasoningText:  true,
	}
}

func newAgenticCodingClient(t *testing.T, candidate agenticCodingCandidate) (unified.Client, string) {
	t.Helper()
	if firstSetEnv(candidate.apiKeyEnv...) == "" && !(candidate.localClaudeOAuth && messages.LocalTokenStoreAvailable()) && !(candidate.localCodexOAuth && codex.LocalAvailable()) {
		t.Skipf("%s unavailable: set one of %s or local credentials", candidate.name, strings.Join(candidate.apiKeyEnv, ","))
	}
	model := candidate.nativeModel
	if fromEnv := os.Getenv(candidate.modelEnv); fromEnv != "" {
		model = fromEnv
	}
	cfg := adapterconfig.Config{
		Providers: []adapterconfig.ProviderConfig{{
			Name:             candidate.providerType,
			Type:             candidate.providerType,
			ModelDBServiceID: candidate.modelDBService,
		}},
		Routes: []adapterconfig.RouteConfig{{
			SourceAPI:   candidate.sourceAPI,
			Model:       candidate.publicModel,
			NativeModel: model,
			Provider:    candidate.providerType,
			ProviderAPI: candidate.providerAPI,
			Weight:      100,
		}},
	}
	adapterconfig.ApplyDefaults(&cfg)
	client, err := adapterconfig.NewMuxClient(cfg, adapterconfig.WithSourceAPI(candidate.sourceAPI), adapterconfig.WithFallback(false))
	if err != nil {
		t.Fatalf("new mux client: %v", err)
	}
	return client, candidate.publicModel
}

func checkAgenticText(t *testing.T, client unified.Client, model string, candidate agenticCodingCandidate) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	maxTokens := agenticMaxTokens(candidate, 128)
	req := unified.Request{
		Model:           model,
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "Reply with exactly: agentic compatibility text ok"}},
		}},
		Stream: true,
	}
	resp, err := collectSmokeResponse(ctx, client, req)
	if err != nil {
		t.Fatalf("text request: %v", err)
	}
	if !strings.Contains(strings.ToLower(responseText(resp)), "agentic compatibility text ok") {
		t.Fatalf("text response = %q", responseText(resp))
	}
	if resp.Usage.TotalTokens() == 0 && resp.Usage.InputTokens() == 0 && resp.Usage.OutputTokens() == 0 {
		t.Fatalf("text response missing usage: %+v", resp.Usage)
	}
	return "live"
}

func checkAgenticToolContinuation(t *testing.T, client unified.Client, model string, candidate agenticCodingCandidate) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	maxTokens := agenticMaxTokens(candidate, 256)
	tool := agenticLookupTool()
	userMessage := unified.Message{
		Role:    unified.RoleUser,
		Content: []unified.ContentPart{unified.TextPart{Text: "Use the lookup_symbol tool with symbol set to nebula-17. After the tool result is provided, reply with the marker value from that result. Do not answer directly before using the tool."}},
	}
	first, err := collectSmokeResponse(ctx, client, unified.Request{
		Model:           model,
		MaxOutputTokens: &maxTokens,
		Messages:        []unified.Message{userMessage},
		Tools:           []unified.Tool{tool},
		ToolChoice:      &unified.ToolChoice{Mode: unified.ToolChoiceTool, Name: tool.Name},
		Stream:          true,
	})
	if err != nil {
		t.Fatalf("tool request: %v", err)
	}
	if first.FinishReason != unified.FinishReasonToolCall || len(first.ToolCalls) != 1 {
		t.Fatalf("tool response = %+v", first)
	}
	call := first.ToolCalls[0]
	if call.Name != tool.Name || !strings.Contains(strings.ToLower(string(call.Arguments)), "nebula-17") {
		t.Fatalf("tool call = %+v", call)
	}

	final, err := collectSmokeResponse(ctx, client, unified.Request{
		Model:           model,
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{
			userMessage,
			{Role: unified.RoleAssistant, ToolCalls: []unified.ToolCall{call}},
			{
				Role: unified.RoleTool,
				ToolResults: []unified.ToolResult{{
					ToolCallID: call.ID,
					Name:       call.Name,
					Content:    []unified.ContentPart{unified.TextPart{Text: `{"marker":"agentic tool continuation ok"}`}},
				}},
			},
		},
		Stream: true,
	})
	if err != nil {
		t.Fatalf("tool continuation: %v", err)
	}
	if !strings.Contains(strings.ToLower(responseText(final)), "agentic tool continuation ok") {
		t.Fatalf("tool continuation text = %q", responseText(final))
	}
	return "live"
}

func checkAgenticStructuredOutput(t *testing.T, client unified.Client, model string, candidate agenticCodingCandidate) string {
	t.Helper()
	if !candidate.responseFormatSchema {
		// Tool schemas are the structured-output path for Anthropic-family APIs.
		return checkAgenticStructuredTool(t, client, model, candidate)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	maxTokens := agenticMaxTokens(candidate, 256)
	resp, err := collectSmokeResponse(ctx, client, unified.Request{
		Model:           model,
		MaxOutputTokens: &maxTokens,
		ResponseFormat: &unified.ResponseFormat{
			Kind:   unified.ResponseFormatJSONSchema,
			Name:   "agentic_structured",
			Strict: true,
			Schema: json.RawMessage(`{"type":"object","properties":{"marker":{"type":"string"}},"required":["marker"],"additionalProperties":false}`),
		},
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: `Return {"marker":"agentic structured ok"} and no other keys.`}},
		}},
		Stream: true,
	})
	if err != nil {
		t.Fatalf("structured response_format request: %v", err)
	}
	var got struct {
		Marker string `json:"marker"`
	}
	if err := json.Unmarshal([]byte(responseText(resp)), &got); err != nil {
		t.Fatalf("structured response is not JSON object: text=%q err=%v", responseText(resp), err)
	}
	if got.Marker != "agentic structured ok" {
		t.Fatalf("structured marker = %q", got.Marker)
	}
	return "live"
}

func checkAgenticStructuredTool(t *testing.T, client unified.Client, model string, candidate agenticCodingCandidate) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	maxTokens := agenticMaxTokens(candidate, 256)
	tool := agenticLookupTool()
	resp, err := collectSmokeResponse(ctx, client, unified.Request{
		Model:           model,
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "Use lookup_symbol with symbol set to structured-42."}},
		}},
		Tools:      []unified.Tool{tool},
		ToolChoice: &unified.ToolChoice{Mode: unified.ToolChoiceTool, Name: tool.Name},
		Stream:     true,
	})
	if err != nil {
		t.Fatalf("structured tool request: %v", err)
	}
	if len(resp.ToolCalls) != 1 || !strings.Contains(strings.ToLower(string(resp.ToolCalls[0].Arguments)), "structured-42") {
		t.Fatalf("structured tool calls = %+v", resp.ToolCalls)
	}
	return "live"
}

func checkAgenticReasoning(t *testing.T, client unified.Client, model string, candidate agenticCodingCandidate) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	maxTokens := candidate.maxOutputTokens
	if maxTokens < 2048 {
		maxTokens = 2048
	}
	budget := 1024
	resp, err := collectSmokeResponse(ctx, client, unified.Request{
		Model:           model,
		MaxOutputTokens: &maxTokens,
		Reasoning:       &unified.ReasoningConfig{Effort: unified.ReasoningEffortHigh, MaxTokens: &budget, Expose: true},
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "What is 1+1? After thinking, answer with exactly: agentic reasoning ok"}},
		}},
		Stream: true,
	})
	if err != nil {
		t.Fatalf("reasoning request: %v", err)
	}
	if !strings.Contains(strings.ToLower(responseText(resp)), "agentic reasoning ok") {
		t.Fatalf("reasoning response text = %q", responseText(resp))
	}
	if !candidate.skipReasoningText && responseReasoningText(resp) == "" && resp.Usage.ReasoningTokens() == 0 {
		t.Fatalf("missing reasoning evidence: response=%+v usage=%+v", resp, resp.Usage)
	}
	return "live"
}

func checkAgenticPromptCache(t *testing.T, client unified.Client, model string, candidate agenticCodingCandidate) (string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	maxTokens := agenticMaxTokens(candidate, 32)
	req := unified.Request{
		Model:           model,
		MaxOutputTokens: &maxTokens,
		CachePolicy:     unified.CachePolicyOn,
		CacheKey:        cacheSmokeKey(candidate.name),
		Instructions: []unified.Instruction{{
			Kind: unified.InstructionSystem,
			Content: []unified.ContentPart{unified.TextPart{
				Text:         cacheSmokePrefix(candidate.cachePrefixRepeat),
				CacheControl: unified.EphemeralCache(""),
			}},
		}},
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "Reply with exactly: agentic cache ok"}},
		}},
		Stream: true,
	}
	first, err := collectSmokeResponse(ctx, client, req)
	if err != nil {
		t.Fatalf("prompt cache request: %v", err)
	}
	if !strings.Contains(strings.ToLower(responseText(first)), "agentic cache ok") {
		t.Fatalf("cache response text = %q", responseText(first))
	}
	if first.Usage.CacheReadTokens() > 0 || first.Usage.CacheWriteTokens() > 0 {
		return "live", "live"
	}
	if !candidate.cacheWarmupNoUsage {
		t.Fatalf("first cache request missing cache accounting: %+v", first.Usage)
	}
	var last unified.Response
	for attempt := 0; attempt < 4; attempt++ {
		time.Sleep(time.Duration(attempt+1) * time.Second)
		last, err = collectSmokeResponse(ctx, client, req)
		if err != nil {
			t.Fatalf("prompt cache read attempt %d: %v", attempt+1, err)
		}
		if last.Usage.CacheReadTokens() > 0 || last.Usage.CacheWriteTokens() > 0 {
			return "live", "live"
		}
	}
	t.Fatalf("prompt cache accounting not observed: first=%+v last=%+v", first.Usage, last.Usage)
	return "live", "failed"
}

func agenticLookupTool() unified.Tool {
	return unified.Tool{
		Kind:        unified.ToolKindFunction,
		Name:        "lookup_symbol",
		Description: "Looks up a marker symbol.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"symbol":{"type":"string"}},"required":["symbol"],"additionalProperties":false}`),
	}
}

func agenticMaxTokens(candidate agenticCodingCandidate, defaultValue int) int {
	if candidate.maxOutputTokens > defaultValue {
		return candidate.maxOutputTokens
	}
	return defaultValue
}
