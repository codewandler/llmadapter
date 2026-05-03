package converse

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/codewandler/llmadapter/unified"
)

func TestBuildRequestMapsMessagesToolsAndInferenceProfile(t *testing.T) {
	client, err := NewClient(WithRegion("us-east-1"))
	if err != nil {
		t.Fatal(err)
	}
	maxTokens := 64
	temperature := 0.2
	reasoningTokens := 1024
	req := unified.Request{
		Model: ModelClaudeSonnet46,
		Instructions: []unified.Instruction{{
			Kind:    unified.InstructionSystem,
			Content: []unified.ContentPart{unified.TextPart{Text: "Be terse."}},
		}},
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "What is next?"}},
		}},
		MaxOutputTokens: &maxTokens,
		Temperature:     &temperature,
		Tools: []unified.Tool{{
			Kind:        unified.ToolKindFunction,
			Name:        "lookup",
			Description: "Look something up.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`),
		}},
		ToolChoice: &unified.ToolChoice{Mode: unified.ToolChoiceTool, Name: "lookup"},
		Reasoning:  &unified.ReasoningConfig{MaxTokens: &reasoningTokens},
	}
	built, err := client.buildRequest(req, PrefixUS+"."+ModelClaudeSonnet46)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := aws.ToString(built.input.ModelId), PrefixUS+"."+ModelClaudeSonnet46; got != want {
		t.Fatalf("model id = %q, want %q", got, want)
	}
	if len(built.input.System) != 1 {
		t.Fatalf("system blocks = %d, want 1", len(built.input.System))
	}
	if len(built.input.Messages) != 1 || built.input.Messages[0].Role != types.ConversationRoleUser {
		t.Fatalf("messages = %+v", built.input.Messages)
	}
	if len(built.input.Messages[0].Content) != 1 {
		t.Fatalf("message content blocks = %d, want 1", len(built.input.Messages[0].Content))
	}
	textBlock, ok := built.input.Messages[0].Content[0].(*types.ContentBlockMemberText)
	if !ok || textBlock.Value != "What is next?" {
		t.Fatalf("message content block = %#v", built.input.Messages[0].Content[0])
	}
	if built.input.InferenceConfig == nil || aws.ToInt32(built.input.InferenceConfig.MaxTokens) != int32(maxTokens) {
		t.Fatalf("inference config = %+v", built.input.InferenceConfig)
	}
	if built.input.ToolConfig == nil || len(built.input.ToolConfig.Tools) != 1 {
		t.Fatalf("tool config = %+v", built.input.ToolConfig)
	}
	tool, ok := built.input.ToolConfig.Tools[0].(*types.ToolMemberToolSpec)
	if !ok {
		t.Fatalf("tool = %#v", built.input.ToolConfig.Tools[0])
	}
	if got := aws.ToString(tool.Value.Name); got != "lookup" {
		t.Fatalf("tool name = %q", got)
	}
	if _, ok := built.input.ToolConfig.ToolChoice.(*types.ToolChoiceMemberAuto); !ok {
		t.Fatalf("tool choice = %#v", built.input.ToolConfig.ToolChoice)
	}
	if len(built.warnings) != 1 || built.warnings[0].Code != "unsupported_field_adjusted" {
		t.Fatalf("warnings = %+v", built.warnings)
	}
	if built.input.AdditionalModelRequestFields == nil {
		t.Fatalf("missing additional model request fields")
	}
}

func TestBuildRequestMapsAssistantToolUseAndToolResult(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{
		Model: ModelClaudeSonnet46,
		Messages: []unified.Message{
			{
				Role: unified.RoleAssistant,
				ToolCalls: []unified.ToolCall{{
					ID:        "toolu_1",
					Name:      "lookup",
					Arguments: json.RawMessage(`{"q":"phase"}`),
				}},
			},
			{
				Role: unified.RoleTool,
				ToolResults: []unified.ToolResult{{
					ToolCallID: "toolu_1",
					Content:    []unified.ContentPart{unified.TextPart{Text: "done"}},
				}},
			},
		},
	}
	built, err := client.buildRequest(req, ModelClaudeSonnet46)
	if err != nil {
		t.Fatal(err)
	}
	if len(built.input.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(built.input.Messages))
	}
	if _, ok := built.input.Messages[0].Content[0].(*types.ContentBlockMemberToolUse); !ok {
		t.Fatalf("assistant content = %#v", built.input.Messages[0].Content[0])
	}
	result, ok := built.input.Messages[1].Content[0].(*types.ContentBlockMemberToolResult)
	if !ok {
		t.Fatalf("tool result content = %#v", built.input.Messages[1].Content[0])
	}
	if got := aws.ToString(result.Value.ToolUseId); got != "toolu_1" {
		t.Fatalf("tool result id = %q", got)
	}
}

func TestBuildRequestDropsToolsWhenToolChoiceNone(t *testing.T) {
	client, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	req := unified.Request{
		Model: ModelClaudeSonnet46,
		Tools: []unified.Tool{{
			Name:        "lookup",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
		ToolChoice: &unified.ToolChoice{Mode: unified.ToolChoiceNone},
	}
	built, err := client.buildRequest(req, ModelClaudeSonnet46)
	if err != nil {
		t.Fatal(err)
	}
	if built.input.ToolConfig != nil {
		t.Fatalf("tool config = %+v, want nil", built.input.ToolConfig)
	}
}
