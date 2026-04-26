package unified

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"unicode"
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
	ExtCodexInteractionMode       = "codex.interaction_mode"
	ExtCodexSessionID             = "codex.session_id"
	ExtCodexBranchID              = "codex.branch_id"
	ExtCodexBranchHeadID          = "codex.branch_head_id"
	ExtCodexInputBaseHash         = "codex.input_base_hash"
	ExtCodexParentResponseID      = "codex.parent_response_id"
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
	InteractionMode      InteractionMode
	SessionID            string
	BranchID             string
	BranchHeadID         string
	InputBaseHash        string
	ParentResponseID     string
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
	return OpenRouterRawExtensions(openRouterRawExtensionsFrom(e))
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
			if strings.TrimSpace(value) == "" {
				warnings = append(warnings, extensionWarning(key, errors.New("empty string")))
				return
			}
			*dest = value
		}
	}
	setString(ExtOpenAIPreviousResponseID, &out.PreviousResponseID)
	setString(ExtOpenAIPromptCacheKey, &out.PromptCacheKey)
	setString(ExtOpenAIPromptCacheRetention, &out.PromptCacheRetention)
	if out.PromptCacheRetention != "" && out.PromptCacheRetention != "24h" && out.PromptCacheRetention != "in_memory" {
		warnings = append(warnings, extensionWarning(ExtOpenAIPromptCacheRetention, errors.New(`must be "24h" or "in_memory"`)))
		out.PromptCacheRetention = ""
	}
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
	var warnings []WarningEvent
	out := OpenRouterExtensions{
		Models:        validatedRawExtension(e, ExtOpenRouterModels, rawStringArray, &warnings),
		Route:         validatedRawExtension(e, ExtOpenRouterRoute, rawObjectOrSafeString, &warnings),
		Provider:      validatedRawExtension(e, ExtOpenRouterProvider, rawProviderObject, &warnings),
		ProviderPrefs: validatedRawExtension(e, ExtOpenRouterProviderPrefs, rawObject, &warnings),
		Plugins:       validatedRawExtension(e, ExtOpenRouterPlugins, rawPluginArray, &warnings),
		Debug:         validatedRawExtension(e, ExtOpenRouterDebug, rawBoolOrObject, &warnings),
		Trace:         validatedRawExtension(e, ExtOpenRouterTrace, rawObject, &warnings),
		SessionID:     validatedRawExtension(e, ExtOpenRouterSessionID, rawSafeString, &warnings),
	}
	return out, warnings
}

func rawStringArray(v any) bool {
	values, ok := v.([]any)
	if !ok {
		return false
	}
	for _, item := range values {
		if !rawSafeString(item) {
			return false
		}
	}
	return true
}

func rawObject(v any) bool {
	_, ok := v.(map[string]any)
	return ok
}

func rawBoolOrObject(v any) bool {
	if _, ok := v.(bool); ok {
		return true
	}
	return rawObject(v)
}

func rawObjectOrSafeString(v any) bool {
	if rawObject(v) {
		return true
	}
	return rawSafeString(v)
}

func rawSafeString(v any) bool {
	value, ok := v.(string)
	return ok && validExtensionString(value)
}

func rawProviderObject(v any) bool {
	values, ok := v.(map[string]any)
	if !ok {
		return false
	}
	for _, key := range []string{"order", "ignore", "only"} {
		value, ok := values[key]
		if !ok {
			continue
		}
		if !rawStringArray(value) {
			return false
		}
	}
	return true
}

func rawPluginArray(v any) bool {
	values, ok := v.([]any)
	if !ok {
		return false
	}
	for _, item := range values {
		plugin, ok := item.(map[string]any)
		if !ok {
			return false
		}
		if id, exists := plugin["id"]; exists && !rawSafeString(id) {
			return false
		}
	}
	return true
}

func validatedRawExtension(e Extensions, key string, valid func(any) bool, warnings *[]WarningEvent) json.RawMessage {
	raw := e.Raw(key)
	if len(raw) == 0 {
		return nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		*warnings = append(*warnings, extensionWarning(key, err))
		return nil
	}
	if !valid(value) {
		*warnings = append(*warnings, extensionWarning(key, errors.New("unexpected JSON type")))
		return nil
	}
	return raw
}

