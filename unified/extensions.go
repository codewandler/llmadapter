package unified

import (
	"encoding/json"
	"errors"
	"sort"
)

const (
	ExtOpenAIPreviousResponseID   = "openai.responses.previous_response_id"
	ExtOpenAIStore                = "openai.responses.store"
	ExtOpenAIPromptCacheKey       = "openai.responses.prompt_cache_key"
	ExtOpenAIPromptCacheRetention = "openai.responses.prompt_cache_retention"
	ExtAnthropicBetas             = "anthropic.betas"
	ExtGeminiSafetySettings       = "gemini.safety_settings"
	ExtOpenRouterModels           = "openrouter.models"
	ExtOpenRouterRoute            = "openrouter.route"
	ExtOpenRouterProvider         = "openrouter.provider"
	ExtOpenRouterProviderPrefs    = "openrouter.provider_preferences"
	ExtOpenRouterPlugins          = "openrouter.plugins"
	ExtOpenRouterDebug            = "openrouter.debug"
	ExtOpenRouterTrace            = "openrouter.trace"
	ExtOpenRouterSessionID        = "openrouter.session_id"
	ExtCodexSessionID             = "codex.session_id"
	ExtCodexWindowID              = "codex.window_id"
	ExtCodexTurnState             = "codex.turn_state"
	ExtCodexTurnMetadata          = "codex.turn_metadata"
	ExtCodexParentThreadID        = "codex.parent_thread_id"
	ExtCodexSubagent              = "codex.subagent"
	ExtCodexMemgenRequest         = "codex.memgen_request"
	ExtCodexIncludeTimingMetrics  = "codex.include_timing_metrics"
	ExtOllamaOptions              = "ollama.options"
)

type Extensions struct {
	values map[string]json.RawMessage
}

type OpenRouterRawExtensions struct {
	Models        json.RawMessage
	Route         json.RawMessage
	Provider      json.RawMessage
	ProviderPrefs json.RawMessage
	Plugins       json.RawMessage
	Debug         json.RawMessage
	Trace         json.RawMessage
	SessionID     json.RawMessage
}

type OpenAIResponsesExtensions struct {
	PreviousResponseID   string
	Store                *bool
	PromptCacheKey       string
	PromptCacheRetention string
}

type OpenRouterExtensions struct {
	Models        json.RawMessage
	Route         json.RawMessage
	Provider      json.RawMessage
	ProviderPrefs json.RawMessage
	Plugins       json.RawMessage
	Debug         json.RawMessage
	Trace         json.RawMessage
	SessionID     json.RawMessage
}

type AnthropicExtensions struct {
	Betas []string
}

type CodexExtensions struct {
	SessionID            string
	WindowID             string
	TurnState            string
	TurnMetadata         string
	ParentThreadID       string
	Subagent             bool
	MemgenRequest        bool
	IncludeTimingMetrics bool
}

