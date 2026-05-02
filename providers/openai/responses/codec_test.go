package responses

import (
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
