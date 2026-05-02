package messages

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/unified"
)

type Codec struct{}

func (Codec) ApiKind() adapt.ApiKind {
	return adapt.ApiAnthropicMessages
}

func (Codec) NewEventDecoder() adapt.EventDecoder[Event, unified.Event] {
	return NewEventDecoder()
}

func (Codec) EncodeRequest(ctx context.Context, req *adapt.Request) (MessageRequest, error) {
	ureq := req.Unified
	if ureq.MaxOutputTokens == nil {
		return MessageRequest{}, &adapt.UnsupportedFieldError{APIKind: adapt.ApiAnthropicMessages, Field: "max_output_tokens", Reason: "Anthropic requires max_tokens"}
	}
	if err := unsupported(req, "seed", ureq.Seed != nil); err != nil {
		return MessageRequest{}, err
	}
	if err := unsupported(req, "response_format", ureq.ResponseFormat != nil); err != nil {
		return MessageRequest{}, err
	}

	out := MessageRequest{
		Model:         ureq.Model,
		MaxTokens:     *ureq.MaxOutputTokens,
		Temperature:   ureq.Temperature,
		TopP:          ureq.TopP,
		TopK:          ureq.TopK,
		StopSequences: append([]string(nil), ureq.Stop...),
		Stream:        ureq.Stream,
	}
	if err := applyReasoning(req, &out); err != nil {
		return MessageRequest{}, err
	}
	applyAnthropicExtensions(req, &out, ureq.Extensions)

	system, err := encodeInstructions(ureq.Instructions)
	if err != nil {
		return MessageRequest{}, err
	}
	out.System = system

	for _, msg := range ureq.Messages {
		if msg.Role == unified.RoleSystem {
			blocks := encodeSystemContentParts(msg.Content)
			if len(blocks) != 0 {
				if out.System == nil {
					out.System = &SystemContent{}
				}
				out.System.Append(blocks...)
			}
			continue
		}
		wire, err := encodeMessage(req, msg)
		if err != nil {
			return MessageRequest{}, err
		}
		out.Messages = append(out.Messages, wire)
	}

	for _, tool := range ureq.Tools {
		unsupportedKind := tool.Kind != "" && tool.Kind != unified.ToolKindFunction
		if err := unsupported(req, "tools.kind", unsupportedKind); err != nil {
			return MessageRequest{}, err
		}
		if unsupportedKind {
			continue
		}
		out.Tools = append(out.Tools, ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
			Cache:       encodeCacheControl(tool.CacheControl),
		})
	}
	applyCachePolicy(&out, ureq)
	if ureq.ToolChoice != nil {
		choice, err := encodeToolChoice(req, *ureq.ToolChoice)
		if err != nil {
			return MessageRequest{}, err
		}
		out.ToolChoice = choice
	}
	if req.SourceAPI == adapt.ApiOpenRouterAnthropicMessages {
		applyOpenRouterExtensions(req, &out, ureq.Extensions)
	}
	return out, nil
}

func applyCachePolicy(out *MessageRequest, req unified.Request) {
	cache := cacheControlForPolicy(req)
	if cache == nil {
		return
	}
	if out.System != nil {
		out.System.ApplyCacheToLastText(cache)
	}
	applyCacheToLastMessageBlock(out.Messages, cache)
	for i := len(out.Tools) - 1; i >= 0; i-- {
		if out.Tools[i].Cache == nil {
			out.Tools[i].Cache = cache
		}
		break
	}
}

func applyCacheToLastMessageBlock(messages []InputMessage, cache *CacheControl) bool {
	if cache == nil {
		return false
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if len(messages[i].Content) == 0 {
			continue
		}
		last := len(messages[i].Content) - 1
		if messages[i].Content[last].Cache == nil {
			messages[i].Content[last].Cache = cache
		}
		return true
	}
	return false
}

func cacheControlForPolicy(req unified.Request) *CacheControl {
	switch req.CachePolicy {
	case unified.CachePolicyOn, unified.CachePolicyAuto:
		ttl := req.CacheTTL
		if ttl == "" {
			ttl = "1h"
		}
		return &CacheControl{Type: "ephemeral", TTL: ttl}
	default:
		return nil
	}
}

