package responses

import (
	"encoding/json"
	"strconv"
	"strings"

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

func encodeRequest(req unified.Request) (requestWire, []mappingWarning) {
	var warnings []mappingWarning
	out := requestWire{
		Model:           req.Model,
		MaxOutputTokens: req.MaxOutputTokens,
		Temperature:     req.Temperature,
		TopP:            req.TopP,
		TopK:            req.TopK,
		Stream:          req.Stream,
		User:            req.User,
	}
	out.Instructions = contentText(instructionParts(req.Instructions), "instructions.content", &warnings)
	for i, msg := range req.Messages {
		item := inputItemWire{Type: "message", Role: string(msg.Role), ID: msg.ID}
		if msg.Role == unified.RoleAssistant {
			item.Status = "completed"
		}
		for j, part := range msg.Content {
			item.Content = appendContentPart(item.Content, part, msg.Role, "messages."+strconv.Itoa(i)+".content."+strconv.Itoa(j), &warnings)
		}
		if len(item.Content) > 0 {
			out.Input = append(out.Input, item)
		}
		for _, call := range msg.ToolCalls {
			out.Input = append(out.Input, inputItemWire{
				Type:      "function_call",
				ID:        functionCallItemID(call.ID),
				CallID:    call.ID,
				Name:      call.Name,
				Arguments: string(call.Arguments),
			})
		}
		for j, result := range msg.ToolResults {
			out.Input = append(out.Input, inputItemWire{
				Type:   "function_call_output",
				CallID: result.ToolCallID,
				Output: contentText(result.Content, "messages."+strconv.Itoa(i)+".tool_results."+strconv.Itoa(j)+".content", &warnings),
			})
		}
	}
	for i, tool := range req.Tools {
		if tool.Kind != "" && tool.Kind != unified.ToolKindFunction {
			addWarning(&warnings, "tools."+strconv.Itoa(i)+".kind", "unsupported tool kind was dropped")
			continue
		}
		out.Tools = append(out.Tools, toolWire{
			Type:        "function",
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.InputSchema,
		})
	}
	if req.ToolChoice != nil {
		out.ToolChoice = encodeToolChoice(*req.ToolChoice)
	}
	if req.ResponseFormat != nil {
		responseFormat := encodeResponseFormat(*req.ResponseFormat)
		if responseFormat == nil {
			addWarning(&warnings, "response_format", "unsupported response format was dropped")
		} else {
			out.Text.Format = responseFormat
		}
	}
	applyOpenAIResponsesExtensions(&out, req.Extensions, &warnings)
	applyOpenRouterExtensions(&out, req.Extensions)
	return out, warnings
}

func appendContentPart(out []contentPartWire, part unified.ContentPart, role unified.Role, field string, warnings *[]mappingWarning) []contentPartWire {
	switch p := part.(type) {
	case unified.TextPart:
		partType := "input_text"
		if role == unified.RoleAssistant {
			partType = "output_text"
		}
		return append(out, contentPartWire{Type: partType, Text: p.Text})
	case unified.ImagePart:
		switch p.Source.Kind {
		case unified.BlobSourceURL:
			return append(out, contentPartWire{Type: "input_image", ImageURL: p.Source.URL})
		case unified.BlobSourceFileID:
			return append(out, contentPartWire{Type: "input_image", FileID: p.Source.FileID})
		case unified.BlobSourceBase64:
			return append(out, contentPartWire{Type: "input_image", ImageURL: "data:" + p.Source.MIMEType + ";base64," + p.Source.Base64})
		default:
			addWarning(warnings, field, "unsupported image source was dropped")
		}
	default:
		addWarning(warnings, field, "non-text content part was dropped")
	}
	return out
}

func functionCallItemID(id string) string {
	if strings.HasPrefix(id, "fc") {
		return id
	}
	return ""
}

func applyOpenRouterExtensions(out *requestWire, extensions unified.Extensions) {
	raw := unified.OpenRouterRawExtensionsFrom(extensions)
	out.OpenRouterModels = raw.Models
	out.OpenRouterRoute = raw.Route
	out.OpenRouterProvider = raw.Provider
	out.OpenRouterPrefs = raw.ProviderPrefs
	out.OpenRouterPlugins = raw.Plugins
	out.OpenRouterDebug = raw.Debug
	out.OpenRouterTrace = raw.Trace
	out.OpenRouterSessionID = raw.SessionID
}

func applyOpenAIResponsesExtensions(out *requestWire, extensions unified.Extensions, warnings *[]mappingWarning) {
	if value, ok, err := unified.GetExtension[string](extensions, unified.ExtOpenAIPreviousResponseID); err != nil {
		addWarning(warnings, unified.ExtOpenAIPreviousResponseID, "invalid previous_response_id extension was dropped")
	} else if ok {
		out.PreviousResponseID = value
	}
	if value, ok, err := unified.GetExtension[bool](extensions, unified.ExtOpenAIStore); err != nil {
		addWarning(warnings, unified.ExtOpenAIStore, "invalid store extension was dropped")
	} else if ok {
		out.Store = &value
	}
	if value, ok, err := unified.GetExtension[string](extensions, unified.ExtOpenAIPromptCacheKey); err != nil {
		addWarning(warnings, unified.ExtOpenAIPromptCacheKey, "invalid prompt_cache_key extension was dropped")
	} else if ok {
		out.PromptCacheKey = value
	}
	if value, ok, err := unified.GetExtension[string](extensions, unified.ExtOpenAIPromptCacheRetention); err != nil {
		addWarning(warnings, unified.ExtOpenAIPromptCacheRetention, "invalid prompt_cache_retention extension was dropped")
	} else if ok {
		out.PromptCacheRetention = value
	}
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
		return map[string]string{"type": "function", "name": choice.Name}
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
			"type":   "json_schema",
			"name":   name,
			"schema": format.Schema,
			"strict": format.Strict,
		}
	default:
		return nil
	}
}

