package anthropicmessages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/codewandler/llmadapter/adapt"
	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
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
	var wire anthropic.MessageRequest
	if err := json.Unmarshal(body, &wire); err != nil {
		return adapt.Request{}, statusError(http.StatusBadRequest, "invalid_json", err.Error())
	}
	ureq, err := decodeRequest(wire)
	if err != nil {
		return adapt.Request{}, err
	}
	return adapt.Request{
		SourceAPI: adapt.ApiAnthropicMessages,
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
	errorType := "api_error"
	message := err.Error()
	var apiErr *unified.APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode != 0 {
			status = apiErr.StatusCode
		}
		if apiErr.Type != "" {
			errorType = apiErr.Type
		} else if apiErr.Code != "" {
			errorType = apiErr.Code
		}
		if apiErr.Message != "" {
			message = apiErr.Message
		}
	}
	var httpErr httpError
	if errors.As(err, &httpErr) {
		status = httpErr.status
		errorType = httpErr.code
		message = httpErr.message
	}
	return writeJSON(w, status, map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    errorType,
			"message": message,
		},
	})
}

func decodeRequest(wire anthropic.MessageRequest) (unified.Request, error) {
	if wire.Model == "" {
		return unified.Request{}, statusError(http.StatusBadRequest, "missing_model", "model is required")
	}
	if wire.MaxTokens == 0 {
		return unified.Request{}, statusError(http.StatusBadRequest, "missing_max_tokens", "max_tokens is required")
	}
	maxTokens := wire.MaxTokens
	out := unified.Request{
		Model:           wire.Model,
		MaxOutputTokens: &maxTokens,
		Temperature:     wire.Temperature,
		TopP:            wire.TopP,
		TopK:            wire.TopK,
		Stop:            append([]string(nil), wire.StopSequences...),
		Stream:          wire.Stream,
	}
	if wire.System != "" {
		out.Instructions = append(out.Instructions, unified.Instruction{
			Kind:    unified.InstructionSystem,
			Content: []unified.ContentPart{unified.TextPart{Text: wire.System}},
		})
	}
	for _, msg := range wire.Messages {
		out.Messages = append(out.Messages, decodeMessage(msg)...)
	}
	for _, tool := range wire.Tools {
		out.Tools = append(out.Tools, unified.Tool{
			Kind:        unified.ToolKindFunction,
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	if wire.ToolChoice != nil {
		out.ToolChoice = decodeToolChoice(*wire.ToolChoice)
	}
	return out, nil
}

func decodeMessage(msg anthropic.InputMessage) []unified.Message {
	var content []unified.ContentPart
	var toolCalls []unified.ToolCall
	var toolResults []unified.ToolResult
	for i, block := range msg.Content {
		switch block.Type {
		case "text":
			content = append(content, unified.TextPart{Text: block.Text})
		case "thinking":
			content = append(content, unified.ReasoningPart{Text: block.Thinking})
		case "tool_use":
			input := block.Input
			if len(input) == 0 {
				input = json.RawMessage(`{}`)
			}
			toolCalls = append(toolCalls, unified.ToolCall{ID: block.ID, Name: block.Name, Arguments: input, Index: i})
		case "tool_result":
			toolResults = append(toolResults, unified.ToolResult{
				ToolCallID: block.ToolUseID,
				Content:    []unified.ContentPart{unified.TextPart{Text: contentBlockText(block.Content)}},
				IsError:    block.IsError,
			})
		}
	}
	if len(toolResults) > 0 && msg.Role == "user" {
		return []unified.Message{{Role: unified.RoleTool, ToolResults: toolResults}}
	}
	return []unified.Message{{
		Role:      unified.Role(msg.Role),
		Content:   content,
		ToolCalls: toolCalls,
	}}
}

func contentBlockText(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			if m, ok := item.(map[string]any); ok && m["type"] == "text" {
				if text, ok := m["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return ""
	}
}

func decodeToolChoice(choice anthropic.ToolChoiceWire) *unified.ToolChoice {
	switch choice.Type {
	case "auto", "":
		return &unified.ToolChoice{Mode: unified.ToolChoiceAuto}
	case "any":
		return &unified.ToolChoice{Mode: unified.ToolChoiceAny}
	case "tool":
		return &unified.ToolChoice{Mode: unified.ToolChoiceTool, Name: choice.Name}
	default:
		return nil
	}
}

func responseFromUnified(resp unified.Response) anthropic.MessageResponse {
	out := anthropic.MessageResponse{
		ID:         resp.ID,
		Type:       "message",
		Role:       "assistant",
		Model:      resp.Model,
		Usage:      usageFromUnified(resp.Usage),
		StopReason: stopReason(resp.FinishReason),
	}
	out.Content = append(out.Content, contentBlocksFromResponse(resp)...)
	return out
}

func contentBlocksFromResponse(resp unified.Response) []anthropic.ContentBlock {
	var blocks []anthropic.ContentBlock
	for _, part := range resp.Content {
		switch p := part.(type) {
		case unified.TextPart:
			blocks = append(blocks, anthropic.ContentBlock{Type: "text", Text: p.Text})
		case unified.ReasoningPart:
			blocks = append(blocks, anthropic.ContentBlock{Type: "thinking", Thinking: p.Text})
		}
	}
	for _, call := range resp.ToolCalls {
		input := call.Arguments
		if len(input) == 0 {
			input = json.RawMessage(`{}`)
		}
		blocks = append(blocks, anthropic.ContentBlock{
			Type:  "tool_use",
			ID:    call.ID,
			Name:  call.Name,
			Input: input,
		})
	}
	return blocks
}

func usageFromUnified(usage unified.Usage) *anthropic.UsageWire {
	if usage.InputTokens == 0 && usage.OutputTokens == 0 && usage.CacheReadTokens == 0 && usage.CacheWriteTokens == 0 {
		return nil
	}
	return &anthropic.UsageWire{
		InputTokens:              usage.InputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheReadInputTokens:     usage.CacheReadTokens,
		CacheCreationInputTokens: usage.CacheWriteTokens,
	}
}

func stopReason(reason unified.FinishReason) string {
	switch reason {
	case unified.FinishReasonStop:
		return "end_turn"
	case unified.FinishReasonLength:
		return "max_tokens"
	case unified.FinishReasonToolCall:
		return "tool_use"
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

func (e httpError) Error() string { return e.message }

func statusError(status int, code, message string) error {
	return httpError{status: status, code: code, message: message}
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
				if err := writeSSEEvent(w, frame.event, frame.data); err != nil {
					return err
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
		}
	}
}

type streamFrame struct {
	event string
	data  any
}

type streamState struct {
	id         string
	model      string
	usage      unified.Usage
	started    map[int]unified.ContentKind
	toolIDs    map[int]string
	toolNames  map[int]string
	toolArgs   map[int]string
	messageEnd bool
}

func (s *streamState) push(ev unified.Event) []streamFrame {
	switch e := ev.(type) {
	case unified.MessageStartEvent:
		s.id = e.ID
		s.model = e.Model
		return []streamFrame{{
			event: "message_start",
			data: anthropic.MessageStartEvent{Type: "message_start", Message: anthropic.MessageResponse{
				ID:      e.ID,
				Type:    "message",
				Role:    "assistant",
				Model:   e.Model,
				Content: []anthropic.ContentBlock{},
			}},
		}}
	case unified.ContentBlockStartEvent:
		return s.startBlock(e.Index, e.Kind, e.ID, e.Name)
	case unified.TextDeltaEvent:
		frames := s.startBlock(e.Index, unified.ContentKindText, "", "")
		frames = append(frames, streamFrame{event: "content_block_delta", data: anthropic.ContentBlockDeltaEvent{
			Type:  "content_block_delta",
			Index: e.Index,
			Delta: anthropic.Delta{Type: "text_delta", Text: e.Text},
		}})
		return frames
	case unified.ReasoningDeltaEvent:
		frames := s.startBlock(e.Index, unified.ContentKindReasoning, "", "")
		frames = append(frames, streamFrame{event: "content_block_delta", data: anthropic.ContentBlockDeltaEvent{
			Type:  "content_block_delta",
			Index: e.Index,
			Delta: anthropic.Delta{Type: "thinking_delta", Thinking: e.Text},
		}})
		return frames
	case unified.ToolCallStartEvent:
		return s.startBlock(e.Index, unified.ContentKindToolCall, e.ID, e.Name)
	case unified.ToolCallArgsDeltaEvent:
		s.ensure()
		s.toolArgs[e.Index] += e.Delta
		return []streamFrame{{event: "content_block_delta", data: anthropic.ContentBlockDeltaEvent{
			Type:  "content_block_delta",
			Index: e.Index,
			Delta: anthropic.Delta{Type: "input_json_delta", PartialJSON: e.Delta},
		}}}
	case unified.ToolCallDoneEvent:
		s.ensure()
		var frames []streamFrame
		if s.toolArgs[e.Index] == "" && len(e.Args) > 0 {
			frames = append(frames, streamFrame{event: "content_block_delta", data: anthropic.ContentBlockDeltaEvent{
				Type:  "content_block_delta",
				Index: e.Index,
				Delta: anthropic.Delta{Type: "input_json_delta", PartialJSON: string(e.Args)},
			}})
		}
		frames = append(frames, s.stopBlock(e.Index)...)
		return frames
	case unified.ContentBlockDoneEvent:
		return s.stopBlock(e.Index)
	case unified.UsageEvent:
		s.usage = unified.Usage{
			InputTokens:      e.InputTokens,
			OutputTokens:     e.OutputTokens,
			ReasoningTokens:  e.ReasoningTokens,
			CacheReadTokens:  e.CacheReadTokens,
			CacheWriteTokens: e.CacheWriteTokens,
			TotalTokens:      e.TotalTokens,
		}
		return nil
	case unified.CompletedEvent:
		if s.messageEnd {
			return nil
		}
		s.messageEnd = true
		return []streamFrame{{
			event: "message_delta",
			data: anthropic.MessageDeltaEvent{
				Type:  "message_delta",
				Delta: anthropic.MessageDeltaBody{StopReason: stopReason(e.FinishReason)},
				Usage: usageFromUnified(s.usage),
			},
		}, {
			event: "message_stop",
			data:  anthropic.MessageStopEvent{Type: "message_stop"},
		}}
	default:
		return nil
	}
}

func (s *streamState) startBlock(index int, kind unified.ContentKind, id, name string) []streamFrame {
	s.ensure()
	if _, ok := s.started[index]; ok {
		return nil
	}
	s.started[index] = kind
	block := anthropic.ContentBlock{Type: "text", Text: ""}
	if kind == unified.ContentKindReasoning {
		block = anthropic.ContentBlock{Type: "thinking", Thinking: ""}
	}
	if kind == unified.ContentKindToolCall {
		s.toolIDs[index] = firstNonEmpty(id, s.toolIDs[index])
		s.toolNames[index] = firstNonEmpty(name, s.toolNames[index])
		block = anthropic.ContentBlock{
			Type:  "tool_use",
			ID:    s.toolIDs[index],
			Name:  s.toolNames[index],
			Input: json.RawMessage(`{}`),
		}
	}
	return []streamFrame{{event: "content_block_start", data: anthropic.ContentBlockStartEvent{
		Type:         "content_block_start",
		Index:        index,
		ContentBlock: block,
	}}}
}

func (s *streamState) stopBlock(index int) []streamFrame {
	s.ensure()
	if _, ok := s.started[index]; !ok {
		return nil
	}
	delete(s.started, index)
	return []streamFrame{{event: "content_block_stop", data: anthropic.ContentBlockStopEvent{Type: "content_block_stop", Index: index}}}
}

func (s *streamState) ensure() {
	if s.started == nil {
		s.started = make(map[int]unified.ContentKind)
	}
	if s.toolIDs == nil {
		s.toolIDs = make(map[int]string)
	}
	if s.toolNames == nil {
		s.toolNames = make(map[int]string)
	}
	if s.toolArgs == nil {
		s.toolArgs = make(map[int]string)
	}
}

func writeSSEEvent(w io.Writer, event string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
	return err
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
