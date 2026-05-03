package converse

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/codewandler/llmadapter/unified"
)

type builtRequest struct {
	input    *bedrockruntime.ConverseStreamInput
	warnings []unified.WarningEvent
}

const betaInterleavedThinking = "interleaved-thinking-2025-05-14"

func (c *Client) resolveModel(model string) (string, error) {
	if hasInferenceProfilePrefix(model) {
		return model, nil
	}
	profile, ok := inferenceProfiles[model]
	if !ok {
		return model, nil
	}
	if containsPrefix(profile.Prefixes, c.prefix) {
		return c.prefix + "." + model, nil
	}
	if containsPrefix(profile.Prefixes, PrefixGlobal) {
		return PrefixGlobal + "." + model, nil
	}
	return "", fmt.Errorf("model %q is not available in region %s through a known inference profile; available prefixes: %v", model, c.region, profile.Prefixes)
}

func (c *Client) buildRequest(req unified.Request, resolvedModel string) (builtRequest, error) {
	out := builtRequest{input: &bedrockruntime.ConverseStreamInput{ModelId: aws.String(resolvedModel)}}

	var system []types.SystemContentBlock
	for _, inst := range req.Instructions {
		for _, part := range inst.Content {
			textPart, ok := part.(unified.TextPart)
			if !ok {
				out.warnings = append(out.warnings, warning("unsupported_field_dropped", "instructions.content", "non-text instruction content part was dropped"))
				continue
			}
			if textPart.Text != "" {
				system = append(system, &types.SystemContentBlockMemberText{Value: textPart.Text})
			}
			if textPart.CacheControl != nil && cacheEnabled(req.CachePolicy) {
				system = append(system, &types.SystemContentBlockMemberCachePoint{Value: cachePointBlock(textPart.CacheControl)})
			}
		}
	}
	var messages []types.Message
	for _, msg := range req.Messages {
		switch msg.Role {
		case unified.RoleSystem:
			for _, part := range msg.Content {
				textPart, ok := part.(unified.TextPart)
				if !ok {
					out.warnings = append(out.warnings, warning("unsupported_field_dropped", "messages.content", "non-text system content part was dropped"))
					continue
				}
				if textPart.Text != "" {
					system = append(system, &types.SystemContentBlockMemberText{Value: textPart.Text})
				}
				if textPart.CacheControl != nil && cacheEnabled(req.CachePolicy) {
					system = append(system, &types.SystemContentBlockMemberCachePoint{Value: cachePointBlock(textPart.CacheControl)})
				}
			}
		case unified.RoleUser:
			content, warnings, err := contentBlocks(msg.Content, "messages.content", req.CachePolicy)
			if err != nil {
				return out, err
			}
			out.warnings = append(out.warnings, warnings...)
			if len(content) > 0 {
				messages = append(messages, types.Message{Role: types.ConversationRoleUser, Content: content})
			}
		case unified.RoleAssistant:
			content, warnings, err := assistantContentBlocks(msg)
			if err != nil {
				return out, err
			}
			out.warnings = append(out.warnings, warnings...)
			if len(content) > 0 {
				messages = append(messages, types.Message{Role: types.ConversationRoleAssistant, Content: content})
			}
		case unified.RoleTool:
			content, err := toolResultBlocks(msg.ToolResults)
			if err != nil {
				return out, err
			}
			if len(content) > 0 {
				messages = append(messages, types.Message{Role: types.ConversationRoleUser, Content: content})
			}
		default:
			out.warnings = append(out.warnings, warning("unsupported_field_dropped", "messages.role", fmt.Sprintf("unsupported role %q was dropped", msg.Role)))
		}
	}
	if len(system) > 0 {
		out.input.System = system
	}
	out.input.Messages = messages

	if len(req.Tools) > 0 {
		toolConfig, warnings, err := toolConfiguration(req)
		if err != nil {
			return out, err
		}
		out.warnings = append(out.warnings, warnings...)
		out.input.ToolConfig = toolConfig
	}
	if req.MaxOutputTokens != nil || req.Temperature != nil || req.TopP != nil || len(req.Stop) > 0 {
		cfg := &types.InferenceConfiguration{}
		if req.MaxOutputTokens != nil {
			cfg.MaxTokens = aws.Int32(int32(*req.MaxOutputTokens))
		}
		if req.Temperature != nil {
			cfg.Temperature = aws.Float32(float32(*req.Temperature))
		}
		if req.TopP != nil {
			cfg.TopP = aws.Float32(float32(*req.TopP))
		}
		if len(req.Stop) > 0 {
			cfg.StopSequences = append([]string(nil), req.Stop...)
		}
		out.input.InferenceConfig = cfg
	}
	additional := map[string]any{}
	if req.TopK != nil {
		additional["top_k"] = *req.TopK
	}
	if req.Reasoning != nil {
		if isAdaptiveReasoningModel(resolvedModel) && req.Reasoning.MaxTokens == nil && req.Reasoning.Effort != "" {
			additional["reasoning_config"] = map[string]any{"type": "adaptive"}
			additional["output_config"] = map[string]any{"effort": string(req.Reasoning.Effort)}
		} else {
			budget := reasoningBudget(*req.Reasoning)
			additional["reasoning_config"] = map[string]any{
				"type":          "enabled",
				"budget_tokens": budget,
			}
		}
	}
	if isClaudeModel(resolvedModel) {
		additional["anthropic_beta"] = []string{betaInterleavedThinking}
	}
	if len(additional) > 0 {
		doc, err := toDocument(additional)
		if err != nil {
			return out, fmt.Errorf("marshal additional Bedrock Converse fields: %w", err)
		}
		out.input.AdditionalModelRequestFields = doc
	}
	return out, nil
}