func applyReasoning(req *adapt.Request, out *MessageRequest) error {
	reasoning := req.Unified.Reasoning
	if reasoning == nil {
		return nil
	}
	if reasoning.MaxTokens == nil && reasoning.Effort != "" && supportsAnthropicOutputEffort(req.Unified.Model) {
		out.Thinking = &ThinkingConfig{Type: "adaptive"}
		if out.OutputConfig == nil {
			out.OutputConfig = &OutputConfig{}
		}
		out.OutputConfig.Effort = string(reasoning.Effort)
		return nil
	}
	budget := thinkingBudget(*reasoning, out.MaxTokens)
	if budget < 1024 {
		return &adapt.UnsupportedFieldError{APIKind: adapt.ApiAnthropicMessages, Field: "reasoning.max_tokens", Reason: "Anthropic thinking requires at least 1024 budget tokens and max_tokens greater than the budget"}
	}
	if out.MaxTokens <= budget {
		out.MaxTokens = budget + 1
		req.AddWarning("field_coerced", "max_output_tokens", "max_output_tokens was increased because Anthropic thinking budget must be less than max_tokens")
	}
	one := 1.0
	if out.Temperature == nil || *out.Temperature != one {
		out.Temperature = &one
		req.AddWarning("field_coerced", "temperature", "temperature was set to 1 because Anthropic extended thinking requires temperature 1")
	}
	if out.TopK != nil {
		if err := unsupported(req, "top_k", true); err != nil {
			return err
		}
		out.TopK = nil
	}
	out.Thinking = &ThinkingConfig{Type: "enabled", BudgetTokens: budget}
	return nil
}

func thinkingBudget(reasoning unified.ReasoningConfig, maxTokens int) int {
	if reasoning.MaxTokens != nil {
		return *reasoning.MaxTokens
	}
	switch reasoning.Effort {
	case unified.ReasoningEffortLow:
		return 1024
	case unified.ReasoningEffortHigh:
		if maxTokens > 8192 {
			return 8192
		}
	case unified.ReasoningEffortMedium:
		if maxTokens > 4096 {
			return 4096
		}
	case unified.ReasoningEffortMax:
		if maxTokens > 1024 {
			return maxTokens - 1
		}
	}
	if maxTokens > 2048 {
		return maxTokens / 2
	}
	return 1024
}

func supportsAnthropicOutputEffort(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(model, "claude-sonnet-4-6"):
		return true
	case strings.Contains(model, "claude-opus-4-5"):
		return true
	case strings.Contains(model, "claude-opus-4-6"):
		return true
	case strings.Contains(model, "claude-opus-4-7"):
		return true
	case strings.Contains(model, "mythos"):
		return true
	default:
		return false
	}
}

func applyAnthropicExtensions(req *adapt.Request, out *MessageRequest, extensions unified.Extensions) {
	values, warnings := unified.AnthropicExtensionsFrom(extensions)
	for _, warning := range warnings {
		key, _ := warning.Meta["key"].(string)
		req.AddWarning(warning.Code, key, warning.Message)
	}
	out.Betas = append(out.Betas, values.Betas...)
	if len(values.ContextManagement) != 0 {
		out.ContextManagement = append(out.ContextManagement[:0], values.ContextManagement...)
	}
}

func applyOpenRouterExtensions(req *adapt.Request, out *MessageRequest, extensions unified.Extensions) {
	raw, warnings := unified.OpenRouterExtensionsFrom(extensions)
	for _, warning := range warnings {
		key, _ := warning.Meta["key"].(string)
		req.AddWarning(warning.Code, key, warning.Message)
	}
	out.OpenRouterModels = raw.Models
	out.OpenRouterRoute = raw.Route
	out.OpenRouterProvider = raw.Provider
	out.OpenRouterPrefs = raw.ProviderPrefs
	out.OpenRouterPlugins = raw.Plugins
	out.OpenRouterDebug = raw.Debug
	out.OpenRouterTrace = raw.Trace
	out.OpenRouterSessionID = raw.SessionID
}

func unsupported(req *adapt.Request, field string, condition bool) error {
	if !condition {
		return nil
	}
	if req.MappingMode == adapt.MappingModeStrict {
		return &adapt.UnsupportedFieldError{APIKind: adapt.ApiAnthropicMessages, Field: field, Reason: "not supported by first Anthropic mapping"}
	}
	req.AddWarning("unsupported_field_dropped", field, fmt.Sprintf("%s is not supported by %s and was dropped", field, adapt.ApiAnthropicMessages))
	return nil
}

func encodeInstructions(instructions []unified.Instruction) (*SystemContent, error) {
	var blocks []ContentBlock
	for _, inst := range instructions {
		blocks = append(blocks, encodeSystemContentParts(inst.Content)...)
	}
	return NewSystemContent(blocks...), nil
}

