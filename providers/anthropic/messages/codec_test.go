package messages

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/codewandler/llmadapter/adapt"
	"github.com/codewandler/llmadapter/unified"
)

func TestCodecEncodeRequest(t *testing.T) {
	maxTokens := 128
	req := adapt.Request{Unified: unified.Request{
		Model:           "claude-test",
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
		Instructions: []unified.Instruction{{
			Kind:    unified.InstructionSystem,
			Content: []unified.ContentPart{unified.TextPart{Text: "be brief"}},
		}},
		Tools: []unified.Tool{{
			Kind:        unified.ToolKindFunction,
			Name:        "lookup",
			Description: "look up a value",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
		ToolChoice: &unified.ToolChoice{Mode: unified.ToolChoiceTool, Name: "lookup"},
		Stream:     true,
	}}

	wire, err := (Codec{}).EncodeRequest(context.Background(), &req)
	if err != nil {
		t.Fatal(err)
	}
	if wire.Model != "claude-test" || wire.MaxTokens != 128 || wire.System != "be brief" || !wire.Stream {
		t.Fatalf("unexpected wire request: %+v", wire)
	}
	if len(wire.Messages) != 1 || wire.Messages[0].Role != "user" || wire.Messages[0].Content[0].Text != "hello" {
		t.Fatalf("unexpected wire messages: %+v", wire.Messages)
	}
	if len(wire.Tools) != 1 || wire.Tools[0].Name != "lookup" {
		t.Fatalf("unexpected tools: %+v", wire.Tools)
	}
	if wire.ToolChoice == nil || wire.ToolChoice.Type != "tool" || wire.ToolChoice.Name != "lookup" {
		t.Fatalf("unexpected tool choice: %+v", wire.ToolChoice)
	}
}

func TestCodecStrictUnsupported(t *testing.T) {
	maxTokens := 128
	seed := int64(1)
	req := adapt.Request{
		MappingMode: adapt.MappingModeStrict,
		Unified: unified.Request{
			Model:           "claude-test",
			MaxOutputTokens: &maxTokens,
			Seed:            &seed,
		},
	}
	_, err := (Codec{}).EncodeRequest(context.Background(), &req)
	var unsupported *adapt.UnsupportedFieldError
	if !errors.As(err, &unsupported) {
		t.Fatalf("err = %v, want UnsupportedFieldError", err)
	}
}

func TestCodecMissingMaxTokens(t *testing.T) {
	req := adapt.Request{Unified: unified.Request{Model: "claude-test"}}
	_, err := (Codec{}).EncodeRequest(context.Background(), &req)
	var unsupported *adapt.UnsupportedFieldError
	if !errors.As(err, &unsupported) {
		t.Fatalf("err = %v, want UnsupportedFieldError", err)
	}
}

func TestCodecBestEffortWarnings(t *testing.T) {
	maxTokens := 128
	seed := int64(1)
	req := adapt.Request{
		MappingMode: adapt.MappingModeBestEffort,
		Unified: unified.Request{
			Model:           "claude-test",
			MaxOutputTokens: &maxTokens,
			Seed:            &seed,
			ResponseFormat:  &unified.ResponseFormat{Kind: unified.ResponseFormatJSON},
		},
	}
	_, err := (Codec{}).EncodeRequest(context.Background(), &req)
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Warnings) != 2 {
		t.Fatalf("warnings = %+v", req.Warnings)
	}
	if req.Warnings[0].Code != "unsupported_field_dropped" || req.Warnings[0].Field != "seed" {
		t.Fatalf("unexpected warning: %+v", req.Warnings[0])
	}
}
