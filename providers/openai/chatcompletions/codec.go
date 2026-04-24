package chatcompletions

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

type mappingWarning struct {
	code    string
	field   string
	message string
}

func (w mappingWarning) event(source string) unified.WarningEvent {
	meta := map[string]any(nil)
	if w.field != "" {
		meta = map[string]any{"field": w.field}
	}
	return unified.WarningEvent{Code: w.code, Message: w.message, Source: source, Meta: meta}
}

func encodeRequest(req unified.Request) (requestWire, []mappingWarning, error) {
	return encodeRequestForAPI(req, adapt.ApiOpenAIChatCompletions)
}

func encodeRequestForAPI(req unified.Request, apiKind adapt.ApiKind) (requestWire, []mappingWarning, error) {
	var warnings []mappingWarning
	out := requestWire{
		Model:       req.Model,
		MaxTokens:   req.MaxOutputTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        append([]string(nil), req.Stop...),
		Stream:      req.Stream,
		User:        req.User,
	}
	for i, inst := range req.Instructions {
		out.Messages = append(out.Messages, messageWire{Role: "system", Content: contentText(inst.Content, "instructions."+strconv.Itoa(i)+".content", &warnings), Name: inst.Name})
	}
	for i, msg := range req.Messages {
		if msg.Role == unified.RoleTool && len(msg.ToolResults) > 0 {
			for j, result := range msg.ToolResults {
				out.Messages = append(out.Messages, messageWire{
					Role:       "tool",
					ToolCallID: result.ToolCallID,
					Name:       result.Name,
					Content:    contentText(result.Content, "messages."+strconv.Itoa(i)+".tool_results."+strconv.Itoa(j)+".content", &warnings),
				})
			}
			continue
		}
		wire := messageWire{Role: string(msg.Role), Content: encodeMessageContent(msg.Content, "messages."+strconv.Itoa(i)+".content", &warnings), Name: msg.Name}
		for _, call := range msg.ToolCalls {
			wire.ToolCalls = append(wire.ToolCalls, toolCallWire{
				Index: call.Index,
				ID:    call.ID,
				Type:  "function",
				Function: toolCallFunctionWire{
					Name:      call.Name,
					Arguments: string(call.Arguments),
				},
			})
		}
		out.Messages = append(out.Messages, wire)
	}
	for i, tool := range req.Tools {
		if tool.Kind != "" && tool.Kind != unified.ToolKindFunction {
			addWarning(&warnings, "tools."+strconv.Itoa(i)+".kind", "unsupported tool kind was dropped")
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
	if req.ResponseFormat != nil {
		responseFormat := encodeResponseFormat(*req.ResponseFormat)
		if responseFormat == nil {
			addWarning(&warnings, "response_format", "unsupported response format was dropped")
		} else {
			out.ResponseFormat = responseFormat
		}
	}
	if apiKind == adapt.ApiOpenRouterChatCompletions {
		applyOpenRouterExtensions(&out, req.Extensions)
	}
	return out, warnings, nil
}

func encodeMessageContent(parts []unified.ContentPart, field string, warnings *[]mappingWarning) any {
	if len(parts) == 0 {
		return nil
	}
	wireParts := make([]contentPartWire, 0, len(parts))
	textOnly := true
	var textParts []string
	for i, part := range parts {
		switch p := part.(type) {
		case unified.TextPart:
			wireParts = append(wireParts, contentPartWire{Type: "text", Text: p.Text})
			textParts = append(textParts, p.Text)
		case unified.ImagePart:
			textOnly = false
			switch p.Source.Kind {
			case unified.BlobSourceURL:
				wireParts = append(wireParts, contentPartWire{Type: "image_url", ImageURL: &imageURLPartWire{URL: p.Source.URL}})
			case unified.BlobSourceBase64:
				wireParts = append(wireParts, contentPartWire{Type: "image_url", ImageURL: &imageURLPartWire{URL: "data:" + p.Source.MIMEType + ";base64," + p.Source.Base64}})
			default:
				addWarning(warnings, field+"."+strconv.Itoa(i), "unsupported image source was dropped")
			}
		default:
			addWarning(warnings, field+"."+strconv.Itoa(i), "non-text content part was dropped")
		}
	}
	if textOnly {
		return strings.Join(textParts, "\n")
	}
	return wireParts
}

func applyOpenRouterExtensions(out *requestWire, extensions unified.Extensions) {
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

func encodeResponseFormat(format unified.ResponseFormat) any {
	switch format.Kind {
	case unified.ResponseFormatText, "":
		return map[string]string{"type": "text"}
	case unified.ResponseFormatJSON:
		return map[string]string{"type": "json_object"}
	case unified.ResponseFormatJSONSchema:
		name := format.Name
		if name == "" {
			name = "response"
		}
		return map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   name,
				"schema": format.Schema,
				"strict": format.Strict,
			},
		}
	default:
		return nil
	}
}

func contentText(parts []unified.ContentPart, field string, warnings *[]mappingWarning) string {
	var out []string
	for i, part := range parts {
		if text, ok := part.(unified.TextPart); ok {
			out = append(out, text.Text)
			continue
		}
		addWarning(warnings, field+"."+strconv.Itoa(i), "non-text content part was dropped")
	}
	return strings.Join(out, "\n")
}

func addWarning(warnings *[]mappingWarning, field, message string) {
	if warnings == nil {
		return
	}
	*warnings = append(*warnings, mappingWarning{code: "unsupported_field_dropped", field: field, message: message})
}

type streamDecoder struct {
	id           string
	model        string
	started      bool
	toolIDs      map[int]string
	toolNames    map[int]string
	toolArgs     map[int]*strings.Builder
	startedTools map[int]bool
}

func (d *streamDecoder) push(raw []byte) ([]unified.Event, error) {
	frame, err := transport.ParseSSEFrame(raw)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(frame.Data))) == 0 || string(frame.Data) == "[DONE]" {
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
			toolIndex := call.Index
			if call.ID != "" {
				d.ensureToolMaps()
				d.toolIDs[toolIndex] = call.ID
			}
			if call.Function.Name != "" {
				d.ensureToolMaps()
				d.toolNames[toolIndex] = call.Function.Name
			}
			if !d.startedTools[toolIndex] && (call.ID != "" || call.Function.Name != "") {
				d.ensureToolMaps()
				d.startedTools[toolIndex] = true
				out = append(out, unified.ContentBlockStartEvent{Index: toolIndex, Kind: unified.ContentKindToolCall, ID: d.toolIDs[toolIndex], Name: d.toolNames[toolIndex]})
				out = append(out, unified.ToolCallStartEvent{Index: toolIndex, ID: d.toolIDs[toolIndex], Name: d.toolNames[toolIndex]})
			}
			if call.Function.Arguments != "" {
				d.ensureToolMaps()
				if d.toolArgs[toolIndex] == nil {
					d.toolArgs[toolIndex] = &strings.Builder{}
				}
				d.toolArgs[toolIndex].WriteString(call.Function.Arguments)
				out = append(out, unified.ToolCallArgsDeltaEvent{Index: toolIndex, ID: d.toolIDs[toolIndex], Delta: call.Function.Arguments})
			}
		}
		if choice.FinishReason != "" {
			toolDone := d.finishToolCalls()
			out = append(out, toolDone...)
			out = append(out, unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText})
			finishReason := mapFinishReason(choice.FinishReason)
			if len(toolDone) > 0 && finishReason == unified.FinishReasonStop {
				finishReason = unified.FinishReasonToolCall
			}
			out = append(out, unified.CompletedEvent{FinishReason: finishReason, MessageID: d.id})
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

