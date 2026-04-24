package messages

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/codewandler/llmadapter/transport"
)

type SSEFrameDecoder struct{}

func (d *SSEFrameDecoder) PushFrame(ctx context.Context, raw []byte) ([]Event, error) {
	frame, err := transport.ParseSSEFrame(raw)
	if err != nil {
		return nil, err
	}
	if len(frame.Data) == 0 || string(frame.Data) == "[DONE]" {
		return nil, nil
	}
	eventType := frame.Event
	if eventType == "" || eventType == "data" || eventType == "message" {
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(frame.Data, &envelope); err == nil && envelope.Type != "" {
			eventType = envelope.Type
		}
	}

	switch eventType {
	case "message_start":
		return decodeOne[MessageStartEvent](frame.Data)
	case "content_block_start":
		return decodeOne[ContentBlockStartEvent](frame.Data)
	case "content_block_delta":
		return decodeOne[ContentBlockDeltaEvent](frame.Data)
	case "content_block_stop":
		return decodeOne[ContentBlockStopEvent](frame.Data)
	case "message_delta":
		return decodeOne[MessageDeltaEvent](frame.Data)
	case "message_stop":
		if len(frame.Data) == 0 {
			return []Event{MessageStopEvent{Type: "message_stop"}}, nil
		}
		return decodeOne[MessageStopEvent](frame.Data)
	case "ping":
		if len(frame.Data) == 0 {
			return []Event{PingEvent{Type: "ping"}}, nil
		}
		return decodeOne[PingEvent](frame.Data)
	case "error":
		return decodeOne[ErrorEventWire](frame.Data)
	default:
		return nil, fmt.Errorf("unknown anthropic SSE event type %q", eventType)
	}
}

func (d *SSEFrameDecoder) Close(ctx context.Context) ([]Event, error) {
	return nil, nil
}

func decodeOne[T Event](data []byte) ([]Event, error) {
	var ev T
	if err := json.Unmarshal(data, &ev); err != nil {
		return nil, err
	}
	return []Event{ev}, nil
}