func instructionParts(instructions []unified.Instruction) []unified.ContentPart {
	var out []unified.ContentPart
	for _, instruction := range instructions {
		out = append(out, instruction.Content...)
	}
	return out
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
	id             string
	model          string
	started        bool
	startedBlock   bool
	toolIDs        map[int]string
	toolNames      map[int]string
	toolArgs       map[int]json.RawMessage
	startedTools   map[int]bool
	completedTools map[int]bool
	doneTools      bool
}

func (d *streamDecoder) push(raw []byte) ([]unified.Event, error) {
	frame, err := transport.ParseSSEFrame(raw)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(frame.Data))) == 0 || string(frame.Data) == "[DONE]" {
		return nil, nil
	}
	var ev eventWire
	if err := json.Unmarshal(frame.Data, &ev); err != nil {
		return nil, err
	}
	ev.Raw = append(json.RawMessage(nil), frame.Data...)
	if ev.Error != nil {
		return []unified.Event{unified.ErrorEvent{Err: &unified.APIError{Type: ev.Error.Type, Code: ev.Error.Code, Message: ev.Error.Message}}}, nil
	}
	if ev.Response != nil && ev.Response.Error != nil {
		return []unified.Event{unified.ErrorEvent{Err: &unified.APIError{Type: ev.Response.Error.Type, Code: ev.Response.Error.Code, Message: ev.Response.Error.Message}}}, nil
	}
	if ev.Response != nil {
		if ev.Response.ID != "" {
			d.id = ev.Response.ID
		}
		if ev.Response.Model != "" {
			d.model = ev.Response.Model
		}
	}
	if ev.ResponseID != "" {
		d.id = ev.ResponseID
	}

	var out []unified.Event
	switch ev.Type {
	case "response.created":
		out = append(out, d.start()...)
	case "response.output_item.added":
		if ev.Item != nil && ev.Item.Type == "function_call" {
			out = append(out, d.startToolCall(ev.OutputIndex, ev.Item)...)
		}
	case "response.output_item.done":
		if ev.Item != nil && ev.Item.Type == "function_call" {
			out = append(out, d.startToolCall(ev.OutputIndex, ev.Item)...)
			out = append(out, d.doneToolCall(ev.OutputIndex, ev.Item.Arguments)...)
		}
	case "response.content_part.added":
		out = append(out, d.start()...)
		if ev.Part == nil || ev.Part.Type == "output_text" {
			out = append(out, d.startBlock()...)
		}
	case "response.content_part.delta", "response.output_text.delta":
		out = append(out, d.start()...)
		out = append(out, d.startBlock()...)
		out = append(out, unified.TextDeltaEvent{Index: 0, Text: ev.Delta})
	case "response.content_part.done":
		if d.startedBlock {
			out = append(out, unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText})
			d.startedBlock = false
		}
	case "response.function_call_arguments.done":
		out = append(out, d.doneToolCall(ev.OutputIndex, ev.Arguments)...)
	case "response.done", "response.completed":
		out = append(out, d.done(ev.Response)...)
	default:
		if ev.Response != nil && ev.Response.Status == "completed" {
			out = append(out, d.done(ev.Response)...)
		}
	}
	return out, nil
}

func (d *streamDecoder) start() []unified.Event {
	if d.started {
		return nil
	}
	d.started = true
	return []unified.Event{unified.MessageStartEvent{ID: d.id, Model: d.model, Role: unified.RoleAssistant}}
}

