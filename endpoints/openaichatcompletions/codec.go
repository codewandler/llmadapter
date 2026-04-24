package openaichatcompletions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

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
		SourceAPI: adapt.ApiOpenAIChatCompletions,
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
		"error": map[string]any{
			"message": message,
			"type":    code,
			"code":    code,
		},
	})
}

func decodeRequest(wire Request) (unified.Request, []adapt.Warning, error) {
	if wire.Model == "" {
		return unified.Request{}, nil, statusError(http.StatusBadRequest, "missing_model", "model is required")
	}
	maxTokens := wire.MaxCompletionTokens
	if maxTokens == nil {
		maxTokens = wire.MaxTokens
	}
	stop, warnings := decodeStop(wire.Stop, "stop")
	out := unified.Request{
		Model:           wire.Model,
		MaxOutputTokens: maxTokens,
		Temperature:     wire.Temperature,
		TopP:            wire.TopP,
		Stop:            stop,
		Stream:          wire.Stream,
		User:            wire.User,
	}
	for i, msg := range wire.Messages {
		field := "messages." + strconv.Itoa(i)
		umsg, instructions, msgWarnings, err := decodeMessage(msg, field)
		if err != nil {
			return unified.Request{}, nil, err
		}
		warnings = append(warnings, msgWarnings...)
		out.Instructions = append(out.Instructions, instructions...)
		if umsg.Role != "" {
			out.Messages = append(out.Messages, umsg)
		}
	}
	for i, tool := range wire.Tools {
		if tool.Type != "function" {
			warnings = append(warnings, decodeWarning("tools."+strconv.Itoa(i)+".type", fmt.Sprintf("unsupported tool type %q was dropped", tool.Type)))
			continue
		}
		out.Tools = append(out.Tools, unified.Tool{
			Kind:        unified.ToolKindFunction,
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: tool.Function.Parameters,
		})
	}
	if len(wire.ToolChoice) > 0 {
		toolChoice, toolChoiceWarnings := decodeToolChoice(wire.ToolChoice, "tool_choice")
		out.ToolChoice = toolChoice
		warnings = append(warnings, toolChoiceWarnings...)
	}
	return out, warnings, nil
}

func decodeMessage(msg Message, field string) (unified.Message, []unified.Instruction, []adapt.Warning, error) {
	content, warnings, err := decodeContent(msg.Content, field+".content")
	if err != nil {
		return unified.Message{}, nil, nil, err
	}
	switch msg.Role {
	case "system", "developer":
		kind := unified.InstructionSystem
		if msg.Role == "developer" {
			kind = unified.InstructionDeveloper
		}
		return unified.Message{}, []unified.Instruction{{Kind: kind, Content: content, Name: msg.Name}}, warnings, nil
	case "user", "assistant":
		out := unified.Message{Role: unified.Role(msg.Role), Name: msg.Name, Content: content}
		for i, call := range msg.ToolCalls {
			out.ToolCalls = append(out.ToolCalls, unified.ToolCall{
				ID:        call.ID,
				Name:      call.Function.Name,
				Arguments: json.RawMessage(call.Function.Arguments),
				Index:     i,
			})
		}
		return out, nil, warnings, nil
	case "tool":
		return unified.Message{
			Role: unified.RoleTool,
			ToolResults: []unified.ToolResult{{
				ToolCallID: msg.ToolCallID,
				Name:       msg.Name,
				Content:    content,
			}},
		}, nil, warnings, nil
	default:
		return unified.Message{}, nil, nil, statusError(http.StatusBadRequest, "unsupported_role", fmt.Sprintf("unsupported role %q", msg.Role))
	}
}

func decodeContent(raw json.RawMessage, field string) ([]unified.ContentPart, []adapt.Warning, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return []unified.ContentPart{unified.TextPart{Text: text}}, nil, nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, nil, statusError(http.StatusBadRequest, "unsupported_content", "content must be a string or text-part array")
	}
	out := make([]unified.ContentPart, 0, len(parts))
	var warnings []adapt.Warning
	for i, part := range parts {
		if part.Type == "text" {
			out = append(out, unified.TextPart{Text: part.Text})
			continue
		}
		warnings = append(warnings, decodeWarning(field+"."+strconv.Itoa(i)+".type", fmt.Sprintf("unsupported content part type %q was dropped", part.Type)))
	}
	return out, warnings, nil
}

func decodeStop(value any, field string) ([]string, []adapt.Warning) {
	switch v := value.(type) {
	case string:
		return []string{v}, nil
	case []any:
		var out []string
		var warnings []adapt.Warning
		for i, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
				continue
			}
			warnings = append(warnings, decodeWarning(field+"."+strconv.Itoa(i), "non-string stop sequence was dropped"))
		}
		return out, warnings
	case nil:
		return nil, nil
	default:
		return nil, []adapt.Warning{decodeWarning(field, "unsupported stop value was dropped")}
	}
}

