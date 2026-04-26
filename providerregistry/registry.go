package providerregistry

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/codewandler/llmadapter/adapt"
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

type Descriptor struct {
	Type                 string                   `json:"type"`
	APIKind              adapt.ApiKind            `json:"api_kind"`
	Family               adapt.ApiFamily          `json:"family"`
	Capabilities         router.CapabilitySet     `json:"capabilities"`
	ConsumerContinuation unified.ContinuationMode `json:"consumer_continuation,omitempty"`
	InternalContinuation unified.ContinuationMode `json:"internal_continuation,omitempty"`
	Transport            unified.TransportKind    `json:"transport,omitempty"`
	DefaultAPIKeyEnvs    []string                 `json:"default_api_key_envs,omitempty"`
	DefaultModelEnv      string                   `json:"default_model_env,omitempty"`
	DefaultModel         string                   `json:"default_model,omitempty"`
	Factory              ClientFactory            `json:"-"`
}

type ClientFactory func(ClientConfig) (unified.Client, error)

type ClientConfig struct {
	Type    string
	APIKey  string
	BaseURL string
}

var descriptors = []Descriptor{
	{
		Type:                 "anthropic",
		APIKind:              adapt.ApiAnthropicMessages,
		Family:               adapt.FamilyAnthropicMessages,
		Capabilities:         router.CapabilitySet{Streaming: true, Tools: true, Vision: true, Reasoning: true, ReasoningDeltas: true, PromptCaching: true},
		ConsumerContinuation: unified.ContinuationReplay,
		InternalContinuation: unified.ContinuationReplay,
		Transport:            unified.TransportHTTPSSE,
		DefaultAPIKeyEnvs:    []string{"ANTHROPIC_API_KEY"},
		DefaultModelEnv:      "ANTHROPIC_MODEL",
		DefaultModel:         "claude-haiku-4-5-20251001",
		Factory:              func(cfg ClientConfig) (unified.Client, error) { return newAnthropicClient(cfg, false) },
	},
	{
		Type:                 "claude",
		APIKind:              adapt.ApiAnthropicMessages,
		Family:               adapt.FamilyAnthropicMessages,
		Capabilities:         router.CapabilitySet{Streaming: true, Tools: true, Vision: true, Reasoning: true, ReasoningDeltas: true, PromptCaching: true},
		ConsumerContinuation: unified.ContinuationReplay,
		InternalContinuation: unified.ContinuationReplay,
		Transport:            unified.TransportHTTPSSE,
		DefaultAPIKeyEnvs:    nil,
		DefaultModelEnv:      "CLAUDE_MODEL",
		DefaultModel:         "claude-haiku-4-5-20251001",
		Factory:              func(cfg ClientConfig) (unified.Client, error) { return newAnthropicClient(cfg, true) },
	},
	{
		Type:                 "openai_chat",
		APIKind:              adapt.ApiOpenAIChatCompletions,
		Family:               adapt.FamilyOpenAIChatCompletions,
		Capabilities:         router.CapabilitySet{Streaming: true, Tools: true, ParallelTools: true, Vision: true, JSONMode: true, JSONSchema: true},
		ConsumerContinuation: unified.ContinuationReplay,
		InternalContinuation: unified.ContinuationReplay,
		Transport:            unified.TransportHTTPSSE,
		DefaultAPIKeyEnvs:    []string{"OPENAI_API_KEY", "OPENAI_KEY"},
		DefaultModelEnv:      "OPENAI_MODEL",
		DefaultModel:         "gpt-4.1-mini",
		Factory:              newOpenAIChatClient,
	},
	{
		Type:                 "openai_responses",
		APIKind:              adapt.ApiOpenAIResponses,
		Family:               adapt.FamilyOpenAIResponses,
		Capabilities:         router.CapabilitySet{Streaming: true, Tools: true, ParallelTools: true, Vision: true, JSONMode: true, JSONSchema: true, Reasoning: true, ReasoningDeltas: true, ServerSideState: true, PromptCaching: true},
		ConsumerContinuation: unified.ContinuationPreviousResponseID,
		InternalContinuation: unified.ContinuationPreviousResponseID,
		Transport:            unified.TransportHTTPSSE,
		DefaultAPIKeyEnvs:    []string{"OPENAI_API_KEY", "OPENAI_KEY"},
		DefaultModelEnv:      "OPENAI_RESPONSES_MODEL",
		DefaultModel:         "gpt-4.1-mini",
		Factory:              newOpenAIResponsesClient,
	},
	{
		Type:                 "codex_responses",
		APIKind:              adapt.ApiCodexResponses,
		Family:               adapt.FamilyOpenAIResponses,
		Capabilities:         router.CapabilitySet{Streaming: true, Tools: true, ParallelTools: true, Reasoning: true, PromptCaching: true},
		ConsumerContinuation: unified.ContinuationReplay,
		InternalContinuation: unified.ContinuationReplay,
		Transport:            unified.TransportHTTPSSE,
		DefaultAPIKeyEnvs:    []string{codex.EnvAccessToken, codex.EnvOAuthToken},
		DefaultModelEnv:      codex.EnvModel,
		DefaultModel:         codex.DefaultModel,
		Factory:              newCodexResponsesClient,
	},
	{
		Type:                 "openrouter_chat",
		APIKind:              adapt.ApiOpenRouterChatCompletions,
		Family:               adapt.FamilyOpenAIChatCompletions,
		Capabilities:         router.CapabilitySet{Streaming: true, Tools: true, ParallelTools: true, Vision: true, JSONMode: true, JSONSchema: true},
		ConsumerContinuation: unified.ContinuationReplay,
		InternalContinuation: unified.ContinuationReplay,
		Transport:            unified.TransportHTTPSSE,
		DefaultAPIKeyEnvs:    []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
		DefaultModelEnv:      "OPENROUTER_MODEL",
		DefaultModel:         "openai/gpt-4.1-mini",
		Factory:              newOpenRouterChatClient,
	},
	{
		Type:                 "openrouter_responses",
		APIKind:              adapt.ApiOpenRouterResponses,
		Family:               adapt.FamilyOpenAIResponses,
		Capabilities:         router.CapabilitySet{Streaming: true, Tools: true, ParallelTools: true, Vision: true, JSONMode: true, JSONSchema: true, Reasoning: true, ReasoningDeltas: true, PromptCaching: true},
		ConsumerContinuation: unified.ContinuationReplay,
		InternalContinuation: unified.ContinuationReplay,
		Transport:            unified.TransportHTTPSSE,
		DefaultAPIKeyEnvs:    []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
		DefaultModelEnv:      "OPENROUTER_RESPONSES_MODEL",
		DefaultModel:         "openai/gpt-4.1-mini",
		Factory:              newOpenRouterResponsesClient,
	},
	{
		Type:                 "openrouter_messages",
		APIKind:              adapt.ApiOpenRouterAnthropicMessages,
		Family:               adapt.FamilyAnthropicMessages,
		Capabilities:         router.CapabilitySet{Streaming: true, Tools: true, Vision: true, Reasoning: true, ReasoningDeltas: true, PromptCaching: true},
		ConsumerContinuation: unified.ContinuationReplay,
		InternalContinuation: unified.ContinuationReplay,
		Transport:            unified.TransportHTTPSSE,
		DefaultAPIKeyEnvs:    []string{"OPENROUTER_API_KEY", "OPENROUTER_KEY"},
		DefaultModelEnv:      "OPENROUTER_MESSAGES_MODEL",
		DefaultModel:         "anthropic/claude-sonnet-4",
		Factory:              newOpenRouterMessagesClient,
	},
	{
		Type:                 "minimax_chat",
		APIKind:              adapt.ApiMiniMaxChatCompletions,
		Family:               adapt.FamilyOpenAIChatCompletions,
		Capabilities:         router.CapabilitySet{Streaming: true, Tools: true},
		ConsumerContinuation: unified.ContinuationReplay,
		InternalContinuation: unified.ContinuationReplay,
		Transport:            unified.TransportHTTPSSE,
		DefaultAPIKeyEnvs:    []string{"MINIMAX_API_KEY", "MINIMAX_KEY"},
		DefaultModelEnv:      "MINIMAX_MODEL",
		DefaultModel:         "MiniMax-M2.7",
		Factory:              newMiniMaxChatClient,
	},
	{
		Type:                 "minimax_messages",
		APIKind:              adapt.ApiMiniMaxAnthropicMessages,
		Family:               adapt.FamilyAnthropicMessages,
		Capabilities:         router.CapabilitySet{Streaming: true, Tools: true, Reasoning: true, ReasoningDeltas: true, PromptCaching: true},
		ConsumerContinuation: unified.ContinuationReplay,
		InternalContinuation: unified.ContinuationReplay,
		Transport:            unified.TransportHTTPSSE,
		DefaultAPIKeyEnvs:    []string{"MINIMAX_API_KEY", "MINIMAX_KEY"},
		DefaultModelEnv:      "MINIMAX_MESSAGES_MODEL",
		DefaultModel:         "MiniMax-M2.7",
		Factory:              newMiniMaxMessagesClient,
	},
}