func (d *streamDecoder) startBlock() []unified.Event {
	if d.startedBlock {
		return nil
	}
	d.startedBlock = true
	return []unified.Event{unified.ContentBlockStartEvent{Index: 0, Kind: unified.ContentKindText}}
}

func (d *streamDecoder) startToolCall(index int, item *outputItemWire) []unified.Event {
	if item == nil {
		return nil
	}
	d.ensureToolMaps()
	if d.completedTools[index] {
		return nil
	}
	id := item.CallID
	if id == "" {
		id = item.ID
	}
	if id != "" {
		d.toolIDs[index] = id
	}
	if item.Name != "" {
		d.toolNames[index] = item.Name
	}
	if d.startedTools[index] {
		return nil
	}
	d.startedTools[index] = true
	return []unified.Event{
		unified.ContentBlockStartEvent{Index: index, Kind: unified.ContentKindToolCall, ID: d.toolIDs[index], Name: d.toolNames[index]},
		unified.ToolCallStartEvent{Index: index, ID: d.toolIDs[index], Name: d.toolNames[index]},
	}
}

func (d *streamDecoder) doneToolCall(index int, arguments string) []unified.Event {
	d.ensureToolMaps()
	if d.completedTools[index] {
		return nil
	}
	if !d.startedTools[index] {
		d.startedTools[index] = true
	}
	args := json.RawMessage(nil)
	if arguments != "" {
		args = json.RawMessage(arguments)
		d.toolArgs[index] = args
	} else if len(d.toolArgs[index]) > 0 {
		args = d.toolArgs[index]
	}
	d.doneTools = true
	d.completedTools[index] = true
	delete(d.startedTools, index)
	return []unified.Event{
		unified.ToolCallDoneEvent{Index: index, ID: d.toolIDs[index], Name: d.toolNames[index], Args: args},
		unified.ContentBlockDoneEvent{Index: index, Kind: unified.ContentKindToolCall},
	}
}

func (d *streamDecoder) ensureToolMaps() {
	if d.toolIDs == nil {
		d.toolIDs = make(map[int]string)
	}
	if d.toolNames == nil {
		d.toolNames = make(map[int]string)
	}
	if d.toolArgs == nil {
		d.toolArgs = make(map[int]json.RawMessage)
	}
	if d.startedTools == nil {
		d.startedTools = make(map[int]bool)
	}
	if d.completedTools == nil {
		d.completedTools = make(map[int]bool)
	}
}

func (d *streamDecoder) done(resp *responseWire) []unified.Event {
	out := d.start()
	if d.startedBlock {
		out = append(out, unified.ContentBlockDoneEvent{Index: 0, Kind: unified.ContentKindText})
		d.startedBlock = false
	}
	if resp != nil && resp.Usage != nil {
		out = append(out, unified.UsageEvent{
			Tokens: tokenItemsFromUsage(resp.Usage),
		})
	}
	reason := finishReason(resp)
	if d.doneTools && reason == unified.FinishReasonStop {
		reason = unified.FinishReasonToolCall
	}
	out = append(out, unified.CompletedEvent{FinishReason: reason, MessageID: d.id})
	out = append(out, unified.MessageDoneEvent{ID: d.id})
	return out
}

func tokenItemsFromUsage(usage *usageWire) unified.TokenItems {
	if usage == nil {
		return nil
	}
	cachedInput := 0
	if usage.InputTokensDetails != nil {
		cachedInput = usage.InputTokensDetails.CachedTokens
	}
	newInput := usage.InputTokens - cachedInput
	if newInput < 0 {
		newInput = 0
	}
	reasoningOutput := 0
	if usage.OutputTokensDetails != nil {
		reasoningOutput = usage.OutputTokensDetails.ReasoningTokens
	}
	output := usage.OutputTokens - reasoningOutput
	if output < 0 {
		output = 0
	}
	return unified.TokenItems{
		{Kind: unified.TokenKindInputNew, Count: newInput},
		{Kind: unified.TokenKindInputCacheRead, Count: cachedInput},
		{Kind: unified.TokenKindOutput, Count: output},
		{Kind: unified.TokenKindOutputReasoning, Count: reasoningOutput},
	}.NonZero()
}

func finishReason(resp *responseWire) unified.FinishReason {
	if resp == nil {
		return unified.FinishReasonStop
	}
	switch resp.Status {
	case "completed", "":
		return unified.FinishReasonStop
	case "incomplete":
		return unified.FinishReasonLength
	default:
		return unified.FinishReasonUnknown
	}
}
