package unified

import (
	"encoding/json"
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
