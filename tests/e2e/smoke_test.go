package e2e

import (
	"context"
	"encoding/json"
	"errors"
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
	parallelTools         bool
	reasoning             bool
	promptCache           bool
	cacheWarmupNoUsage    bool
	cachePrefixRepeat     int
	responsesContinuation bool
	skipInvalidModel      bool
	maxOutputTokens       int
	newClient             func(apiKey string) (unified.Client, error)
}

func TestSmokeTextStream(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}

	for _, provider := range smokeProviders() {
		t.Run(provider.name, func(t *testing.T) {
			t.Parallel()
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
			t.Parallel()
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
			t.Parallel()
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
			t.Parallel()
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
					unified.TextPart{Text: "Use the lookup_city tool with city set to Berlin."},
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

func TestSmokeParallelToolUse(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}

	for _, provider := range smokeProviders() {
		t.Run(provider.name, func(t *testing.T) {
			t.Parallel()
			if !provider.parallelTools {
				t.Skipf("%s does not advertise parallel tool smoke support in this slice", provider.name)
			}
			client, model := newSmokeClient(t, provider)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			maxTokens := provider.maxTokens(256)
			events, err := client.Request(ctx, unified.Request{
				Model:           model,
				MaxOutputTokens: &maxTokens,
				Messages: []unified.Message{{
					Role: unified.RoleUser,
					Content: []unified.ContentPart{
						unified.TextPart{Text: "Call both tools now: lookup_city with city Berlin and lookup_country with country Germany. Do not answer directly."},
					},
				}},
				Tools: []unified.Tool{
					{
						Kind:        unified.ToolKindFunction,
						Name:        "lookup_city",
						Description: "Looks up a city by name.",
						InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"],"additionalProperties":false}`),
					},
					{
						Kind:        unified.ToolKindFunction,
						Name:        "lookup_country",
						Description: "Looks up a country by name.",
						InputSchema: json.RawMessage(`{"type":"object","properties":{"country":{"type":"string"}},"required":["country"],"additionalProperties":false}`),
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
			if len(resp.ToolCalls) < 2 {
				t.Fatalf("tool calls = %+v", resp.ToolCalls)
			}
			seen := map[string]bool{}
			for _, call := range resp.ToolCalls {
				seen[call.Name] = true
			}
			if !seen["lookup_city"] || !seen["lookup_country"] {
				t.Fatalf("tool calls = %+v", resp.ToolCalls)
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
			t.Parallel()
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
			t.Parallel()
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

func TestSmokeCodexWebSocketContinuation(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}

	var provider smokeProvider
	for _, candidate := range smokeProviders() {
		if candidate.name == "codex_responses" {
			provider = candidate
			break
		}
	}
	if provider.name == "" {
		t.Fatal("codex_responses smoke provider is not registered")
	}

	client, model := newSmokeClient(t, provider)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	sessionID := "codex-ws-" + time.Now().UTC().Format("20060102150405")
	branchID := "main"
	maxTokens := provider.maxTokens(64)
	firstUser := unified.Message{
		Role: unified.RoleUser,
		Content: []unified.ContentPart{
			unified.TextPart{Text: "Reply with exactly: codex websocket first ok"},
		},
	}
	firstReq := unified.Request{
		Model:           model,
		MaxOutputTokens: &maxTokens,
		CachePolicy:     unified.CachePolicyOn,
		CacheKey:        sessionID,
		Messages:        []unified.Message{firstUser},
		Stream:          true,
	}
	setCodexSessionExtensions(t, &firstReq, sessionID, branchID)

	first, firstMeta, err := collectSmokeResponseWithExecution(ctx, client, firstReq)
	if err != nil {
		t.Fatalf("first websocket request: %v", err)
	}
	if !strings.Contains(strings.ToLower(responseText(first)), "codex websocket first ok") {
		t.Fatalf("first response text = %q; response=%+v metadata=%+v", responseText(first), first, firstMeta)
	}
	if firstMeta.transport() != unified.TransportWebSocket && firstMeta.transport() != unified.TransportHTTPSSE {
		t.Fatalf("first transport = %q, want %q or %q; metadata=%+v", firstMeta.transport(), unified.TransportWebSocket, unified.TransportHTTPSSE, firstMeta)
	}
	if firstMeta.transport() == unified.TransportHTTPSSE {
		t.Log("codex websocket did not complete before a response; provider fell back to HTTP/SSE")
	}
	if firstMeta.internalContinuation() != unified.ContinuationReplay {
		t.Fatalf("first internal continuation = %q, want %q; metadata=%+v", firstMeta.internalContinuation(), unified.ContinuationReplay, firstMeta)
	}
	if firstMeta.transport() == unified.TransportHTTPSSE {
		return
	}

	secondReq := unified.Request{
		Model:           model,
		MaxOutputTokens: &maxTokens,
		CachePolicy:     unified.CachePolicyOn,
		CacheKey:        sessionID,
		Messages: []unified.Message{
			firstUser,
			{
				Role: unified.RoleAssistant,
				Content: []unified.ContentPart{
					unified.TextPart{Text: responseText(first)},
				},
			},
			{
				Role: unified.RoleUser,
				Content: []unified.ContentPart{
					unified.TextPart{Text: "Reply with exactly: codex websocket second ok"},
				},
			},
		},
		Stream: true,
	}
	setCodexSessionExtensions(t, &secondReq, sessionID, branchID)

	second, secondMeta, err := collectSmokeResponseWithExecution(ctx, client, secondReq)
	if err != nil {
		t.Fatalf("second websocket request: %v", err)
	}
	if !strings.Contains(strings.ToLower(responseText(second)), "codex websocket second ok") {
		t.Fatalf("second response text = %q; response=%+v metadata=%+v", responseText(second), second, secondMeta)
	}
	if firstMeta.transport() == unified.TransportWebSocket && secondMeta.transport() != unified.TransportWebSocket {
		t.Fatalf("second transport = %q, want %q after websocket first turn; metadata=%+v", secondMeta.transport(), unified.TransportWebSocket, secondMeta)
	}
	if firstMeta.transport() == unified.TransportHTTPSSE && secondMeta.transport() != unified.TransportHTTPSSE {
		t.Fatalf("second transport = %q, want %q after HTTP fallback; metadata=%+v", secondMeta.transport(), unified.TransportHTTPSSE, secondMeta)
	}
	wantSecondContinuation := unified.ContinuationPreviousResponseID
	if firstMeta.transport() == unified.TransportHTTPSSE {
		wantSecondContinuation = unified.ContinuationReplay
	}
	if secondMeta.internalContinuation() != wantSecondContinuation {
		t.Fatalf("second internal continuation = %q, want %q; metadata=%+v", secondMeta.internalContinuation(), wantSecondContinuation, secondMeta)
	}
}

func TestSmokeCodexWebSocketPromptCache(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}

	var provider smokeProvider
	for _, candidate := range smokeProviders() {
		if candidate.name == "codex_responses" {
			provider = candidate
			break
		}
	}
	if provider.name == "" {
		t.Fatal("codex_responses smoke provider is not registered")
	}

	client, model := newSmokeClient(t, provider)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	sessionID := "codex-ws-cache-" + time.Now().UTC().Format("20060102150405")
	branchID := "cache"
	maxTokens := provider.maxTokens(16)
	req := unified.Request{
		Model:           model,
		MaxOutputTokens: &maxTokens,
		CachePolicy:     unified.CachePolicyOn,
		CacheKey:        sessionID,
		Instructions: []unified.Instruction{{
			Kind: unified.InstructionSystem,
			Content: []unified.ContentPart{unified.TextPart{
				Text:         cacheSmokePrefix(provider.cachePrefixRepeat),
				CacheControl: unified.EphemeralCache(""),
			}},
		}},
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "Reply with exactly: codex websocket cache ok"}},
		}},
		Stream: true,
	}
	setCodexSessionExtensions(t, &req, sessionID, branchID)

	first, firstMeta, err := collectSmokeResponseWithExecution(ctx, client, req)
	if err != nil {
		t.Fatalf("first websocket cache request: %v", err)
	}
	if firstMeta.transport() != unified.TransportWebSocket {
		t.Fatalf("first transport = %q, want %q; metadata=%+v response=%+v", firstMeta.transport(), unified.TransportWebSocket, firstMeta, first)
	}
	if first.Usage.CacheReadTokens() > 0 {
		return
	}
	if first.Usage.CacheWriteTokens() == 0 {
		t.Logf("first websocket cache request did not report cache write usage; checking follow-up cache reads: %+v", first.Usage)
	}

	var last unified.Response
	var lastMeta smokeExecutionMetadata
	for attempt := 0; attempt < 4; attempt++ {
		time.Sleep(time.Duration(attempt+1) * time.Second)
		last, lastMeta, err = collectSmokeResponseWithExecution(ctx, client, req)
		if err != nil {
			t.Fatalf("websocket cache read attempt %d: %v", attempt+1, err)
		}
		if lastMeta.transport() != unified.TransportWebSocket {
			t.Fatalf("attempt %d transport = %q, want %q; metadata=%+v response=%+v", attempt+1, lastMeta.transport(), unified.TransportWebSocket, lastMeta, last)
		}
		if last.Usage.CacheReadTokens() > 0 {
			return
		}
	}
	t.Fatalf("websocket cache requests did not report cache read usage; first=%+v last=%+v metadata=%+v", first.Usage, last.Usage, lastMeta)
}

func TestSmokeOpenAIResponsesWebSocket(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}
	apiKey := firstSetEnv("OPENAI_API_KEY", "OPENAI_KEY")
	if apiKey == "" {
		t.Skip("set OPENAI_API_KEY or OPENAI_KEY to run OpenAI Responses WebSocket smoke")
	}
	model := os.Getenv("OPENAI_RESPONSES_MODEL")
	if model == "" {
		model = "gpt-4.1-mini"
	}
	client, err := openairesponses.NewClient(
		openairesponses.WithAPIKey(apiKey),
		openairesponses.WithWebSocketMode(openairesponses.WebSocketModeAuto),
	)
	if err != nil {
		t.Fatalf("new OpenAI Responses client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	maxTokens := 32
	sessionID := "openai-ws-" + time.Now().UTC().Format("20060102150405")
	resp, metadata, err := collectSmokeResponseWithExecution(ctx, client, unified.Request{
		Model:           model,
		MaxOutputTokens: &maxTokens,
		CacheKey:        sessionID,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "Reply with exactly: openai websocket smoke ok"}},
		}},
		Stream: true,
	})
	if err != nil {
		t.Fatalf("openai responses websocket request: %v", err)
	}
	if metadata.transport() != unified.TransportWebSocket {
		t.Fatalf("transport = %q, want %q; metadata=%+v response=%+v", metadata.transport(), unified.TransportWebSocket, metadata, resp)
	}
	if !strings.Contains(strings.ToLower(responseText(resp)), "openai websocket smoke ok") {
		t.Fatalf("response text = %q; response=%+v metadata=%+v", responseText(resp), resp, metadata)
	}
	if resp.Usage.TotalTokens() == 0 && resp.Usage.InputTokens() == 0 && resp.Usage.OutputTokens() == 0 {
		t.Fatalf("missing usage in response: %+v", resp)
	}
}

func TestSmokeInvalidCredential(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}

	for _, provider := range smokeProviders() {
		t.Run(provider.name, func(t *testing.T) {
			t.Parallel()
			if len(provider.apiKeyEnv) == 0 || provider.localCodexOAuth {
				t.Skipf("%s does not use direct API-key invalid credential coverage", provider.name)
			}
			client, err := provider.newClient("llmadapter-invalid-key")
			if err != nil {
				t.Fatalf("new client: %v", err)
			}
			assertSmokeAPIError(t, client, provider.model)
		})
	}
}

func TestSmokeInvalidModel(t *testing.T) {
	if os.Getenv("TEST_INTEGRATION") == "" {
		t.Skip("set TEST_INTEGRATION=1 to run e2e smoke tests")
	}

	for _, provider := range smokeProviders() {
		t.Run(provider.name, func(t *testing.T) {
			t.Parallel()
			if provider.skipInvalidModel {
				t.Skipf("%s does not reject the shared invalid model sentinel", provider.name)
			}
			client, _ := newSmokeClient(t, provider)
			assertSmokeAPIError(t, client, "llmadapter-invalid-model")
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
			name:          "openai_chat",
			apiKeyEnv:     []string{"OPENAI_API_KEY", "OPENAI_KEY"},
			modelEnv:      "OPENAI_MODEL",
			model:         "gpt-4.1-mini",
			tools:         true,
			parallelTools: true,
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
			parallelTools:         true,
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
			parallelTools:      true,
			reasoning:          true,
			promptCache:        true,
			cacheWarmupNoUsage: true,
			cachePrefixRepeat:  1200,
			newClient:          newCodexSmokeClient,
		},
		{
			name:          "openrouter_chat",
			apiKeyEnv:     []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
			modelEnv:      "OPENROUTER_MODEL",
			model:         "openai/gpt-4.1-mini",
			tools:         true,
			parallelTools: true,
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
			name:      "minimax_messages",
			apiKeyEnv: []string{"MINIMAX_API_KEY", "MINIMAX_KEY"},
			modelEnv:  "MINIMAX_MESSAGES_MODEL",
			model:     "MiniMax-M2.7",
			tools:     true,
			reasoning: true,
			// MiniMax emits reasoning before final text on the Anthropic-compatible surface.
			maxOutputTokens:  512,
			skipInvalidModel: true,
			newClient: func(apiKey string) (unified.Client, error) {
				return minimaxmessages.NewClient(minimaxmessages.WithAPIKey(apiKey))
			},
		},
		{
			name:          "openrouter_responses",
			apiKeyEnv:     []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
			modelEnv:      "OPENROUTER_RESPONSES_MODEL",
			model:         "openai/gpt-4.1-mini",
			tools:         true,
			parallelTools: true,
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

type smokeExecutionMetadata struct {
	route    *unified.RouteEvent
	provider *unified.ProviderExecutionEvent
}

func (m smokeExecutionMetadata) transport() unified.TransportKind {
	if m.route != nil && m.route.Transport != "" {
		return m.route.Transport
	}
	if m.provider != nil {
		return m.provider.Transport
	}
	return ""
}

func (m smokeExecutionMetadata) internalContinuation() unified.ContinuationMode {
	if m.route != nil && m.route.InternalContinuation != "" {
		return m.route.InternalContinuation
	}
	if m.provider != nil {
		return m.provider.InternalContinuation
	}
	return ""
}

func collectSmokeResponseWithExecution(ctx context.Context, client unified.Client, req unified.Request) (unified.Response, smokeExecutionMetadata, error) {
	events, err := client.Request(ctx, req)
	if err != nil {
		return unified.Response{}, smokeExecutionMetadata{}, err
	}

	filtered := make(chan unified.Event)
	metadataCh := make(chan smokeExecutionMetadata, 1)
	go func() {
		defer close(filtered)
		var metadata smokeExecutionMetadata
		defer func() {
			metadataCh <- metadata
		}()
		for ev := range events {
			switch event := ev.(type) {
			case unified.RouteEvent:
				copied := event
				metadata.route = &copied
				continue
			case unified.ProviderExecutionEvent:
				copied := event
				metadata.provider = &copied
				continue
			}
			select {
			case filtered <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()

	resp, err := unified.Collect(ctx, filtered)
	metadata := <-metadataCh
	return resp, metadata, err
}

func setCodexSessionExtensions(t *testing.T, req *unified.Request, sessionID, branchID string) {
	t.Helper()
	if err := unified.SetCodexExtensions(&req.Extensions, unified.CodexExtensions{
		InteractionMode: unified.InteractionSession,
		SessionID:       sessionID,
		BranchID:        branchID,
	}); err != nil {
		t.Fatalf("set codex extensions: %v", err)
	}
}

func assertSmokeAPIError(t *testing.T, client unified.Client, model string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	maxTokens := 8
	events, err := client.Request(ctx, unified.Request{
		Model:           model,
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
		Stream: true,
	})
	if err == nil {
		_, err = unified.Collect(ctx, events)
	}
	var apiErr *unified.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T %v, want APIError", err, err)
	}
	if apiErr.Message == "" && apiErr.StatusCode == 0 {
		t.Fatalf("APIError lacks useful details: %+v", apiErr)
	}
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
		anthropic.WithSystemCacheControl("1h"),
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