func List() []Descriptor {
	out := append([]Descriptor(nil), descriptors...)
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

func Lookup(providerType string) (Descriptor, bool) {
	for _, descriptor := range descriptors {
		if descriptor.Type == providerType {
			return descriptor, true
		}
	}
	return Descriptor{}, false
}

func NewClient(cfg ClientConfig) (unified.Client, error) {
	providerType := strings.TrimSpace(cfg.Type)
	if providerType == "" {
		return nil, fmt.Errorf("provider type is required")
	}
	descriptor, ok := Lookup(providerType)
	if !ok {
		return nil, fmt.Errorf("unsupported provider type %q", providerType)
	}
	if descriptor.Factory == nil {
		return nil, fmt.Errorf("provider type %q has no client factory", providerType)
	}
	cfg.Type = providerType
	return descriptor.Factory(cfg)
}

func newAnthropicClient(cfg ClientConfig, claudeCompatible bool) (unified.Client, error) {
	if cfg.APIKey == "" && !claudeCompatible {
		return nil, fmt.Errorf("provider type %q requires api key", cfg.Type)
	}

	opts := []anthropic.Option{}
	if claudeCompatible {
		opts = append(opts,
			anthropic.WithClaudeHeaders(),
			anthropic.WithClaudeCodePreflight(),
			anthropic.WithSystemCacheControl(""),
		)
	}

	if cfg.APIKey != "" {
		if claudeCompatible {
			opts = append(opts, anthropic.WithBearerTokenProvider(anthropic.NewStaticTokenProvider(anthropic.NewStaticBearerToken(cfg.APIKey))))
		} else {
			opts = append(opts, anthropic.WithAPIKey(cfg.APIKey))
		}
	} else if claudeCompatible {
		opts = append(opts, anthropic.WithLocalClaudeOAuth())
	}

	opts = append(opts, anthropic.WithRequestProcessor(requestProcessorFunc(func(ctx context.Context, req *adapt.Request) error {
		req.Unified.Stream = true
		return nil
	})))

	if cfg.BaseURL != "" {
		opts = append(opts, anthropic.WithBaseURL(cfg.BaseURL))
	}

	return anthropic.NewClient(opts...)
}

func newOpenAIChatClient(cfg ClientConfig) (unified.Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("provider type %q requires api key", cfg.Type)
	}
	opts := []openai.Option{openai.WithAPIKey(cfg.APIKey)}
	if cfg.BaseURL != "" {
		opts = append(opts, openai.WithBaseURL(cfg.BaseURL))
	}
	return openai.NewClient(opts...)
}

