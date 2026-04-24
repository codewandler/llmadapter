package unified

import (
	"encoding/json"
	"errors"
	"sort"
)

const (
	ExtOpenAIPreviousResponseID = "openai.responses.previous_response_id"
	ExtOpenAIStore              = "openai.responses.store"
	ExtAnthropicBetas           = "anthropic.betas"
	ExtGeminiSafetySettings     = "gemini.safety_settings"
	ExtOpenRouterModels         = "openrouter.models"
	ExtOpenRouterRoute          = "openrouter.route"
	ExtOpenRouterProvider       = "openrouter.provider"
	ExtOpenRouterProviderPrefs  = "openrouter.provider_preferences"
	ExtOpenRouterPlugins        = "openrouter.plugins"
	ExtOpenRouterDebug          = "openrouter.debug"
	ExtOpenRouterTrace          = "openrouter.trace"
	ExtOpenRouterSessionID      = "openrouter.session_id"
	ExtOllamaOptions            = "ollama.options"
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
	return OpenRouterRawExtensions{
		Models:        e.Raw(ExtOpenRouterModels),
		Route:         e.Raw(ExtOpenRouterRoute),
		Provider:      e.Raw(ExtOpenRouterProvider),
		ProviderPrefs: e.Raw(ExtOpenRouterProviderPrefs),
		Plugins:       e.Raw(ExtOpenRouterPlugins),
		Debug:         e.Raw(ExtOpenRouterDebug),
		Trace:         e.Raw(ExtOpenRouterTrace),
		SessionID:     e.Raw(ExtOpenRouterSessionID),
	}
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