func contentBlocks(parts []unified.ContentPart, field string, cachePolicy unified.CachePolicy) ([]types.ContentBlock, []unified.WarningEvent, error) {
	var blocks []types.ContentBlock
	var warnings []unified.WarningEvent
	for _, part := range parts {
		switch p := part.(type) {
		case unified.TextPart:
			if p.Text != "" {
				blocks = append(blocks, &types.ContentBlockMemberText{Value: p.Text})
			}
			if p.CacheControl != nil && cacheEnabled(cachePolicy) {
				blocks = append(blocks, &types.ContentBlockMemberCachePoint{Value: cachePointBlock(p.CacheControl)})
			}
		default:
			warnings = append(warnings, warning("unsupported_field_dropped", field, "non-text content part was dropped"))
		}
	}
	return blocks, warnings, nil
}

func cacheEnabled(policy unified.CachePolicy) bool {
	return policy == unified.CachePolicyOn || policy == unified.CachePolicyAuto
}

func cachePointBlock(control *unified.CacheControl) types.CachePointBlock {
	block := types.CachePointBlock{Type: types.CachePointTypeDefault}
	if control != nil {
		switch control.TTL {
		case "5m":
			block.Ttl = types.CacheTTLFiveMinutes
		case "1h":
			block.Ttl = types.CacheTTLOneHour
		}
	}
	return block
}

func isAdaptiveReasoningModel(model string) bool {
	return stripInferenceProfilePrefix(model) == ModelClaudeOpus47
}

func assistantContentBlocks(msg unified.Message) ([]types.ContentBlock, []unified.WarningEvent, error) {
	var blocks []types.ContentBlock
	var warnings []unified.WarningEvent
	for _, part := range msg.Content {
		switch p := part.(type) {
		case unified.TextPart:
			if p.Text != "" {
				blocks = append(blocks, &types.ContentBlockMemberText{Value: p.Text})
			}
		case unified.ReasoningPart:
			if p.Text != "" || p.Signature != "" {
				blocks = append(blocks, &types.ContentBlockMemberReasoningContent{
					Value: &types.ReasoningContentBlockMemberReasoningText{
						Value: types.ReasoningTextBlock{
							Text:      aws.String(p.Text),
							Signature: aws.String(p.Signature),
						},
					},
				})
			}
		default:
			warnings = append(warnings, warning("unsupported_field_dropped", "messages.content", "unsupported assistant content part was dropped"))
		}
	}
	for _, call := range msg.ToolCalls {
		doc, err := jsonDocument(call.Arguments)
		if err != nil {
			return nil, warnings, fmt.Errorf("marshal tool call arguments: %w", err)
		}
		blocks = append(blocks, &types.ContentBlockMemberToolUse{
			Value: types.ToolUseBlock{
				ToolUseId: aws.String(call.ID),
				Name:      aws.String(call.Name),
				Input:     doc,
			},
		})
	}
	return blocks, warnings, nil
}

