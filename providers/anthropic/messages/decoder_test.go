package messages

import (
	"context"
	"errors"
	"testing"

	"github.com/codewandler/llmadapter/unified"
)

func TestEventDecoderTextAndTool(t *testing.T) {
	dec := NewEventDecoder()
	input := []Event{
		MessageStartEvent{Message: MessageResponse{ID: "msg", Model: "claude"}},
		ContentBlockStartEvent{Index: 0, ContentBlock: ContentBlock{Type: "text"}},
		ContentBlockDeltaEvent{Index: 0, Delta: Delta{Type: "text_delta", Text: "hi"}},
		ContentBlockStopEvent{Index: 0},
		ContentBlockStartEvent{Index: 1, ContentBlock: ContentBlock{Type: "tool_use", ID: "toolu", Name: "lookup"}},
		ContentBlockDeltaEvent{Index: 1, Delta: Delta{Type: "input_json_delta", PartialJSON: `{"q":"x"}`}},
		ContentBlockStopEvent{Index: 1},
		MessageDeltaEvent{Delta: MessageDeltaBody{StopReason: "tool_use"}, Usage: &UsageWire{InputTokens: 3, OutputTokens: 4}},
		MessageStopEvent{},
	}
	var out []unified.Event
	for _, ev := range input {
		got, err := dec.Push(context.Background(), ev)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, got...)
	}
	if _, err := dec.Close(context.Background()); err != nil {
		t.Fatal(err)
	}

	resp, err := unified.Collect(context.Background(), sliceEvents(out))
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != "msg" || resp.Model != "claude" || resp.FinishReason != unified.FinishReasonToolCall {
		t.Fatalf("unexpected response metadata: %+v", resp)
	}
	if len(resp.Content) != 1 || resp.Content[0].(unified.TextPart).Text != "hi" {
		t.Fatalf("unexpected content: %+v", resp.Content)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].ID != "toolu" || string(resp.ToolCalls[0].Arguments) != `{"q":"x"}` {
		t.Fatalf("unexpected tools: %+v", resp.ToolCalls)
	}
}

func TestEventDecoderErrorEvent(t *testing.T) {
	dec := NewEventDecoder()
	out, err := dec.Push(context.Background(), ErrorEventWire{
		Type:  "error",
		Error: APIErrorBody{Type: "overloaded_error", Message: "try again"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = unified.Collect(context.Background(), sliceEvents(out))
	var apiErr *unified.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T, want APIError", err)
	}
	if apiErr.Type != "overloaded_error" || apiErr.Message != "try again" {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
}

func sliceEvents(events []unified.Event) <-chan unified.Event {
	ch := make(chan unified.Event, len(events))
	for _, ev := range events {
		ch <- ev
	}
	close(ch)
	return ch
}
