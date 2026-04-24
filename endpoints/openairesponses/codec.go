package openairesponses

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/unified"
)

type Codec struct{}

func (Codec) DecodeHTTP(ctx context.Context, r *http.Request) (adapt.Request, error) {
	if r.Method != http.MethodPost {
		return adapt.Request{}, statusError(http.StatusMethodNotAllowed, "method_not_allowed", "only POST is supported")
	}
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return adapt.Request{}, err
	}
	var wire Request
	if err := json.Unmarshal(body, &wire); err != nil {
		return adapt.Request{}, statusError(http.StatusBadRequest, "invalid_json", err.Error())
	}
	ureq, warnings, err := decodeRequest(wire)
	if err != nil {
		return adapt.Request{}, err
	}
	return adapt.Request{
		SourceAPI: adapt.ApiOpenAIResponses,
		HTTP: &adapt.HTTPRequestInfo{
			Method:  r.Method,
			Path:    r.URL.Path,
			Query:   r.URL.Query(),
			Headers: r.Header.Clone(),
			Remote:  r.RemoteAddr,
		},
		RawBody:     body,
		Raw:         wire,
		Unified:     ureq,
		MappingMode: adapt.MappingModeBestEffort,
		Warnings:    warnings,
	}, nil
}

func (Codec) WriteEvents(ctx context.Context, w http.ResponseWriter, req adapt.Request, events <-chan unified.Event) error {
	if req.Unified.Stream {
		return writeStream(ctx, w, events)
	}
	resp, err := unified.Collect(ctx, events)
	if err != nil {
		return err
	}
	return writeJSON(w, http.StatusOK, responseFromUnified(resp))
}

func (Codec) WriteError(ctx context.Context, w http.ResponseWriter, err error) error {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := err.Error()
	var apiErr *unified.APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode != 0 {
			status = apiErr.StatusCode
		}
		if apiErr.Code != "" {
			code = apiErr.Code
		} else if apiErr.Type != "" {
			code = apiErr.Type
		}
		if apiErr.Message != "" {
			message = apiErr.Message
		}
	}
	var httpErr httpError
	if errors.As(err, &httpErr) {
		status = httpErr.status
		code = httpErr.code
		message = httpErr.message
	}
	return writeJSON(w, status, map[string]any{
		"error": ErrorBody{Type: code, Code: code, Message: message},
	})
}

func decodeRequest(wire Request) (unified.Request, []adapt.Warning, error) {
	if wire.Model == "" {
		return unified.Request{}, nil, statusError(http.StatusBadRequest, "missing_model", "model is required")
	}
	out := unified.Request{
		Model:           wire.Model,
		MaxOutputTokens: wire.MaxOutputTokens,
		Temperature:     wire.Temperature,
		TopP:            wire.TopP,
		Stream:          wire.Stream,
		User:            wire.User,
	}
	if wire.Instructions != "" {
		out.Instructions = append(out.Instructions, unified.Instruction{
			Kind:    unified.InstructionSystem,
			Content: []unified.ContentPart{unified.TextPart{Text: wire.Instructions}},
		})
	}
	var warnings []adapt.Warning
	for i, item := range wire.Input {
		messages, itemWarnings := decodeInputItem(item, "input."+strconv.Itoa(i))
		out.Messages = append(out.Messages, messages...)
		warnings = append(warnings, itemWarnings...)
	}
	for i, tool := range wire.Tools {
		if tool.Type != "function" {
			warnings = append(warnings, decodeWarning("tools."+strconv.Itoa(i)+".type", fmt.Sprintf("unsupported tool type %q was dropped", tool.Type)))
			continue
		}
		out.Tools = append(out.Tools, unified.Tool{
			Kind:        unified.ToolKindFunction,
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.Parameters,
		})
	}
	if len(wire.ToolChoice) > 0 {
		toolChoice, toolChoiceWarnings := decodeToolChoice(wire.ToolChoice, "tool_choice")
		out.ToolChoice = toolChoice
		warnings = append(warnings, toolChoiceWarnings...)
	}
	return out, warnings, nil
}

