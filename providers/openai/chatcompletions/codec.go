package chatcompletions

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

func encodeRequest(req unified.Request) (requestWire, error) {
	out := requestWire{
		Model:       req.Model,
		MaxTokens:   req.MaxOutputTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        append([]string(nil), req.Stop...),
		Stream:      req.Stream,
		User:        req.User,
	}
	for _, inst := range req.Instructions {
		out.Messages = append(out.Messages, messageWire{Role: "system", Content: contentText(inst.Content), Name: inst.Name})
	}
	for _, msg := range req.Messages {
		wire := messageWire{Role: string(msg.Role), Content: contentText(msg.Content), Name: msg.Name}
		for _, call := range msg.ToolCalls {
			wire.ToolCalls = append(wire.ToolCalls, toolCallWire{
				ID:   call.ID,
				Type: "function",
				Function: toolCallFunctionWire{
					Name:      call.Name,
					Arguments: string(call.Arguments),
				},
			})
		}
		if msg.Role == unified.RoleTool && len(msg.ToolResults) > 0 {
			wire.Role = "tool"
			wire.ToolCallID = msg.ToolResults[0].ToolCallID
			wire.Content = contentText(msg.ToolResults[0].Content)
		}
		out.Messages = append(out.Messages, wire)
	}
	for _, tool := range req.Tools {
		if tool.Kind != "" && tool.Kind != unified.ToolKindFunction {
			continue
		}
		out.Tools = append(out.Tools, toolWire{Type: "function", Function: functionToolWire{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.InputSchema,
		}})
	}
	if req.ToolChoice != nil {
		out.ToolChoice = encodeToolChoice(*req.ToolChoice)
	}
	return out, nil
}

func encodeToolChoice(choice unified.ToolChoice) any {
	switch choice.Mode {
	case unified.ToolChoiceAuto, "":
		return "auto"
	case unified.ToolChoiceNone:
		return "none"
	case unified.ToolChoiceRequired, unified.ToolChoiceAny:
		return "required"
	case unified.ToolChoiceTool:
		return map[string]any{"type": "function", "function": map[string]string{"name": choice.Name}}
	default:
		return nil
	}
}

func contentText(parts []unified.ContentPart) string {
	var out []string
	for _, part := range parts {
		if text, ok := part.(unified.TextPart); ok {
			out = append(out, text.Text)
		}
	}
	return strings.Join(out, "\n")
}

type streamDecoder struct {
	id      string
	model   string
	started bool
}

func (d *streamDecoder) push(raw []byte) ([]unified.Event, error) {
	frame, err := transport.ParseSSEFrame(raw)
	if err != nil {
		return nil, err
	}
	if string(frame.Data) == "[DONE]" {
		return nil, nil
	}
	var chunk chunkWire
	if err := json.Unmarshal(frame.Data, &chunk); err != nil {
		return nil, err
	}
	if chunk.Error != nil {
		return []unified.Event{unified.ErrorEvent{Err: &unified.APIError{Type: chunk.Error.Type, Code: chunk.Error.Code, Message: chunk.Error.Message}}}, nil
	}
	if chunk.ID != "" {
		d.id = chunk.ID
	}
	if chunk.Model != "" {
		d.model = chunk.Model
	}
	var out []unified.Event
	if !d.started {
		d.started = true
		out = append(out, unified.MessageStartEvent{ID: d.id, Model: d.model, Role: unified.RoleAssistant})
		out = append(out, unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindText})
	}
	for _, choice := range chunk.Choices {
		if choice.Delta.Content != "" {
			out = append(out, unified.TextDeltaEvent{Index: 0, Text: choice.Delta.Content})
		}
		for _, call := range choice.Delta.ToolCalls {
			if call.Function.Name != "" {
				out = append(out, unified.ToolCallStartEvent{Index: choice.Index, ID: call.ID, Name: call.Function.Name})
			}
			if call.Function.Arguments != "" {
				out = append(out, unified.ToolCallArgsDeltaEvent{Index: choice.Index, ID: call.ID, Delta: call.Function.Arguments})
			}
		}
		if choice.FinishReason != "" {
			out = append(out, unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText})
			out = append(out, unified.CompletedEvent{FinishReason: mapFinishReason(choice.FinishReason), MessageID: d.id})
			out = append(out, unified.MessageDoneEvent{ID: d.id})
		}
	}
	if chunk.Usage != nil {
		out = append(out, unified.UsageEvent{
			InputTokens:  chunk.Usage.PromptTokens,
			OutputTokens: chunk.Usage.CompletionTokens,
			TotalTokens:  chunk.Usage.TotalTokens,
		})
	}
	return out, nil
}

func mapFinishReason(reason string) unified.FinishReason {
	switch reason {
	case "stop":
		return unified.FinishReasonStop
	case "length":
		return unified.FinishReasonLength
	case "tool_calls":
		return unified.FinishReasonToolCall
	default:
		return unified.FinishReasonUnknown
	}
}

func requireTextResponse(resp unified.Response) error {
	if len(resp.Content) == 0 {
		return fmt.Errorf("response has no content")
	}
	return nil
}
