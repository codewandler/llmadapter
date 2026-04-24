package adapt

import "fmt"

type ApiKind string

const (
	ApiOpenAIResponses             ApiKind = "openai.responses"
	ApiOpenAIChatCompletions       ApiKind = "openai.chat_completions"
	ApiAnthropicMessages           ApiKind = "anthropic.messages"
	ApiGeminiGenerateContent       ApiKind = "gemini.generate_content"
	ApiOllamaChat                  ApiKind = "ollama.chat"
	ApiOllamaGenerate              ApiKind = "ollama.generate"
	ApiOpenRouterChatCompletions   ApiKind = "openrouter.chat_completions"
	ApiOpenRouterResponses         ApiKind = "openrouter.responses"
	ApiOpenRouterAnthropicMessages ApiKind = "openrouter.anthropic_messages"
	ApiMiniMaxChatCompletions      ApiKind = "minimax.chat_completions"
	ApiBedrockConverse             ApiKind = "bedrock.converse"
	ApiMistralChat                 ApiKind = "mistral.chat"
	ApiCohereChatV2                ApiKind = "cohere.chat_v2"

	// Backward-compatible aliases for the shorter names used in the original design draft.
	ApiOpenRouterChat     = ApiOpenRouterChatCompletions
	ApiOpenRouterMessages = ApiOpenRouterAnthropicMessages
)

type ApiFamily string

const (
	FamilyOpenAIResponses       ApiFamily = "openai.responses"
	FamilyOpenAIChatCompletions ApiFamily = "openai.chat_completions"
	FamilyAnthropicMessages     ApiFamily = "anthropic.messages"
	FamilyGeminiGenerateContent ApiFamily = "gemini.generate_content"
	FamilyOllamaChat            ApiFamily = "ollama.chat"
	FamilyBedrockConverse       ApiFamily = "bedrock.converse"
)

type MappingMode string

const (
	MappingModeStrict     MappingMode = "strict"
	MappingModeBestEffort MappingMode = "best_effort"
)

type Warning struct {
	Code    string `json:"code,omitempty"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message,omitempty"`
}

type UnsupportedFieldError struct {
	APIKind ApiKind
	Field   string
	Reason  string
}

func (e *UnsupportedFieldError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Reason == "" {
		return fmt.Sprintf("%s does not support field %s", e.APIKind, e.Field)
	}
	return fmt.Sprintf("%s does not support field %s: %s", e.APIKind, e.Field, e.Reason)
}
