package responses

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/codewandler/llmadapter/unified"
)

func TestEncodeToolChoiceNoneWithoutTools(t *testing.T) {
	wire, _ := encodeRequest(unified.Request{
		Model: "gpt-test",
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
		ToolChoice: &unified.ToolChoice{Mode: unified.ToolChoiceNone},
	})
	if wire.ToolChoice != "none" {
		t.Fatalf("tool_choice = %#v, want none", wire.ToolChoice)
	}
}

func TestEncodeAssistantMessagePhase(t *testing.T) {
	wire, warnings := encodeRequest(unified.Request{
		Model: "gpt-test",
		Messages: []unified.Message{
			{
				Role:    unified.RoleUser,
				Phase:   unified.MessagePhaseCommentary,
				Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
			},
			{
				Role:    unified.RoleAssistant,
				Phase:   unified.MessagePhaseFinalAnswer,
				Content: []unified.ContentPart{unified.TextPart{Text: "answer"}},
			},
		},
	})
	if len(wire.Input) != 2 {
		t.Fatalf("input items = %d, want 2", len(wire.Input))
	}
	if wire.Input[0].Phase != "" {
		t.Fatalf("user phase = %q, want empty", wire.Input[0].Phase)
	}
	if wire.Input[1].Phase != "final_answer" {
		t.Fatalf("assistant phase = %q, want final_answer", wire.Input[1].Phase)
	}
	if len(warnings) != 1 || warnings[0].field != "messages.0.phase" {
		t.Fatalf("warnings = %+v, want user phase warning", warnings)
	}
}

func TestDecodeAssistantMessagePhase(t *testing.T) {
	frames := [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-test","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.output_text.delta","response_id":"resp_1","output_index":0,"content_index":0,"delta":"working"}`),
		[]byte(`data: {"type":"response.output_item.done","item":{"id":"msg_1","type":"message","role":"assistant","status":"completed","phase":"commentary","content":[{"type":"output_text","text":"working"}]}}`),
		[]byte(`data: {"type":"response.completed","response":{"id":"resp_1","model":"gpt-test","status":"completed"}}`),
	}
	var decoder streamDecoder
	var got []unified.Event
	for _, frame := range frames {
		events, err := decoder.push(frame)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, events...)
	}
	ch := make(chan unified.Event, len(got))
	for _, event := range got {
		ch <- event
	}
	close(ch)
	resp, err := unified.Collect(context.Background(), ch)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Phase != unified.MessagePhaseCommentary {
		t.Fatalf("phase = %q, want commentary", resp.Phase)
	}
}

func TestDecodeAssistantPhaseFromOutputItemAddedBeforeStart(t *testing.T) {
	var decoder streamDecoder
	events, err := decoder.push([]byte(`data: {"type":"response.output_item.added","response_id":"resp_1","output_index":0,"item":{"id":"msg_1","type":"message","role":"assistant","status":"in_progress","phase":"final_answer"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("events after output_item.added = %+v, want none", events)
	}
	events, err = decoder.push([]byte(`data: {"type":"response.output_text.delta","response_id":"resp_1","output_index":0,"content_index":0,"delta":"answer"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("expected events")
	}
	start, ok := events[0].(unified.MessageStartEvent)
	if !ok {
		b, _ := json.Marshal(events)
		t.Fatalf("first event = %T %s, want MessageStartEvent", events[0], b)
	}
	if start.Phase != unified.MessagePhaseFinalAnswer {
		t.Fatalf("start phase = %q, want final_answer", start.Phase)
	}
}
