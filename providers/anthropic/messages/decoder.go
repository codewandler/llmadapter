package messages

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/codewandler/llmadapter/internal/citations"
	"github.com/codewandler/llmadapter/unified"
)

type EventDecoder struct {
	blockTypes map[int]string
	toolIDs    map[int]string
	toolNames  map[int]string
	toolArgs   map[int]*bytes.Buffer
	messageID  string
}

func NewEventDecoder() *EventDecoder {
	return &EventDecoder{
		blockTypes: make(map[int]string),
		toolIDs:    make(map[int]string),
		toolNames:  make(map[int]string),
		toolArgs:   make(map[int]*bytes.Buffer),
	}
}

func (d *EventDecoder) Push(ctx context.Context, ev Event) ([]unified.Event, error) {
	switch e := ev.(type) {
	case MessageStartEvent:
		d.messageID = e.Message.ID
		return []unified.Event{unified.MessageStartEvent{ID: e.Message.ID, Model: e.Message.Model, Role: unified.RoleAssistant}}, nil
	case ContentBlockStartEvent:
		return d.contentBlockStart(e), nil
	case ContentBlockDeltaEvent:
		return d.contentBlockDelta(e)
	case ContentBlockStopEvent:
		return d.contentBlockStop(e), nil
	case MessageDeltaEvent:
		return d.messageDelta(e), nil
	case MessageStopEvent:
		return []unified.Event{unified.MessageDoneEvent{ID: d.messageID}}, nil
	case PingEvent:
		return nil, nil
	case ErrorEventWire:
		return []unified.Event{unified.ErrorEvent{Err: &unified.APIError{Type: e.Error.Type, Message: e.Error.Message, ProviderRaw: providerRawJSON(e)}}}, nil
	default:
		return nil, fmt.Errorf("unsupported anthropic event %T", ev)
	}
}

func (d *EventDecoder) Close(ctx context.Context) ([]unified.Event, error) {
	if len(d.toolArgs) > 0 {
		return nil, fmt.Errorf("incomplete Anthropic tool call stream")
	}
	return nil, nil
}

func (d *EventDecoder) contentBlockStart(e ContentBlockStartEvent) []unified.Event {
	block := e.ContentBlock
	d.blockTypes[e.Index] = block.Type
	switch block.Type {
	case "text":
		out := []unified.Event{unified.ContentBlockStartEvent{Index: e.Index, Kind: unified.ContentKindText}}
		out = append(out, citationEventsFromAnthropic(e.Index, block.Citations)...)
		return out
	case "thinking":
		out := []unified.Event{unified.ContentBlockStartEvent{Index: e.Index, Kind: unified.ContentKindReasoning}}
		if block.Thinking != "" || block.Signature != "" {
			out = append(out, unified.ReasoningDeltaEvent{Index: e.Index, Text: block.Thinking, Signature: block.Signature})
		}
		return out
	case "tool_use":
		d.toolIDs[e.Index] = block.ID
		d.toolNames[e.Index] = block.Name
		d.toolArgs[e.Index] = &bytes.Buffer{}
		var out []unified.Event
		out = append(out, unified.ContentBlockStartEvent{Index: e.Index, Kind: unified.ContentKindToolCall, ID: block.ID, Name: block.Name})
		out = append(out, unified.ToolCallStartEvent{Index: e.Index, ID: block.ID, Name: block.Name})
		if len(block.Input) > 0 && string(block.Input) != "null" && string(block.Input) != "{}" {
			d.toolArgs[e.Index].Write(block.Input)
			out = append(out, unified.ToolCallArgsDeltaEvent{Index: e.Index, ID: block.ID, Delta: string(block.Input)})
		}
		return out
	default:
		return []unified.Event{unified.RawEvent{APIKind: "anthropic.messages", Type: block.Type, JSON: providerRawJSON(block), Value: block}}
	}
}