func toolResultBlocks(results []unified.ToolResult) ([]types.ContentBlock, error) {
	var blocks []types.ContentBlock
	for _, result := range results {
		status := types.ToolResultStatusSuccess
		if result.IsError {
			status = types.ToolResultStatusError
		}
		blocks = append(blocks, &types.ContentBlockMemberToolResult{
			Value: types.ToolResultBlock{
				ToolUseId: aws.String(result.ToolCallID),
				Content: []types.ToolResultContentBlock{
					&types.ToolResultContentBlockMemberText{Value: contentText(result.Content)},
				},
				Status: status,
			},
		})
	}
	return blocks, nil
}

func toolConfiguration(req unified.Request) (*types.ToolConfiguration, []unified.WarningEvent, error) {
	var tools []types.Tool
	var warnings []unified.WarningEvent
	for _, tool := range req.Tools {
		if tool.Kind != "" && tool.Kind != unified.ToolKindFunction {
			warnings = append(warnings, warning("unsupported_field_dropped", "tools.kind", "unsupported tool kind was dropped"))
			continue
		}
		doc, err := jsonDocument(tool.InputSchema)
		if err != nil {
			return nil, warnings, fmt.Errorf("marshal tool schema: %w", err)
		}
		tools = append(tools, &types.ToolMemberToolSpec{
			Value: types.ToolSpecification{
				Name:        aws.String(tool.Name),
				Description: aws.String(tool.Description),
				InputSchema: &types.ToolInputSchemaMemberJson{Value: doc},
			},
		})
	}
	if len(tools) == 0 {
		return nil, warnings, nil
	}
	cfg := &types.ToolConfiguration{Tools: tools}
	if req.ToolChoice != nil {
		switch req.ToolChoice.Mode {
		case unified.ToolChoiceAuto, "":
			cfg.ToolChoice = &types.ToolChoiceMemberAuto{Value: types.AutoToolChoice{}}
		case unified.ToolChoiceAny, unified.ToolChoiceRequired:
			cfg.ToolChoice = &types.ToolChoiceMemberAny{Value: types.AnyToolChoice{}}
		case unified.ToolChoiceNone:
			return nil, warnings, nil
		case unified.ToolChoiceTool:
			if req.Reasoning != nil {
				cfg.ToolChoice = &types.ToolChoiceMemberAuto{Value: types.AutoToolChoice{}}
				warnings = append(warnings, warning("unsupported_field_adjusted", "tool_choice", "forced tool_choice is incompatible with Bedrock Converse reasoning and was sent as auto"))
				break
			}
			cfg.ToolChoice = &types.ToolChoiceMemberTool{Value: types.SpecificToolChoice{Name: aws.String(req.ToolChoice.Name)}}
		default:
			warnings = append(warnings, warning("unsupported_field_dropped", "tool_choice", "unsupported tool_choice was dropped"))
		}
	}
	return cfg, warnings, nil
}

func contentText(parts []unified.ContentPart) string {
	var b strings.Builder
	for _, part := range parts {
		if text, ok := part.(unified.TextPart); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

func reasoningBudget(reasoning unified.ReasoningConfig) int {
	if reasoning.MaxTokens != nil && *reasoning.MaxTokens > 0 {
		return *reasoning.MaxTokens
	}
	switch reasoning.Effort {
	case unified.ReasoningEffortLow:
		return 1024
	case unified.ReasoningEffortMedium, "":
		return 4096
	case unified.ReasoningEffortHigh:
		return 8192
	case unified.ReasoningEffortMax:
		return 31999
	default:
		return 4096
	}
}

func isClaudeModel(model string) bool {
	model = stripInferenceProfilePrefix(model)
	return strings.Contains(model, "anthropic.claude")
}

func jsonDocument(raw json.RawMessage) (document.Interface, error) {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return document.NewLazyDocument(value), nil
}

func toDocument(value any) (document.Interface, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return jsonDocument(raw)
}

func warning(code, field, message string) unified.WarningEvent {
	return unified.WarningEvent{Code: code, Message: message, Source: string(defaultAPIKind), Meta: map[string]any{"field": field}}
}