func openRouterRawExtensionsFrom(e Extensions) OpenRouterExtensions {
	return OpenRouterExtensions{
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

func SetOpenRouterExtensions(e *Extensions, value OpenRouterExtensions) error {
	return SetOpenRouterRawExtensions(e, OpenRouterRawExtensions(value))
}

func AnthropicExtensionsFrom(e Extensions) (AnthropicExtensions, []WarningEvent) {
	var out AnthropicExtensions
	var warnings []WarningEvent
	if value, ok, err := GetExtension[[]string](e, ExtAnthropicBetas); err != nil {
		warnings = append(warnings, extensionWarning(ExtAnthropicBetas, err))
	} else if ok {
		for _, beta := range value {
			if !validBetaValue(beta) {
				warnings = append(warnings, extensionWarning(ExtAnthropicBetas, errors.New("must be a non-empty beta header value without whitespace, comma, or control characters")))
				continue
			}
			out.Betas = append(out.Betas, beta)
		}
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
			if !validExtensionString(value) {
				warnings = append(warnings, extensionWarning(key, errors.New("must be a non-empty string without control characters")))
				return
			}
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
	var interactionMode string
	setString(ExtCodexInteractionMode, &interactionMode)
	out.InteractionMode = InteractionMode(interactionMode)
	if out.InteractionMode != "" && !ValidInteractionMode(out.InteractionMode) {
		warnings = append(warnings, extensionWarning(ExtCodexInteractionMode, errors.New(`must be "auto", "one_shot", or "session"`)))
		out.InteractionMode = ""
	}
	setString(ExtCodexSessionID, &out.SessionID)
	setString(ExtCodexBranchID, &out.BranchID)
	setString(ExtCodexBranchHeadID, &out.BranchHeadID)
	setString(ExtCodexInputBaseHash, &out.InputBaseHash)
	setString(ExtCodexParentResponseID, &out.ParentResponseID)
	setString(ExtCodexWindowID, &out.WindowID)
	setString(ExtCodexTurnState, &out.TurnState)
	setString(ExtCodexTurnMetadata, &out.TurnMetadata)
	if out.TurnMetadata != "" && !validJSONObject(out.TurnMetadata) {
		warnings = append(warnings, extensionWarning(ExtCodexTurnMetadata, errors.New("must be a JSON object string")))
		out.TurnMetadata = ""
	}
	setString(ExtCodexParentThreadID, &out.ParentThreadID)
	setBool(ExtCodexSubagent, &out.Subagent)
	setBool(ExtCodexMemgenRequest, &out.MemgenRequest)
	setBool(ExtCodexIncludeTimingMetrics, &out.IncludeTimingMetrics)
	return out, warnings
}

func validBetaValue(value string) bool {
	if strings.TrimSpace(value) != value || value == "" || strings.Contains(value, ",") {
		return false
	}
	for _, r := range value {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func validExtensionString(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func validJSONObject(value string) bool {
	var obj map[string]any
	return json.Unmarshal([]byte(value), &obj) == nil
}

func SetCodexExtensions(e *Extensions, value CodexExtensions) error {
	if value.InteractionMode != "" {
		if !ValidInteractionMode(value.InteractionMode) {
			return errors.New(`invalid codex interaction mode: must be "auto", "one_shot", or "session"`)
		}
		if err := e.Set(ExtCodexInteractionMode, string(value.InteractionMode)); err != nil {
			return err
		}
	}
	if value.SessionID != "" {
		if err := e.Set(ExtCodexSessionID, value.SessionID); err != nil {
			return err
		}
	}
	if value.BranchID != "" {
		if err := e.Set(ExtCodexBranchID, value.BranchID); err != nil {
			return err
		}
	}
	if value.BranchHeadID != "" {
		if err := e.Set(ExtCodexBranchHeadID, value.BranchHeadID); err != nil {
			return err
		}
	}
	if value.InputBaseHash != "" {
		if err := e.Set(ExtCodexInputBaseHash, value.InputBaseHash); err != nil {
			return err
		}
	}
	if value.ParentResponseID != "" {
		if err := e.Set(ExtCodexParentResponseID, value.ParentResponseID); err != nil {
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