func (d *EventDecoder) contentBlockDelta(e ContentBlockDeltaEvent) ([]unified.Event, error) {
	switch e.Delta.Type {
	case "text_delta":
		return []unified.Event{unified.TextDeltaEvent{Index: e.Index, Text: e.Delta.Text}}, nil
	case "input_json_delta":
		if d.toolArgs[e.Index] == nil {
			d.toolArgs[e.Index] = &bytes.Buffer{}
		}
		d.toolArgs[e.Index].WriteString(e.Delta.PartialJSON)
		return []unified.Event{unified.ToolCallArgsDeltaEvent{Index: e.Index, ID: d.toolIDs[e.Index], Delta: e.Delta.PartialJSON}}, nil
	case "thinking_delta":
		return []unified.Event{unified.ReasoningDeltaEvent{Index: e.Index, Text: e.Delta.Thinking}}, nil
	case "signature_delta":
		return []unified.Event{unified.ReasoningDeltaEvent{Index: e.Index, Signature: e.Delta.Signature}}, nil
	default:
		return nil, fmt.Errorf("unsupported Anthropic content block delta type %q", e.Delta.Type)
	}
}

func (d *EventDecoder) contentBlockStop(e ContentBlockStopEvent) []unified.Event {
	blockType := d.blockTypes[e.Index]
	delete(d.blockTypes, e.Index)
	switch blockType {
	case "tool_use":
		args := json.RawMessage(nil)
		if buf := d.toolArgs[e.Index]; buf != nil && buf.Len() > 0 {
			args = json.RawMessage(buf.String())
		}
		out := []unified.Event{
			unified.ToolCallDoneEvent{Index: e.Index, ID: d.toolIDs[e.Index], Name: d.toolNames[e.Index], Args: args},
			unified.ContentBlockDoneEvent{Index: e.Index, Kind: unified.ContentKindToolCall},
		}
		delete(d.toolIDs, e.Index)
		delete(d.toolNames, e.Index)
		delete(d.toolArgs, e.Index)
		return out
	case "thinking":
		return []unified.Event{unified.ContentBlockDoneEvent{Index: e.Index, Kind: unified.ContentKindReasoning}}
	default:
		return []unified.Event{unified.ContentBlockDoneEvent{Index: e.Index, Kind: unified.ContentKindText}}
	}
}

func (d *EventDecoder) messageDelta(e MessageDeltaEvent) []unified.Event {
	var out []unified.Event
	if e.Delta.StopReason != "" {
		out = append(out, unified.CompletedEvent{FinishReason: mapStopReason(e.Delta.StopReason), MessageID: d.messageID})
	}
	if e.Usage != nil {
		out = append(out, unified.UsageEvent{
			Tokens: unified.TokenItems{
				{Kind: unified.TokenKindInputNew, Count: e.Usage.InputTokens},
				{Kind: unified.TokenKindInputCacheRead, Count: e.Usage.CacheReadInputTokens},
				{Kind: unified.TokenKindInputCacheWrite, Count: e.Usage.CacheCreationInputTokens},
				{Kind: unified.TokenKindOutput, Count: e.Usage.OutputTokens},
			}.NonZero(),
			ProviderRaw: providerRawJSON(e.Usage),
		})
	}
	return out
}

func providerRawJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil || len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return raw
}

func citationEventsFromAnthropic(index int, raw []any) []unified.Event {
	out := make([]unified.Event, 0, len(raw))
	for _, value := range raw {
		values, ok := value.(map[string]any)
		if !ok {
			out = append(out, unified.RawEvent{APIKind: "anthropic.messages", Type: "citation", Value: value})
			continue
		}
		citation := citationFromAnthropicMap(values)
		if citation.Type == "" {
			out = append(out, unified.RawEvent{APIKind: "anthropic.messages", Type: "citation", JSON: providerRawJSON(values), Value: value})
			continue
		}
		out = append(out, unified.CitationEvent{Index: index, Citation: citation})
	}
	return out
}

func citationFromAnthropicMap(values map[string]any) unified.Citation {
	return citations.FromMap(values, citations.Spec{
		TextKeys:       []string{"cited_text", "text", "quote"},
		TitleKeys:      []string{"title", "document_title"},
		DocumentIDKeys: []string{"document_id", "file_id"},
		StartKeys:      []string{"start_char_index", "start_offset", "start_index"},
		EndKeys:        []string{"end_char_index", "end_offset", "end_index"},
	})
}

func mapStopReason(reason string) unified.FinishReason {
	switch reason {
	case "end_turn", "stop_sequence":
		return unified.FinishReasonStop
	case "max_tokens":
		return unified.FinishReasonLength
	case "tool_use":
		return unified.FinishReasonToolCall
	default:
		return unified.FinishReasonUnknown
	}
}
