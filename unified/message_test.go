package unified

import (
	"encoding/json"
	"testing"
)

func TestAssistantMessageFromResponsePreservesContentAndToolCalls(t *testing.T) {
	resp := Response{
		ID:      "resp_123",
		Content: []ContentPart{TextPart{Text: "thinking context"}},
		ToolCalls: []ToolCall{{
			ID:        "call_1",
			Name:      "lookup",
			Arguments: json.RawMessage(`{"city":"Berlin"}`),
			Index:     2,
		}},
	}

	msg := AssistantMessageFromResponse(resp)
	if msg.Role != RoleAssistant || msg.ID != "resp_123" {
		t.Fatalf("message identity = %+v", msg)
	}
	if len(msg.Content) != 1 || msg.Content[0].(TextPart).Text != "thinking context" {
		t.Fatalf("content = %+v", msg.Content)
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].ID != "call_1" || string(msg.ToolCalls[0].Arguments) != `{"city":"Berlin"}` {
		t.Fatalf("tool calls = %+v", msg.ToolCalls)
	}

	resp.ToolCalls[0].Arguments[0] = '['
	if string(msg.ToolCalls[0].Arguments) != `{"city":"Berlin"}` {
		t.Fatalf("tool call arguments were not cloned: %s", msg.ToolCalls[0].Arguments)
	}
}