func (e *Extensions) Set(key string, value any) error {
	if e.values == nil {
		e.values = make(map[string]json.RawMessage)
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	e.values[key] = b
	return nil
}

func (e *Extensions) SetRaw(key string, raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	if !json.Valid(raw) {
		return errors.New("invalid raw extension JSON")
	}
	if e.values == nil {
		e.values = make(map[string]json.RawMessage)
	}
	e.values[key] = append(json.RawMessage(nil), raw...)
	return nil
}

func (e Extensions) Raw(key string) json.RawMessage {
	raw, ok := e.values[key]
	if !ok || len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func (e Extensions) Has(key string) bool {
	_, ok := e.values[key]
	return ok
}

func (e Extensions) Keys() []string {
	keys := make([]string, 0, len(e.values))
	for key := range e.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func GetExtension[T any](e Extensions, key string) (T, bool, error) {
	var zero T
	raw, ok := e.values[key]
	if !ok {
		return zero, false, nil
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		return zero, true, err
	}
	return out, true, nil
}

func OpenRouterRawExtensionsFrom(e Extensions) OpenRouterRawExtensions {
	raw, _ := OpenRouterExtensionsFrom(e)
	return OpenRouterRawExtensions(raw)
}

func SetOpenRouterRawExtensions(e *Extensions, raw OpenRouterRawExtensions) error {
	if err := e.SetRaw(ExtOpenRouterModels, raw.Models); err != nil {
		return err
	}
	if err := e.SetRaw(ExtOpenRouterRoute, raw.Route); err != nil {
		return err
	}
	if err := e.SetRaw(ExtOpenRouterProvider, raw.Provider); err != nil {
		return err
	}
	if err := e.SetRaw(ExtOpenRouterProviderPrefs, raw.ProviderPrefs); err != nil {
		return err
	}
	if err := e.SetRaw(ExtOpenRouterPlugins, raw.Plugins); err != nil {
		return err
	}
	if err := e.SetRaw(ExtOpenRouterDebug, raw.Debug); err != nil {
		return err
	}
	if err := e.SetRaw(ExtOpenRouterTrace, raw.Trace); err != nil {
		return err
	}
	return e.SetRaw(ExtOpenRouterSessionID, raw.SessionID)
}

func OpenAIResponsesExtensionsFrom(e Extensions) (OpenAIResponsesExtensions, []WarningEvent) {
	var out OpenAIResponsesExtensions
	var warnings []WarningEvent
	setString := func(key string, dest *string) {
		value, ok, err := GetExtension[string](e, key)
		if err != nil {
			warnings = append(warnings, extensionWarning(key, err))
			return
		}
		if ok {
			*dest = value
		}
	}
	setString(ExtOpenAIPreviousResponseID, &out.PreviousResponseID)
	setString(ExtOpenAIPromptCacheKey, &out.PromptCacheKey)
	setString(ExtOpenAIPromptCacheRetention, &out.PromptCacheRetention)
	value, ok, err := GetExtension[bool](e, ExtOpenAIStore)
	if err != nil {
		warnings = append(warnings, extensionWarning(ExtOpenAIStore, err))
	} else if ok {
		out.Store = &value
	}
	return out, warnings
}

func SetOpenAIResponsesExtensions(e *Extensions, value OpenAIResponsesExtensions) error {
	if value.PreviousResponseID != "" {
		if err := e.Set(ExtOpenAIPreviousResponseID, value.PreviousResponseID); err != nil {
			return err
		}
	}
	if value.Store != nil {
		if err := e.Set(ExtOpenAIStore, *value.Store); err != nil {
			return err
		}
	}
	if value.PromptCacheKey != "" {
		if err := e.Set(ExtOpenAIPromptCacheKey, value.PromptCacheKey); err != nil {
			return err
		}
	}
	if value.PromptCacheRetention != "" {
		if err := e.Set(ExtOpenAIPromptCacheRetention, value.PromptCacheRetention); err != nil {
			return err
		}
	}
	return nil
}

func OpenRouterExtensionsFrom(e Extensions) (OpenRouterExtensions, []WarningEvent) {
	return OpenRouterExtensions{
		Models:        e.Raw(ExtOpenRouterModels),
		Route:         e.Raw(ExtOpenRouterRoute),
		Provider:      e.Raw(ExtOpenRouterProvider),
		ProviderPrefs: e.Raw(ExtOpenRouterProviderPrefs),
		Plugins:       e.Raw(ExtOpenRouterPlugins),
		Debug:         e.Raw(ExtOpenRouterDebug),
		Trace:         e.Raw(ExtOpenRouterTrace),
		SessionID:     e.Raw(ExtOpenRouterSessionID),
	}, nil
}

func SetOpenRouterExtensions(e *Extensions, value OpenRouterExtensions) error {
	return SetOpenRouterRawExtensions(e, OpenRouterRawExtensions(value))
}

func AnthropicExtensionsFrom(e Extensions) (AnthropicExtensions, []WarningEvent) {
	var out AnthropicExtensions
	var warnings []WarningEvent
	if value, ok, err := GetExtension[[]string](e, ExtAnthropicBetas); err != nil {
		warnings = append(warnings, extensionWarning(ExtAnthropicBetas, err))
	} else if ok {
		out.Betas = append([]string(nil), value...)
	}
	return out, warnings
}

func SetAnthropicExtensions(e *Extensions, value AnthropicExtensions) error {
	if len(value.Betas) == 0 {
		return nil
	}
	return e.Set(ExtAnthropicBetas, append([]string(nil), value.Betas...))
}

func CodexExtensionsFrom(e Extensions) (CodexExtensions, []WarningEvent) {
	var out CodexExtensions
	var warnings []WarningEvent
	setString := func(key string, dest *string) {
		value, ok, err := GetExtension[string](e, key)
		if err != nil {
			warnings = append(warnings, extensionWarning(key, err))
			return
		}
		if ok {
			*dest = value
		}
	}
	setBool := func(key string, dest *bool) {
		value, ok, err := GetExtension[bool](e, key)
		if err != nil {
			warnings = append(warnings, extensionWarning(key, err))
			return
		}
		if ok {
			*dest = value
		}
	}
	setString(ExtCodexSessionID, &out.SessionID)
	setString(ExtCodexWindowID, &out.WindowID)
	setString(ExtCodexTurnState, &out.TurnState)
	setString(ExtCodexTurnMetadata, &out.TurnMetadata)
	setString(ExtCodexParentThreadID, &out.ParentThreadID)
	setBool(ExtCodexSubagent, &out.Subagent)
	setBool(ExtCodexMemgenRequest, &out.MemgenRequest)
	setBool(ExtCodexIncludeTimingMetrics, &out.IncludeTimingMetrics)
	return out, warnings
}

func SetCodexExtensions(e *Extensions, value CodexExtensions) error {
	if value.SessionID != "" {
		if err := e.Set(ExtCodexSessionID, value.SessionID); err != nil {
			return err
		}
	}
	if value.WindowID != "" {
		if err := e.Set(ExtCodexWindowID, value.WindowID); err != nil {
			return err
		}
	}
	if value.TurnState != "" {
		if err := e.Set(ExtCodexTurnState, value.TurnState); err != nil {
			return err
		}
	}
	if value.TurnMetadata != "" {
		if err := e.Set(ExtCodexTurnMetadata, value.TurnMetadata); err != nil {
			return err
		}
	}
	if value.ParentThreadID != "" {
		if err := e.Set(ExtCodexParentThreadID, value.ParentThreadID); err != nil {
			return err
		}
	}
	if value.Subagent {
		if err := e.Set(ExtCodexSubagent, true); err != nil {
			return err
		}
	}
	if value.MemgenRequest {
		if err := e.Set(ExtCodexMemgenRequest, true); err != nil {
			return err
		}
	}
	if value.IncludeTimingMetrics {
		if err := e.Set(ExtCodexIncludeTimingMetrics, true); err != nil {
			return err
		}
	}
	return nil
}

func (e Extensions) TransportValues() map[string]any {
	if len(e.values) == 0 {
		return nil
	}
	out := make(map[string]any, len(e.values))
	for key, raw := range e.values {
		out[key] = append(json.RawMessage(nil), raw...)
	}
	return out
}

func extensionWarning(key string, err error) WarningEvent {
	return WarningEvent{
		Code:    "invalid_extension_dropped",
		Message: "invalid extension " + key + " was dropped: " + err.Error(),
		Source:  "unified.extensions",
		Meta:    map[string]any{"key": key},
	}
}
