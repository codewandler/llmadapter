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
	if wire.Model != "claude-test" || wire.MaxTokens != 128 || wire.System.Text() != "be brief" || !wire.Stream {
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

func TestCodecEncodeSystemCacheControl(t *testing.T) {
	maxTokens := 128
	req := adapt.Request{Unified: unified.Request{
		Model:           "claude-test",
		MaxOutputTokens: &maxTokens,
		Instructions: []unified.Instruction{{
			Kind: unified.InstructionSystem,
			Content: []unified.ContentPart{unified.TextPart{
				Text:         "cache this stable prefix",
				CacheControl: unified.EphemeralCache("5m"),
			}},
		}},
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
		Stream: true,
	}}

	wire, err := (Codec{}).EncodeRequest(context.Background(), &req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		System []ContentBlock `json:"system"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.System) != 1 || raw.System[0].Cache == nil || raw.System[0].Cache.Type != "ephemeral" || raw.System[0].Cache.TTL != "5m" {
		t.Fatalf("unexpected system cache control: %s", body)
	}
}

func TestCodecEncodeCachePolicy(t *testing.T) {
	maxTokens := 128
	req := adapt.Request{Unified: unified.Request{
		Model:           "claude-test",
		MaxOutputTokens: &maxTokens,
		CachePolicy:     unified.CachePolicyOn,
		CacheTTL:        "1h",
		Instructions: []unified.Instruction{{
			Kind: unified.InstructionSystem,
			Content: []unified.ContentPart{
				unified.TextPart{Text: "first"},
				unified.TextPart{Text: "last"},
			},
		}},
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
		}},
		Tools: []unified.Tool{{
			Kind:        unified.ToolKindFunction,
			Name:        "lookup",
			Description: "look up a value",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
		Stream: true,
	}}

	wire, err := (Codec{}).EncodeRequest(context.Background(), &req)
	if err != nil {
		t.Fatal(err)
	}
	if wire.System.Blocks[0].Cache != nil {
		t.Fatalf("expected first system block to remain uncached: %+v", wire.System.Blocks)
	}
	if got := wire.System.Blocks[1].Cache; got == nil || got.Type != "ephemeral" || got.TTL != "1h" {
		t.Fatalf("unexpected system cache control: %+v", wire.System.Blocks)
	}
	if len(wire.Tools) != 1 || wire.Tools[0].Cache == nil || wire.Tools[0].Cache.TTL != "1h" {
		t.Fatalf("unexpected tool cache control: %+v", wire.Tools)
	}
}

func TestCodecEncodeReasoningThinking(t *testing.T) {
	maxTokens := 4096
	temperature := 0.2
	topK := 5
	budget := 2048
	req := adapt.Request{Unified: unified.Request{
		Model:           "claude-test",
		MaxOutputTokens: &maxTokens,
		Temperature:     &temperature,
		TopK:            &topK,
		Reasoning:       &unified.ReasoningConfig{Effort: unified.ReasoningEffortHigh, MaxTokens: &budget},
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "think"}},
		}},
		Stream: true,
	}}

	wire, err := (Codec{}).EncodeRequest(context.Background(), &req)
	if err != nil {
		t.Fatal(err)
	}
	if wire.Thinking == nil || wire.Thinking.Type != "enabled" || wire.Thinking.BudgetTokens != 2048 {
		t.Fatalf("unexpected thinking config: %+v", wire.Thinking)
	}
	if wire.Temperature == nil || *wire.Temperature != 1 {
		t.Fatalf("temperature = %v, want 1", wire.Temperature)
	}
	if wire.TopK != nil {
		t.Fatalf("top_k should be dropped with thinking: %v", *wire.TopK)
	}
	if len(req.Warnings) != 2 {
		t.Fatalf("warnings = %+v", req.Warnings)
	}
}

func TestCodecOpenRouterExtensionWarnings(t *testing.T) {
	maxTokens := 128
	req := adapt.Request{
		SourceAPI: adapt.ApiOpenRouterAnthropicMessages,
		Unified: unified.Request{
			Model:           "claude-test",
			MaxOutputTokens: &maxTokens,
			Messages: []unified.Message{{
				Role:    unified.RoleUser,
				Content: []unified.ContentPart{unified.TextPart{Text: "hello"}},
			}},
			Stream: true,
		},
	}
	if err := req.Unified.Extensions.Set(unified.ExtOpenRouterProvider, []string{"not-object"}); err != nil {
		t.Fatal(err)
	}
	wire, err := (Codec{}).EncodeRequest(context.Background(), &req)
	if err != nil {
		t.Fatal(err)
	}
	if len(wire.OpenRouterProvider) != 0 {
		t.Fatalf("invalid extension should be dropped: %+v", wire.OpenRouterProvider)
	}
	if len(req.Warnings) != 1 || req.Warnings[0].Code != "invalid_extension_dropped" {
		t.Fatalf("warnings = %+v", req.Warnings)
	}
}

func TestCodecEncodeAssistantReasoningSignature(t *testing.T) {
	maxTokens := 128
	req := adapt.Request{Unified: unified.Request{
		Model:           "claude-test",
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{{
			Role: unified.RoleAssistant,
			Content: []unified.ContentPart{
				unified.ReasoningPart{Text: "think", Signature: "sig"},
				unified.TextPart{Text: "answer"},
			},
		}},
		Stream: true,
	}}

	wire, err := (Codec{}).EncodeRequest(context.Background(), &req)
	if err != nil {
		t.Fatal(err)
	}
	if len(wire.Messages) != 1 || len(wire.Messages[0].Content) != 2 {
		t.Fatalf("messages = %+v", wire.Messages)
	}
	block := wire.Messages[0].Content[0]
	if block.Type != "thinking" || block.Thinking != "think" || block.Signature != "sig" {
		t.Fatalf("thinking block = %+v", block)
	}
}

func TestCodecEncodeImageContent(t *testing.T) {
	maxTokens := 128
	req := adapt.Request{Unified: unified.Request{
		Model:           "claude-test",
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{{
			Role: unified.RoleUser,
			Content: []unified.ContentPart{
				unified.TextPart{Text: "describe"},
				unified.ImagePart{Source: unified.BlobSource{Kind: unified.BlobSourceURL, URL: "https://example.com/image.png"}},
				unified.ImagePart{Source: unified.BlobSource{Kind: unified.BlobSourceBase64, MIMEType: "image/png", Base64: "aW1hZ2U="}},
			},
		}},
	}}

	wire, err := (Codec{}).EncodeRequest(context.Background(), &req)
	if err != nil {
		t.Fatal(err)
	}
	blocks := wire.Messages[0].Content
	if len(blocks) != 3 || blocks[1].Source.Type != "url" || blocks[1].Source.URL != "https://example.com/image.png" {
		t.Fatalf("url image block = %+v", blocks)
	}
	if blocks[2].Source.Type != "base64" || blocks[2].Source.MediaType != "image/png" || blocks[2].Source.Data != "aW1hZ2U=" {
		t.Fatalf("base64 image block = %+v", blocks[2])
	}
}

func TestCodecStrictUnsupportedMultimodal(t *testing.T) {
	maxTokens := 128
	req := adapt.Request{
		MappingMode: adapt.MappingModeStrict,
		Unified: unified.Request{
			Model:           "claude-test",
			MaxOutputTokens: &maxTokens,
			Messages: []unified.Message{{
				Role: unified.RoleUser,
				Content: []unified.ContentPart{
					unified.AudioPart{Source: unified.BlobSource{Kind: unified.BlobSourceURL, URL: "https://example.com/audio.wav"}},
				},
			}},
		},
	}
	_, err := (Codec{}).EncodeRequest(context.Background(), &req)
	var unsupported *adapt.UnsupportedFieldError
	if !errors.As(err, &unsupported) {
		t.Fatalf("err = %v, want UnsupportedFieldError", err)
	}
}

func TestCodecBestEffortUnsupportedMultimodalWarning(t *testing.T) {
	maxTokens := 128
	req := adapt.Request{
		MappingMode: adapt.MappingModeBestEffort,
		Unified: unified.Request{
			Model:           "claude-test",
			MaxOutputTokens: &maxTokens,
			Messages: []unified.Message{{
				Role: unified.RoleUser,
				Content: []unified.ContentPart{
					unified.TextPart{Text: "describe"},
					unified.FilePart{Source: unified.BlobSource{Kind: unified.BlobSourceURL, URL: "https://example.com/file.pdf"}},
				},
			}},
		},
	}
	wire, err := (Codec{}).EncodeRequest(context.Background(), &req)
	if err != nil {
		t.Fatal(err)
	}
	if len(wire.Messages[0].Content) != 1 || wire.Messages[0].Content[0].Text != "describe" {
		t.Fatalf("unexpected encoded content: %+v", wire.Messages[0].Content)
	}
	if len(req.Warnings) != 1 || req.Warnings[0].Code != "unsupported_field_dropped" || req.Warnings[0].Field != "content" {
		t.Fatalf("warnings = %+v", req.Warnings)
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