func decodeInputItem(item InputItem, field string) ([]unified.Message, []adapt.Warning) {
	switch item.Type {
	case "message":
		content, warnings := decodeContent(item.Content, field+".content")
		return []unified.Message{{
			ID:      item.ID,
			Role:    unified.Role(item.Role),
			Content: content,
		}}, warnings
	case "function_call":
		args := json.RawMessage(item.Arguments)
		if len(args) == 0 {
			args = json.RawMessage(`{}`)
		}
		return []unified.Message{{
			Role: unified.RoleAssistant,
			ToolCalls: []unified.ToolCall{{
				ID:        firstNonEmpty(item.CallID, item.ID),
				Name:      item.Name,
				Arguments: args,
			}},
		}}, nil
	case "function_call_output":
		return []unified.Message{{
			Role: unified.RoleTool,
			ToolResults: []unified.ToolResult{{
				ToolCallID: item.CallID,
				Content:    []unified.ContentPart{unified.TextPart{Text: item.Output}},
			}},
		}}, nil
	default:
		return nil, []adapt.Warning{decodeWarning(field+".type", fmt.Sprintf("unsupported input item type %q was dropped", item.Type))}
	}
}

func decodeContent(parts []ContentPart, field string) ([]unified.ContentPart, []adapt.Warning) {
	var out []unified.ContentPart
	var warnings []adapt.Warning
	for i, part := range parts {
		switch part.Type {
		case "input_text", "output_text", "text":
			out = append(out, unified.TextPart{Text: part.Text})
		default:
			warnings = append(warnings, decodeWarning(field+"."+strconv.Itoa(i)+".type", fmt.Sprintf("unsupported content part type %q was dropped", part.Type)))
		}
	}
	return out, warnings
}

func decodeToolChoice(raw json.RawMessage, field string) (*unified.ToolChoice, []adapt.Warning) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch s {
		case "auto":
			return &unified.ToolChoice{Mode: unified.ToolChoiceAuto}, nil
		case "none":
			return &unified.ToolChoice{Mode: unified.ToolChoiceNone}, nil
		case "required":
			return &unified.ToolChoice{Mode: unified.ToolChoiceRequired}, nil
		}
		return nil, []adapt.Warning{decodeWarning(field, fmt.Sprintf("unsupported tool_choice value %q was dropped", s))}
	}
	var obj struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Type == "function" {
		return &unified.ToolChoice{Mode: unified.ToolChoiceTool, Name: obj.Name}, nil
	}
	return nil, []adapt.Warning{decodeWarning(field, "unsupported tool_choice object was dropped")}
}

func decodeWarning(field, message string) adapt.Warning {
	return adapt.Warning{
		Code:    "unsupported_field_dropped",
		Field:   field,
		Message: message,
	}
}

func responseFromUnified(resp unified.Response) Response {
	return Response{
		ID:     resp.ID,
		Object: "response",
		Model:  resp.Model,
		Status: statusFromFinish(resp.FinishReason),
		Output: outputFromUnified(resp),
		Usage:  usageFromUnified(resp.Usage),
	}
}

func outputFromUnified(resp unified.Response) []OutputItem {
	var out []OutputItem
	var content []ContentPart
	for _, part := range resp.Content {
		if text, ok := part.(unified.TextPart); ok {
			content = append(content, ContentPart{Type: "output_text", Text: text.Text})
		}
	}
	if len(content) > 0 {
		out = append(out, OutputItem{Type: "message", Role: "assistant", Status: "completed", Content: content})
	}
	for _, call := range resp.ToolCalls {
		out = append(out, OutputItem{
			Type:      "function_call",
			ID:        call.ID,
			CallID:    call.ID,
			Name:      call.Name,
			Arguments: string(call.Arguments),
			Status:    "completed",
		})
	}
	return out
}

func usageFromUnified(usage unified.Usage) Usage {
	return Usage{InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens, TotalTokens: usage.TotalTokens}
}

func statusFromFinish(reason unified.FinishReason) string {
	switch reason {
	case unified.FinishReasonLength:
		return "incomplete"
	default:
		return "completed"
	}
}

func writeStream(ctx context.Context, w http.ResponseWriter, events <-chan unified.Event) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	state := streamState{}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			frames := state.push(ev)
			if errEvent, ok := ev.(unified.ErrorEvent); ok && errEvent.Err != nil {
				return errEvent.Err
			}
			for _, frame := range frames {
				if err := writeSSEData(w, frame); err != nil {
					return err
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
		}
	}
}

type streamState struct {
	id             string
	model          string
	textStarted    bool
	textDone       bool
	toolIDs        map[int]string
	toolNames      map[int]string
	toolArgs       map[int]string
	startedTools   map[int]bool
	completedTools map[int]bool
	usage          unified.Usage
	doneTools      bool
}