func decodeToolChoice(raw json.RawMessage, field string) (*unified.ToolChoice, []adapt.Warning) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch s {
		case "auto":
			return &unified.ToolChoice{Mode: unified.ToolChoiceAuto}, nil
		case "required":
			return &unified.ToolChoice{Mode: unified.ToolChoiceRequired}, nil
		case "none":
			return &unified.ToolChoice{Mode: unified.ToolChoiceNone}, nil
		}
		return nil, []adapt.Warning{decodeWarning(field, fmt.Sprintf("unsupported tool_choice value %q was dropped", s))}
	}
	var obj struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Type == "function" {
		return &unified.ToolChoice{Mode: unified.ToolChoiceTool, Name: obj.Function.Name}, nil
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

func writeStream(ctx context.Context, w http.ResponseWriter, events <-chan unified.Event) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	state := streamState{created: time.Now().Unix()}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-events:
			if !ok {
				_, err := io.WriteString(w, "data: [DONE]\n\n")
				return err
			}
			chunks := state.push(ev)
			if errEvent, ok := ev.(unified.ErrorEvent); ok && errEvent.Err != nil {
				return errEvent.Err
			}
			for _, chunk := range chunks {
				if err := writeSSEData(w, chunk); err != nil {
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
	id      string
	model   string
	created int64
}

func (s *streamState) push(ev unified.Event) []Response {
	switch e := ev.(type) {
	case unified.MessageStartEvent:
		s.id = e.ID
		s.model = e.Model
		return []Response{streamChunk(s.id, s.model, s.created, Choice{Index: 0, Delta: &ResponseDelta{Role: "assistant"}})}
	case unified.TextDeltaEvent:
		return []Response{streamChunk(s.id, s.model, s.created, Choice{Index: 0, Delta: &ResponseDelta{Content: e.Text}})}
	case unified.ReasoningDeltaEvent:
		return []Response{streamChunk(s.id, s.model, s.created, Choice{Index: 0, Delta: &ResponseDelta{ReasoningDetails: []ReasoningDetail{{Type: "text", Text: e.Text}}}})}
	case unified.ToolCallStartEvent:
		return []Response{streamChunk(s.id, s.model, s.created, Choice{Index: 0, Delta: &ResponseDelta{ToolCalls: []ToolCall{{ID: e.ID, Type: "function", Function: ToolCallFunction{Name: e.Name}}}}})}
	case unified.ToolCallArgsDeltaEvent:
		return []Response{streamChunk(s.id, s.model, s.created, Choice{Index: 0, Delta: &ResponseDelta{ToolCalls: []ToolCall{{ID: e.ID, Type: "function", Function: ToolCallFunction{Arguments: e.Delta}}}}})}
	case unified.CompletedEvent:
		return []Response{streamChunk(s.id, s.model, s.created, Choice{Index: 0, Delta: &ResponseDelta{}, FinishReason: finishReason(e.FinishReason)})}
	case unified.ErrorEvent:
		return nil
	default:
		return nil
	}
}

func streamChunk(id, model string, created int64, choice Choice) Response {
	return Response{ID: id, Object: "chat.completion.chunk", Created: created, Model: model, Choices: []Choice{choice}}
}

func writeSSEData(w io.Writer, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", b)
	return err
}

func responseFromUnified(resp unified.Response) Response {
	var content strings.Builder
	var reasoning []ReasoningDetail
	for _, part := range resp.Content {
		switch text := part.(type) {
		case unified.TextPart:
			content.WriteString(text.Text)
		case unified.ReasoningPart:
			reasoning = append(reasoning, ReasoningDetail{Type: "text", Text: text.Text})
		}
	}
	toolCalls := make([]ToolCall, 0, len(resp.ToolCalls))
	for _, call := range resp.ToolCalls {
		toolCalls = append(toolCalls, ToolCall{
			ID:   call.ID,
			Type: "function",
			Function: ToolCallFunction{
				Name:      call.Name,
				Arguments: string(call.Arguments),
			},
		})
	}
	return Response{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   resp.Model,
		Choices: []Choice{{
			Index:        0,
			Message:      &ResponseMessage{Role: "assistant", Content: content.String(), ReasoningDetails: reasoning, ToolCalls: toolCalls},
			FinishReason: finishReason(resp.FinishReason),
		}},
		Usage: UsageWire{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}
}

func finishReason(reason unified.FinishReason) string {
	switch reason {
	case unified.FinishReasonStop:
		return "stop"
	case unified.FinishReasonLength:
		return "length"
	case unified.FinishReasonToolCall:
		return "tool_calls"
	default:
		return string(reason)
	}
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

func statusError(status int, code, message string) error {
	return httpError{status: status, code: code, message: message}
}

func (e httpError) Error() string {
	return e.message
}
