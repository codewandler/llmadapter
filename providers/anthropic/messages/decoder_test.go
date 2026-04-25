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
	if got, want := resp.Usage.Tokens.Count(unified.TokenKindInputNew), 3; got != want {
		t.Fatalf("input.new = %d, want %d", got, want)
	}
	if got, want := resp.Usage.Tokens.Count(unified.TokenKindOutput), 4; got != want {
		t.Fatalf("output = %d, want %d", got, want)
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

func TestEventDecoderReasoningSignature(t *testing.T) {
	dec := NewEventDecoder()
	input := []Event{
		ContentBlockStartEvent{Index: 0, ContentBlock: ContentBlock{Type: "thinking"}},
		ContentBlockDeltaEvent{Index: 0, Delta: Delta{Type: "thinking_delta", Thinking: "think"}},
		ContentBlockDeltaEvent{Index: 0, Delta: Delta{Type: "signature_delta", Signature: "sig"}},
		ContentBlockStopEvent{Index: 0},
	}
	var out []unified.Event
	for _, ev := range input {
		got, err := dec.Push(context.Background(), ev)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, got...)
	}
	resp, err := unified.Collect(context.Background(), sliceEvents(out))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Content) != 1 {
		t.Fatalf("content = %+v", resp.Content)
	}
	part, ok := resp.Content[0].(unified.ReasoningPart)
	if !ok || part.Text != "think" || part.Signature != "sig" {
		t.Fatalf("reasoning = %+v", resp.Content[0])
	}
}

func TestEventDecoderPreservesUnknownContentBlockAsRaw(t *testing.T) {
	dec := NewEventDecoder()
	out, err := dec.Push(context.Background(), ContentBlockStartEvent{
		Index:        0,
		ContentBlock: ContentBlock{Type: "server_tool_use", ID: "srv_1", Name: "web_search"},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), sliceEvents(out))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Raw) != 1 {
		t.Fatalf("raw = %+v", resp.Raw)
	}
	raw := resp.Raw[0]
	if raw.APIKind != "anthropic.messages" || raw.Type != "server_tool_use" {
		t.Fatalf("unexpected raw event: %+v", raw)
	}
	block, ok := raw.Value.(ContentBlock)
	if !ok || block.ID != "srv_1" || block.Name != "web_search" {
		t.Fatalf("unexpected raw value: %#v", raw.Value)
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
