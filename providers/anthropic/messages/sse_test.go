package messages

import (
	"context"
	"testing"
)

func TestSSEFrameDecoder(t *testing.T) {
	raw := []byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`)
	events, err := (&SSEFrameDecoder{}).PushFrame(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	ev, ok := events[0].(ContentBlockDeltaEvent)
	if !ok || ev.Index != 0 || ev.Delta.Text != "hi" {
		t.Fatalf("unexpected event: %#v", events[0])
	}
}

func TestSSEFrameDecoderFallsBackToDataType(t *testing.T) {
	raw := []byte(`event: data
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`)
	events, err := (&SSEFrameDecoder{}).PushFrame(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	ev, ok := events[0].(ContentBlockDeltaEvent)
	if !ok || ev.Index != 0 || ev.Delta.Text != "hi" {
		t.Fatalf("unexpected event: %#v", events[0])
	}
}

func TestSSEFrameDecoderSkipsDone(t *testing.T) {
	events, err := (&SSEFrameDecoder{}).PushFrame(context.Background(), []byte(`event: data
data: [DONE]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestSSEFrameDecoderUnknown(t *testing.T) {
	_, err := (&SSEFrameDecoder{}).PushFrame(context.Background(), []byte(`event: nope
data: {}`))
	if err == nil {
		t.Fatalf("expected error")
	}
}