func newOpenAIResponsesClient(cfg ClientConfig) (unified.Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("provider type %q requires api key", cfg.Type)
	}
	opts := []openairesponses.Option{openairesponses.WithAPIKey(cfg.APIKey)}
	if cfg.BaseURL != "" {
		opts = append(opts, openairesponses.WithBaseURL(cfg.BaseURL))
	}
	return openairesponses.NewClient(opts...)
}

func newCodexResponsesClient(cfg ClientConfig) (unified.Client, error) {
	opts := []codex.Option{}
	if cfg.APIKey != "" {
		opts = append(opts, codex.WithAccessToken(cfg.APIKey))
	}
	if cfg.BaseURL != "" {
		opts = append(opts, codex.WithBaseURL(cfg.BaseURL))
	}
	return codex.NewClient(opts...)
}

func newOpenRouterChatClient(cfg ClientConfig) (unified.Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("provider type %q requires api key", cfg.Type)
	}
	opts := []openrouter.Option{openrouter.WithAPIKey(cfg.APIKey)}
	if cfg.BaseURL != "" {
		opts = append(opts, openrouter.WithBaseURL(cfg.BaseURL))
	}
	return openrouter.NewClient(opts...)
}

func newOpenRouterResponsesClient(cfg ClientConfig) (unified.Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("provider type %q requires api key", cfg.Type)
	}
	opts := []openrouterresponses.Option{openrouterresponses.WithAPIKey(cfg.APIKey)}
	if cfg.BaseURL != "" {
		opts = append(opts, openrouterresponses.WithBaseURL(cfg.BaseURL))
	}
	return openrouterresponses.NewClient(opts...)
}

func newOpenRouterMessagesClient(cfg ClientConfig) (unified.Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("provider type %q requires api key", cfg.Type)
	}
	opts := []openroutermessages.Option{openroutermessages.WithAPIKey(cfg.APIKey)}
	if cfg.BaseURL != "" {
		opts = append(opts, openroutermessages.WithBaseURL(cfg.BaseURL))
	}
	return openroutermessages.NewClient(opts...)
}

func newMiniMaxChatClient(cfg ClientConfig) (unified.Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("provider type %q requires api key", cfg.Type)
	}
	opts := []minimax.Option{minimax.WithAPIKey(cfg.APIKey)}
	if cfg.BaseURL != "" {
		opts = append(opts, minimax.WithBaseURL(cfg.BaseURL))
	}
	return minimax.NewClient(opts...)
}

func newMiniMaxMessagesClient(cfg ClientConfig) (unified.Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("provider type %q requires api key", cfg.Type)
	}
	opts := []minimaxmessages.Option{minimaxmessages.WithAPIKey(cfg.APIKey)}
	if cfg.BaseURL != "" {
		opts = append(opts, minimaxmessages.WithBaseURL(cfg.BaseURL))
	}
	return minimaxmessages.NewClient(opts...)
}

type requestProcessorFunc func(context.Context, *adapt.Request) error

func (f requestProcessorFunc) ProcessRequest(ctx context.Context, req *adapt.Request) error {
	return f(ctx, req)
}