func encodeSystemContentParts(parts []unified.ContentPart) []ContentBlock {
	var blocks []ContentBlock
	for _, part := range parts {
		switch p := part.(type) {
		case unified.TextPart:
			if p.Text != "" {
				blocks = append(blocks, ContentBlock{Type: "text", Text: p.Text, Cache: encodeCacheControl(p.CacheControl)})
			}
		case unified.ReasoningPart:
			if p.Text != "" {
				blocks = append(blocks, ContentBlock{Type: "text", Text: p.Text})
			}
		case unified.RefusalPart:
			if p.Text != "" {
				blocks = append(blocks, ContentBlock{Type: "text", Text: p.Text})
			}
		}
	}
	return blocks
}

func encodeMessage(req *adapt.Request, msg unified.Message) (InputMessage, error) {
	role := string(msg.Role)
	if msg.Role == unified.RoleTool {
		role = "user"
	}
	if role != "user" && role != "assistant" {
		return InputMessage{}, fmt.Errorf("unsupported Anthropic message role %q", msg.Role)
	}

	var blocks []ContentBlock
	for _, part := range msg.Content {
		block, err := encodeContentPart(req, part)
		if err != nil {
			return InputMessage{}, err
		}
		if block.Type == "" {
			continue
		}
		blocks = append(blocks, block)
	}
	for _, call := range msg.ToolCalls {
		input := call.Arguments
		if len(input) == 0 {
			input = json.RawMessage(`{}`)
		}
		blocks = append(blocks, ContentBlock{Type: "tool_use", ID: call.ID, Name: call.Name, Input: input})
	}
	for _, result := range msg.ToolResults {
		blocks = append(blocks, ContentBlock{
			Type:      "tool_result",
			ToolUseID: result.ToolCallID,
			Content:   contentText(result.Content),
			IsError:   result.IsError,
		})
	}
	return InputMessage{Role: role, Content: blocks}, nil
}

func encodeContentPart(req *adapt.Request, part unified.ContentPart) (ContentBlock, error) {
	switch p := part.(type) {
	case unified.TextPart:
		return ContentBlock{Type: "text", Text: p.Text, Cache: encodeCacheControl(p.CacheControl)}, nil
	case unified.ImagePart:
		src := BlockSource{MediaType: p.Source.MIMEType}
		switch p.Source.Kind {
		case unified.BlobSourceBase64:
			src.Type = "base64"
			src.Data = p.Source.Base64
		case unified.BlobSourceURL:
			src.Type = "url"
			src.URL = p.Source.URL
		default:
			if err := unsupported(req, "content.image.source", true); err != nil {
				return ContentBlock{}, err
			}
			return ContentBlock{}, nil
		}
		return ContentBlock{Type: "image", Source: &src}, nil
	case unified.ReasoningPart:
		if p.Text == "" && p.Signature == "" {
			return ContentBlock{}, nil
		}
		return ContentBlock{Type: "thinking", Thinking: p.Text, Signature: p.Signature}, nil
	default:
		if err := unsupported(req, "content", true); err != nil {
			return ContentBlock{}, err
		}
		return ContentBlock{}, nil
	}
}

func encodeCacheControl(cache *unified.CacheControl) *CacheControl {
	if cache == nil {
		return nil
	}
	return &CacheControl{Type: string(cache.Type), TTL: cache.TTL}
}

func encodeToolChoice(req *adapt.Request, choice unified.ToolChoice) (*ToolChoiceWire, error) {
	switch choice.Mode {
	case unified.ToolChoiceAuto, "":
		return &ToolChoiceWire{Type: "auto"}, nil
	case unified.ToolChoiceAny, unified.ToolChoiceRequired:
		return &ToolChoiceWire{Type: "any"}, nil
	case unified.ToolChoiceTool:
		return &ToolChoiceWire{Type: "tool", Name: choice.Name}, nil
	case unified.ToolChoiceNone:
		if err := unsupported(req, "tool_choice.none", true); err != nil {
			return nil, err
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown tool choice mode %q", choice.Mode)
	}
}

func contentText(parts []unified.ContentPart) string {
	var out []string
	for _, part := range parts {
		switch p := part.(type) {
		case unified.TextPart:
			out = append(out, p.Text)
		case unified.ReasoningPart:
			out = append(out, p.Text)
		case unified.RefusalPart:
			out = append(out, p.Text)
		}
	}
	return strings.Join(out, "\n")
}
