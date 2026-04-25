package providers_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	anthropic "github.com/codewandler/llmadapter/providers/anthropic/messages"
	openai "github.com/codewandler/llmadapter/providers/openai/chatcompletions"
	openrouterresponses "github.com/codewandler/llmadapter/providers/openrouter/responses"
	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

func TestProviderImageEncodeConformance(t *testing.T) {
	cases := []struct {
		name      string
		frames    [][]byte
		newClient func(transport.ByteStreamTransport) (unified.Client, error)
		wantURL   string
		wantData  string
	}{
		{
			name:     "openai_chat",
			frames:   chatCompleteFrames(),
			wantURL:  `"url":"https://example.com/image.png"`,
			wantData: `"url":"data:image/png;base64,aW1hZ2U="`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openai.NewClient(openai.WithAPIKey("key"), openai.WithTransport(t))
			},
		},
		{
			name:     "openrouter_responses",
			frames:   responsesCompleteFrames(),
			wantURL:  `"image_url":"https://example.com/image.png"`,
			wantData: `"image_url":"data:image/png;base64,aW1hZ2U="`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openrouterresponses.NewClient(openrouterresponses.WithAPIKey("key"), openrouterresponses.WithTransport(t))
			},
		},
		{
			name:     "anthropic_messages",
			frames:   anthropicCompleteFrames(),
			wantURL:  `"type":"url","url":"https://example.com/image.png"`,
			wantData: `"type":"base64","media_type":"image/png","data":"aW1hZ2U="`,
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return anthropic.NewClient(anthropic.WithAPIKey("key"), anthropic.WithTransport(t))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &transport.FakeByteStreamTransport{Frames: tc.frames}
			client, err := tc.newClient(fake)
			if err != nil {
				t.Fatal(err)
			}
			events, err := client.Request(context.Background(), imageSmokeRequest())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := unified.Collect(context.Background(), events); err != nil {
				t.Fatal(err)
			}
			body := requestBody(t, fake)
			if !strings.Contains(body, tc.wantURL) || !strings.Contains(body, tc.wantData) {
				t.Fatalf("encoded body missing image forms:\n%s", body)
			}
		})
	}
}

func TestProviderUnsupportedMultimodalWarnings(t *testing.T) {
	cases := []struct {
		name      string
		frames    [][]byte
		newClient func(transport.ByteStreamTransport) (unified.Client, error)
	}{
		{
			name:   "openai_chat",
			frames: chatCompleteFrames(),
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openai.NewClient(openai.WithAPIKey("key"), openai.WithTransport(t))
			},
		},
		{
			name:   "openrouter_responses",
			frames: responsesCompleteFrames(),
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openrouterresponses.NewClient(openrouterresponses.WithAPIKey("key"), openrouterresponses.WithTransport(t))
			},
		},
		{
			name:   "anthropic_messages",
			frames: anthropicCompleteFrames(),
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return anthropic.NewClient(anthropic.WithAPIKey("key"), anthropic.WithTransport(t))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := tc.newClient(&transport.FakeByteStreamTransport{Frames: tc.frames})
			if err != nil {
				t.Fatal(err)
			}
			events, err := client.Request(context.Background(), unsupportedMultimodalRequest())
			if err != nil {
				t.Fatal(err)
			}
			resp, err := unified.Collect(context.Background(), events)
			if err != nil {
				t.Fatal(err)
			}
			if len(resp.Warnings) == 0 {
				t.Fatalf("expected unsupported multimodal warnings")
			}
			for _, warning := range resp.Warnings {
				if warning.Code != "unsupported_field_dropped" {
					t.Fatalf("unexpected warning: %+v", warning)
				}
			}
		})
	}
}

func TestProviderUnsupportedMultimodalDroppedFromWire(t *testing.T) {
	cases := []struct {
		name      string
		frames    [][]byte
		newClient func(transport.ByteStreamTransport) (unified.Client, error)
		forbidden []string
	}{
		{
			name:      "openai_chat",
			frames:    chatCompleteFrames(),
			forbidden: []string{"audio.wav", "file.pdf", "video.mp4", "input_audio", "input_file", "video"},
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openai.NewClient(openai.WithAPIKey("key"), openai.WithTransport(t))
			},
		},
		{
			name:      "openrouter_responses",
			frames:    responsesCompleteFrames(),
			forbidden: []string{"audio.wav", "file.pdf", "video.mp4", "input_audio", "input_file", "input_video"},
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openrouterresponses.NewClient(openrouterresponses.WithAPIKey("key"), openrouterresponses.WithTransport(t))
			},
		},
		{
			name:      "anthropic_messages",
			frames:    anthropicCompleteFrames(),
			forbidden: []string{"audio.wav", "file.pdf", "video.mp4", `"type":"audio"`, `"type":"document"`, `"type":"video"`},
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return anthropic.NewClient(anthropic.WithAPIKey("key"), anthropic.WithTransport(t))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &transport.FakeByteStreamTransport{Frames: tc.frames}
			client, err := tc.newClient(fake)
			if err != nil {
				t.Fatal(err)
			}
			events, err := client.Request(context.Background(), unsupportedMultimodalRequest())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := unified.Collect(context.Background(), events); err != nil {
				t.Fatal(err)
			}
			body := requestBody(t, fake)
			for _, forbidden := range tc.forbidden {
				if strings.Contains(body, forbidden) {
					t.Fatalf("unsupported media leaked into wire body (%s):\n%s", forbidden, body)
				}
			}
		})
	}
}

