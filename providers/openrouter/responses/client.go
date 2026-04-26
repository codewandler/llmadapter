package responses

import (
	"encoding/json"
	"fmt"

	openairesponses "github.com/codewandler/llmadapter/providers/openai/responses"
	"github.com/codewandler/llmadapter/unified"
)

const defaultBaseURL = "https://openrouter.ai/api"

func NewClient(opts ...Option) (unified.Client, error) {
	cfg := config{baseURL: defaultBaseURL, warningSource: "openrouter.responses"}
	for _, opt := range opts {
		opt.applyOpenRouterResponses(&cfg)
	}
	if cfg.apiKey == "" {
		return nil, fmt.Errorf("openrouter API key is required")
	}
	openAIOptions := []openairesponses.Option{
		openairesponses.WithAPIKey(cfg.apiKey),
		openairesponses.WithBaseURL(cfg.baseURL),
		openairesponses.WithWarningSource(cfg.warningSource),
		openairesponses.WithBodyMutator(openRouterBodyMutator(cfg.warningSource)),
	}
	if cfg.transport != nil {
		openAIOptions = append(openAIOptions, openairesponses.WithTransport(cfg.transport))
	}
	return openairesponses.NewClient(openAIOptions...)
}

func openRouterBodyMutator(source string) func(unified.Request, []byte) ([]byte, []unified.WarningEvent, error) {
	return func(req unified.Request, body []byte) ([]byte, []unified.WarningEvent, error) {
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(body, &payload); err != nil {
			return body, nil, nil
		}
		var warnings []unified.WarningEvent
		raw, extensionWarnings := unified.OpenRouterExtensionsFrom(req.Extensions)
		for _, warning := range extensionWarnings {
			warnings = append(warnings, unified.WarningEvent{
				Code:    "invalid_extension_dropped",
				Message: warning.Message,
				Source:  source,
				Meta:    warning.Meta,
			})
		}
		setRaw(payload, "models", raw.Models)
		setRaw(payload, "route", raw.Route)
		setRaw(payload, "provider", raw.Provider)
		setRaw(payload, "provider_preferences", raw.ProviderPrefs)
		setRaw(payload, "plugins", raw.Plugins)
		setRaw(payload, "debug", raw.Debug)
		setRaw(payload, "trace", raw.Trace)
		setRaw(payload, "session_id", raw.SessionID)
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, err
		}
		return encoded, warnings, nil
	}
}

func setRaw(payload map[string]json.RawMessage, key string, value json.RawMessage) {
	if len(value) == 0 {
		delete(payload, key)
		return
	}
	payload[key] = value
}
