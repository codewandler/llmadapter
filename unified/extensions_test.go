package unified

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestExtensionsRoundtrip(t *testing.T) {
	var e Extensions
	if err := e.Set("string", "value"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := GetExtension[string](e, "string")
	if err != nil || !ok || got != "value" {
		t.Fatalf("got (%q,%v,%v), want value,true,nil", got, ok, err)
	}
}

func TestExtensionsMissingAndKeys(t *testing.T) {
	var e Extensions
	_ = e.Set("b", 1)
	_ = e.Set("a", nil)
	if _, ok, err := GetExtension[string](e, "missing"); err != nil || ok {
		t.Fatalf("missing = ok %v err %v, want false nil", ok, err)
	}
	if !e.Has("a") || e.Has("missing") {
		t.Fatalf("Has returned unexpected result")
	}
	if got := e.Keys(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("Keys = %v", got)
	}
}

func TestExtensionsTypeMismatch(t *testing.T) {
	var e Extensions
	_ = e.Set("value", "abc")
	if _, ok, err := GetExtension[int](e, "value"); !ok || err == nil {
		t.Fatalf("expected present key with unmarshal error")
	}
}

func TestExtensionsRawRoundtrip(t *testing.T) {
	var e Extensions
	if err := e.SetRaw("raw", json.RawMessage(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if got := string(e.Raw("raw")); got != `{"a":1}` {
		t.Fatalf("Raw = %s", got)
	}
	if err := e.SetRaw("bad", json.RawMessage(`{`)); err == nil {
		t.Fatalf("expected invalid raw JSON error")
	}
}

func TestOpenRouterRawExtensionsRoundtrip(t *testing.T) {
	var e Extensions
	err := SetOpenRouterRawExtensions(&e, OpenRouterRawExtensions{
		Provider:  json.RawMessage(`{"order":["openai"]}`),
		Debug:     json.RawMessage(`true`),
		SessionID: json.RawMessage(`"sess_1"`),
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := OpenRouterRawExtensionsFrom(e)
	if string(raw.Provider) != `{"order":["openai"]}` || string(raw.Debug) != `true` || string(raw.SessionID) != `"sess_1"` {
		t.Fatalf("unexpected OpenRouter extensions: %+v", raw)
	}
}

func TestOpenAIResponsesExtensionsRoundtrip(t *testing.T) {
	store := true
	var e Extensions
	err := SetOpenAIResponsesExtensions(&e, OpenAIResponsesExtensions{
		PreviousResponseID:   "resp_1",
		Store:                &store,
		PromptCacheKey:       "cache",
		PromptCacheRetention: "24h",
	})
	if err != nil {
		t.Fatal(err)
	}
	got, warnings := OpenAIResponsesExtensionsFrom(e)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if got.PreviousResponseID != "resp_1" || got.Store == nil || *got.Store != true || got.PromptCacheKey != "cache" || got.PromptCacheRetention != "24h" {
		t.Fatalf("responses extensions = %+v", got)
	}
}

func TestOpenAIResponsesExtensionsValidationWarnings(t *testing.T) {
	var e Extensions
	if err := e.Set(ExtOpenAIPromptCacheRetention, "1h"); err != nil {
		t.Fatal(err)
	}
	if err := e.Set(ExtOpenAIPromptCacheKey, " "); err != nil {
		t.Fatal(err)
	}
	got, warnings := OpenAIResponsesExtensionsFrom(e)
	if got.PromptCacheRetention != "" || got.PromptCacheKey != "" {
		t.Fatalf("invalid responses extensions should be dropped: %+v", got)
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %+v", warnings)
	}
}

func TestOpenRouterExtensionsRoundtrip(t *testing.T) {
	var e Extensions
	err := SetOpenRouterExtensions(&e, OpenRouterExtensions{
		Models:    json.RawMessage(`["openai/gpt-5","anthropic/claude-sonnet"]`),
		Route:     json.RawMessage(`"fallback"`),
		Provider:  json.RawMessage(`{"order":["anthropic"]}`),
		Plugins:   json.RawMessage(`[{"id":"web"}]`),
		SessionID: json.RawMessage(`"sess"`),
	})
	if err != nil {
		t.Fatal(err)
	}
	got, warnings := OpenRouterExtensionsFrom(e)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if string(got.Models) != `["openai/gpt-5","anthropic/claude-sonnet"]` || string(got.Route) != `"fallback"` || string(got.Provider) != `{"order":["anthropic"]}` || string(got.Plugins) != `[{"id":"web"}]` || string(got.SessionID) != `"sess"` {
		t.Fatalf("openrouter extensions = %+v", got)
	}
}

func TestOpenRouterExtensionsValidationWarnings(t *testing.T) {
	var e Extensions
	if err := e.Set(ExtOpenRouterModels, []any{"openai/gpt-5", ""}); err != nil {
		t.Fatal(err)
	}
	if err := e.Set(ExtOpenRouterRoute, "bad\nroute"); err != nil {
		t.Fatal(err)
	}
	if err := e.Set(ExtOpenRouterProvider, map[string]any{"order": []any{"anthropic", ""}}); err != nil {
		t.Fatal(err)
	}
	if err := e.Set(ExtOpenRouterPlugins, []map[string]any{{"id": ""}}); err != nil {
		t.Fatal(err)
	}
	if err := e.Set(ExtOpenRouterSessionID, " "); err != nil {
		t.Fatal(err)
	}
	got, warnings := OpenRouterExtensionsFrom(e)
	if len(got.Models) != 0 || len(got.Route) != 0 || len(got.Provider) != 0 || len(got.Plugins) != 0 || len(got.SessionID) != 0 {
		t.Fatalf("invalid OpenRouter extensions should be dropped: %+v", got)
	}
	if len(warnings) != 5 {
		t.Fatalf("warnings = %+v", warnings)
	}
	raw := OpenRouterRawExtensionsFrom(e)
	if len(raw.Provider) == 0 || len(raw.SessionID) == 0 {
		t.Fatalf("raw OpenRouter extensions should preserve valid JSON: %+v", raw)
	}
}

func TestAnthropicExtensionsRoundtrip(t *testing.T) {
	var e Extensions
	contextManagement := json.RawMessage(`{"edits":[{"type":"clear_thinking_20251015","keep":"all"}]}`)
	if err := SetAnthropicExtensions(&e, AnthropicExtensions{Betas: []string{"thinking"}, ContextManagement: contextManagement}); err != nil {
		t.Fatal(err)
	}
	got, warnings := AnthropicExtensionsFrom(e)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if !reflect.DeepEqual(got.Betas, []string{"thinking"}) || string(got.ContextManagement) != string(contextManagement) {
		t.Fatalf("anthropic extensions = %+v", got)
	}
}

func TestAnthropicExtensionsValidationWarnings(t *testing.T) {
	var e Extensions
	if err := e.Set(ExtAnthropicBetas, []string{"thinking", "", "bad beta", "bad,beta"}); err != nil {
		t.Fatal(err)
	}
	if err := e.Set(ExtAnthropicContextManagement, []string{"not-object"}); err != nil {
		t.Fatal(err)
	}
	got, warnings := AnthropicExtensionsFrom(e)
	if !reflect.DeepEqual(got.Betas, []string{"thinking"}) {
		t.Fatalf("anthropic extensions = %+v", got)
	}
	if len(got.ContextManagement) != 0 {
		t.Fatalf("invalid context management should be dropped: %+v", got)
	}
	if len(warnings) != 4 {
		t.Fatalf("warnings = %+v", warnings)
	}
}

func TestCodexExtensionsRoundtrip(t *testing.T) {
	var e Extensions
	err := SetCodexExtensions(&e, CodexExtensions{
		InteractionMode:      InteractionSession,
		SessionID:            "sess",
		BranchID:             "branch",
		BranchHeadID:         "head",
		InputBaseHash:        "hash",
		ParentResponseID:     "resp_1",
		WindowID:             "sess:2",
		TurnState:            "sticky",
		TurnMetadata:         `{"turn":1}`,
		ParentThreadID:       "thread",
		Subagent:             true,
		MemgenRequest:        true,
		IncludeTimingMetrics: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, warnings := CodexExtensionsFrom(e)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if got.InteractionMode != InteractionSession || got.SessionID != "sess" || got.BranchID != "branch" || got.BranchHeadID != "head" || got.InputBaseHash != "hash" || got.ParentResponseID != "resp_1" || got.WindowID != "sess:2" || got.TurnState != "sticky" || got.TurnMetadata != `{"turn":1}` || got.ParentThreadID != "thread" || !got.Subagent || !got.MemgenRequest || !got.IncludeTimingMetrics {
		t.Fatalf("codex extensions = %+v", got)
	}
}

func TestCodexExtensionsWarnings(t *testing.T) {
	var e Extensions
	if err := e.Set(ExtCodexInteractionMode, "bad"); err != nil {
		t.Fatal(err)
	}
	if err := e.Set(ExtCodexSubagent, "not-bool"); err != nil {
		t.Fatal(err)
	}
	if err := e.Set(ExtCodexSessionID, " "); err != nil {
		t.Fatal(err)
	}
	if err := e.Set(ExtCodexWindowID, "bad\nwindow"); err != nil {
		t.Fatal(err)
	}
	if err := e.Set(ExtCodexTurnMetadata, "not-json"); err != nil {
		t.Fatal(err)
	}
	_, warnings := CodexExtensionsFrom(e)
	if len(warnings) != 5 || warnings[0].Code != "invalid_extension_dropped" {
		t.Fatalf("warnings = %+v", warnings)
	}
}

func TestSetCodexExtensionsRejectsInvalidInteractionMode(t *testing.T) {
	var e Extensions
	if err := SetCodexExtensions(&e, CodexExtensions{InteractionMode: InteractionMode("bad")}); err == nil {
		t.Fatalf("expected invalid interaction mode error")
	}
}