func TestProviderUnsupportedBuiltInToolWarnings(t *testing.T) {
	cases := []struct {
		name      string
		frames    [][]byte
		newClient func(transport.ByteStreamTransport) (unified.Client, error)
		forbidden []string
	}{
		{
			name:      "openai_chat",
			frames:    chatCompleteFrames(),
			forbidden: []string{"web_search", "code_interpreter"},
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openai.NewClient(openai.WithAPIKey("key"), openai.WithTransport(t))
			},
		},
		{
			name:      "openrouter_responses",
			frames:    responsesCompleteFrames(),
			forbidden: []string{"web_search", "code_interpreter"},
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return openrouterresponses.NewClient(openrouterresponses.WithAPIKey("key"), openrouterresponses.WithTransport(t))
			},
		},
		{
			name:      "anthropic_messages",
			frames:    anthropicCompleteFrames(),
			forbidden: []string{"web_search", "code_interpreter"},
			newClient: func(t transport.ByteStreamTransport) (unified.Client, error) {
				return anthropic.NewClient(anthropic.WithAPIKey("key"), anthropic.WithTransport(t))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &transport.FakeByteStreamTransport{Frames: tc.frames}
			client, err := tc.newClient(fake)
			if err != nil {
				t.Fatal(err)
			}
			events, err := client.Request(context.Background(), unsupportedBuiltInToolRequest())
			if err != nil {
				t.Fatal(err)
			}
			resp, err := unified.Collect(context.Background(), events)
			if err != nil {
				t.Fatal(err)
			}
			assertUnsupportedWarning(t, resp.Warnings)
			body := requestBody(t, fake)
			for _, forbidden := range tc.forbidden {
				if strings.Contains(body, forbidden) {
					t.Fatalf("unsupported built-in tool leaked into wire body (%s):\n%s", forbidden, body)
				}
			}
		})
	}
}

func imageSmokeRequest() unified.Request {
	maxTokens := 16
	return unified.Request{
		Model:           "test-model",
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{{
			Role: unified.RoleUser,
			Content: []unified.ContentPart{
				unified.TextPart{Text: "describe"},
				unified.ImagePart{Source: unified.BlobSource{Kind: unified.BlobSourceURL, URL: "https://example.com/image.png"}},
				unified.ImagePart{Source: unified.BlobSource{Kind: unified.BlobSourceBase64, MIMEType: "image/png", Base64: "aW1hZ2U="}},
			},
		}},
		Stream: true,
	}
}

func unsupportedMultimodalRequest() unified.Request {
	maxTokens := 16
	return unified.Request{
		Model:           "test-model",
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{{
			Role: unified.RoleUser,
			Content: []unified.ContentPart{
				unified.TextPart{Text: "describe"},
				unified.AudioPart{Source: unified.BlobSource{Kind: unified.BlobSourceURL, URL: "https://example.com/audio.wav"}},
				unified.FilePart{Source: unified.BlobSource{Kind: unified.BlobSourceURL, URL: "https://example.com/file.pdf"}, Filename: "file.pdf"},
				unified.VideoPart{Source: unified.BlobSource{Kind: unified.BlobSourceURL, URL: "https://example.com/video.mp4"}},
			},
		}},
		Stream: true,
	}
}

func unsupportedBuiltInToolRequest() unified.Request {
	maxTokens := 16
	return unified.Request{
		Model:           "test-model",
		MaxOutputTokens: &maxTokens,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "search"}},
		}},
		Tools: []unified.Tool{
			{Kind: unified.ToolKind("web_search"), Name: "web_search"},
			{Kind: unified.ToolKind("code_interpreter"), Name: "code_interpreter"},
			{Kind: unified.ToolKindFunction, Name: "lookup", InputSchema: json.RawMessage(`{"type":"object"}`)},
		},
		Stream: true,
	}
}

func assertUnsupportedWarning(t *testing.T, warnings []unified.WarningEvent) {
	t.Helper()
	for _, warning := range warnings {
		if warning.Code == "unsupported_field_dropped" {
			return
		}
	}
	t.Fatalf("missing unsupported warning: %+v", warnings)
}

func requestBody(t *testing.T, fake *transport.FakeByteStreamTransport) string {
	t.Helper()
	if len(fake.Seen) != 1 {
		t.Fatalf("seen requests = %d", len(fake.Seen))
	}
	var payload json.RawMessage
	if err := json.NewDecoder(fake.Seen[0].Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

func chatCompleteFrames() [][]byte {
	return [][]byte{
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{"role":"assistant"}}]}`),
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{"content":"ok"}}]}`),
		[]byte(`data: {"id":"chatcmpl","model":"gpt-test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`),
		[]byte(`data: [DONE]`),
	}
}

func responsesCompleteFrames() [][]byte {
	return [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"gpt-test","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.output_text.delta","response_id":"resp_1","output_index":0,"delta":"ok"}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"gpt-test","status":"completed"}}`),
		[]byte(`data: [DONE]`),
	}
}

func anthropicCompleteFrames() [][]byte {
	return [][]byte{
		[]byte(`event: message_start
data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-test","content":[]}}`),
		[]byte(`event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`),
		[]byte(`event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}`),
		[]byte(`event: content_block_stop
data: {"type":"content_block_stop","index":0}`),
		[]byte(`event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`),
		[]byte(`event: message_stop
data: {"type":"message_stop"}`),
	}
}