func (s *streamState) push(ev unified.Event) []Event {
	switch e := ev.(type) {
	case unified.MessageStartEvent:
		s.id = e.ID
		s.model = e.Model
		return []Event{{Type: "response.created", Response: &Response{ID: e.ID, Object: "response", Model: e.Model, Status: "in_progress"}}}
	case unified.ContentBlockStartEvent:
		if e.Kind == unified.ContentKindText {
			return s.startTextPart()
		}
		if e.Kind == unified.ContentKindToolCall {
			return s.startToolCall(e.Index, e.ID, e.Name)
		}
		return nil
	case unified.TextDeltaEvent:
		frames := s.startTextPart()
		frames = append(frames, Event{Type: "response.output_text.delta", ResponseID: s.id, OutputIndex: 0, ContentIndex: 0, Delta: e.Text})
		return frames
	case unified.ToolCallStartEvent:
		return s.startToolCall(e.Index, e.ID, e.Name)
	case unified.ToolCallArgsDeltaEvent:
		s.ensureToolMaps()
		s.toolArgs[e.Index] += e.Delta
		return nil
	case unified.ToolCallDoneEvent:
		return s.doneToolCall(e.Index, e.ID, e.Name, e.Args)
	case unified.ContentBlockDoneEvent:
		if e.Kind == unified.ContentKindText && s.textStarted && !s.textDone {
			s.textDone = true
			return []Event{{Type: "response.content_part.done", ResponseID: s.id, OutputIndex: 0, ContentIndex: 0, Part: &ContentPart{Type: "output_text"}}}
		}
		return nil
	case unified.UsageEvent:
		s.usage = unified.Usage{InputTokens: e.InputTokens, OutputTokens: e.OutputTokens, TotalTokens: e.TotalTokens}
		return nil
	case unified.CompletedEvent:
		resp := &Response{
			ID:     firstNonEmpty(e.MessageID, s.id),
			Object: "response",
			Model:  s.model,
			Status: statusFromFinish(e.FinishReason),
			Usage:  usageFromUnified(s.usage),
		}
		return []Event{{Type: "response.done", Response: resp}}
	default:
		return nil
	}
}

func (s *streamState) startTextPart() []Event {
	if s.textStarted {
		return nil
	}
	s.textStarted = true
	return []Event{
		{Type: "response.output_item.added", ResponseID: s.id, OutputIndex: 0, Item: &OutputItem{Type: "message", Role: "assistant", Status: "in_progress"}},
		{Type: "response.content_part.added", ResponseID: s.id, OutputIndex: 0, ContentIndex: 0, Part: &ContentPart{Type: "output_text"}},
	}
}

func (s *streamState) startToolCall(index int, id, name string) []Event {
	s.ensureToolMaps()
	if s.startedTools[index] {
		return nil
	}
	s.startedTools[index] = true
	s.toolIDs[index] = firstNonEmpty(id, s.toolIDs[index])
	s.toolNames[index] = firstNonEmpty(name, s.toolNames[index])
	return []Event{{Type: "response.output_item.added", ResponseID: s.id, OutputIndex: index, Item: &OutputItem{
		Type:   "function_call",
		ID:     s.toolIDs[index],
		CallID: s.toolIDs[index],
		Name:   s.toolNames[index],
		Status: "in_progress",
	}}}
}

func (s *streamState) doneToolCall(index int, id, name string, args json.RawMessage) []Event {
	s.ensureToolMaps()
	if s.completedTools[index] {
		return nil
	}
	if !s.startedTools[index] {
		_ = s.startToolCall(index, id, name)
	}
	if len(args) > 0 && s.toolArgs[index] == "" {
		s.toolArgs[index] = string(args)
	}
	s.completedTools[index] = true
	s.doneTools = true
	return []Event{{Type: "response.output_item.done", ResponseID: s.id, OutputIndex: index, Item: &OutputItem{
		Type:      "function_call",
		ID:        firstNonEmpty(id, s.toolIDs[index]),
		CallID:    firstNonEmpty(id, s.toolIDs[index]),
		Name:      firstNonEmpty(name, s.toolNames[index]),
		Arguments: s.toolArgs[index],
		Status:    "completed",
	}}}
}

func (s *streamState) ensureToolMaps() {
	if s.toolIDs == nil {
		s.toolIDs = make(map[int]string)
	}
	if s.toolNames == nil {
		s.toolNames = make(map[int]string)
	}
	if s.toolArgs == nil {
		s.toolArgs = make(map[int]string)
	}
	if s.startedTools == nil {
		s.startedTools = make(map[int]bool)
	}
	if s.completedTools == nil {
		s.completedTools = make(map[int]bool)
	}
}

func writeSSEData(w io.Writer, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", b)
	return err
}

func writeJSON(w http.ResponseWriter, status int, value any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(value)
}

type httpError struct {
	status  int
	code    string
	message string
}

func (e httpError) Error() string { return e.message }

func statusError(status int, code, message string) error {
	return httpError{status: status, code: code, message: message}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
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