func (d *streamDecoder) ensureToolMaps() {
	if d.toolIDs == nil {
		d.toolIDs = make(map[int]string)
	}
	if d.toolNames == nil {
		d.toolNames = make(map[int]string)
	}
	if d.toolArgs == nil {
		d.toolArgs = make(map[int]*strings.Builder)
	}
	if d.startedTools == nil {
		d.startedTools = make(map[int]bool)
	}
}

func (d *streamDecoder) finishToolCalls() []unified.Event {
	if len(d.startedTools) == 0 {
		return nil
	}
	indexes := make([]int, 0, len(d.startedTools))
	for index := range d.startedTools {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	out := make([]unified.Event, 0, len(indexes)*2)
	for _, index := range indexes {
		args := json.RawMessage(nil)
		if d.toolArgs[index] != nil && d.toolArgs[index].Len() > 0 {
			args = json.RawMessage(d.toolArgs[index].String())
		}
		out = append(out, unified.ToolCallDoneEvent{Index: index, ID: d.toolIDs[index], Name: d.toolNames[index], Args: args})
		out = append(out, unified.ContentBlockDoneEvent{Index: index, Kind: unified.ContentKindToolCall})
		delete(d.startedTools, index)
		delete(d.toolIDs, index)
		delete(d.toolNames, index)
		delete(d.toolArgs, index)
	}
	return out
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
