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

	system, err := encodeInstructions(ureq.Instructions)
	if err != nil {
		return MessageRequest{}, err
	}
	out.System = system

	for _, msg := range ureq.Messages {
		if msg.Role == unified.RoleSystem {
			text := contentText(msg.Content)
			if text != "" {
				if out.System != "" {
					out.System += "\n"
				}
				out.System += text
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
		})
	}
	if ureq.ToolChoice != nil {
		choice, err := encodeToolChoice(req, *ureq.ToolChoice)
		if err != nil {
			return MessageRequest{}, err
		}
		out.ToolChoice = choice
	}
	if req.SourceAPI == adapt.ApiOpenRouterAnthropicMessages {
		applyOpenRouterExtensions(&out, ureq.Extensions)
	}
	return out, nil
}

func applyOpenRouterExtensions(out *MessageRequest, extensions unified.Extensions) {
	out.OpenRouterModels = rawExtension(extensions, unified.ExtOpenRouterModels)
	out.OpenRouterRoute = rawExtension(extensions, unified.ExtOpenRouterRoute)
	out.OpenRouterProvider = rawExtension(extensions, unified.ExtOpenRouterProvider)
	out.OpenRouterPrefs = rawExtension(extensions, unified.ExtOpenRouterProviderPrefs)
	out.OpenRouterPlugins = rawExtension(extensions, unified.ExtOpenRouterPlugins)
	out.OpenRouterDebug = rawExtension(extensions, unified.ExtOpenRouterDebug)
	out.OpenRouterTrace = rawExtension(extensions, unified.ExtOpenRouterTrace)
	out.OpenRouterSessionID = rawExtension(extensions, unified.ExtOpenRouterSessionID)
}

func rawExtension(extensions unified.Extensions, key string) json.RawMessage {
	raw, ok, err := unified.GetExtension[json.RawMessage](extensions, key)
	if err != nil || !ok || len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
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

func encodeInstructions(instructions []unified.Instruction) (string, error) {
	var parts []string
	for _, inst := range instructions {
		text := contentText(inst.Content)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n"), nil
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
		return ContentBlock{Type: "text", Text: p.Text}, nil
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
	default:
		if err := unsupported(req, "content", true); err != nil {
			return ContentBlock{}, err
		}
		return ContentBlock{}, nil
	}
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
