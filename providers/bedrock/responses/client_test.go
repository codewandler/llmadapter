package responses

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/codewandler/llmadapter/transport"
	"github.com/codewandler/llmadapter/unified"
)

func TestGenerateTokenShape(t *testing.T) {
	token, err := GenerateToken(context.Background(), aws.Credentials{
		AccessKeyID:     "AKIDEXAMPLE",
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY",
		SessionToken:    "session",
	}, "us-east-1", 3600, time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	const tokenPrefix = "bedrock-api-key-"
	if !strings.HasPrefix(token, tokenPrefix) {
		t.Fatalf("token prefix = %q", token)
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(token, tokenPrefix))
	if err != nil {
		t.Fatalf("decode token payload: %v", err)
	}
	decoded := string(payload)
	for _, want := range []string{
		"bedrock.amazonaws.com/?Action=CallWithBearerToken",
		"X-Amz-Algorithm=AWS4-HMAC-SHA256",
		"X-Amz-Credential=",
		"X-Amz-Expires=3600",
		"X-Amz-Security-Token=session",
		"&Version=1",
	} {
		if !strings.Contains(decoded, want) {
			t.Fatalf("decoded token %q does not contain %q", decoded, want)
		}
	}
}

func TestDefaultBaseURLUsesAWSRegion(t *testing.T) {
	t.Setenv(EnvRegion, "eu-central-1")
	t.Setenv(EnvDefaultRegion, "us-west-2")
	if got, want := defaultBaseURL(), "https://bedrock-mantle.eu-central-1.api.aws"; got != want {
		t.Fatalf("default base URL = %q, want %q", got, want)
	}
}

func TestDefaultBaseURLUsesAWSDefaultRegion(t *testing.T) {
	t.Setenv(EnvRegion, "")
	t.Setenv(EnvDefaultRegion, "us-west-2")
	if got, want := defaultBaseURL(), "https://bedrock-mantle.us-west-2.api.aws"; got != want {
		t.Fatalf("default base URL = %q, want %q", got, want)
	}
}

func TestClientUsesMantleResponsesURLAndAPIKind(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"openai.gpt-oss-120b","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.content_part.added","response_id":"resp_1","output_index":0,"content_index":0,"part":{"type":"output_text","text":""}}`),
		[]byte(`data: {"type":"response.content_part.delta","response_id":"resp_1","output_index":0,"content_index":0,"delta":"ok"}`),
		[]byte(`data: {"type":"response.bedrock_extension","response_id":"resp_1","marker":"kept"}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"openai.gpt-oss-120b","status":"completed","usage":{"input_tokens":4,"output_tokens":1,"total_tokens":5}}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(
		WithAPIKey("key"),
		WithBaseURL("https://bedrock-mantle.eu-central-1.api.aws/v1"),
		WithTransport(fake),
	)
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{
		Model:    DefaultModel,
		Messages: []unified.Message{{Role: unified.RoleUser, Content: []unified.ContentPart{unified.TextPart{Text: "hello"}}}},
		Stream:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if responseText(resp) != "ok" {
		t.Fatalf("response text = %q", responseText(resp))
	}
	if len(fake.Seen) != 1 {
		t.Fatalf("requests = %d, want 1", len(fake.Seen))
	}
	if got, want := fake.Seen[0].URL, "https://bedrock-mantle.eu-central-1.api.aws/v1/responses"; got != want {
		t.Fatalf("request URL = %q, want %q", got, want)
	}
	if got := fake.Seen[0].Header.Get("Authorization"); got != "Bearer key" {
		t.Fatalf("authorization header = %q", got)
	}
	if len(resp.Raw) == 0 || resp.Raw[0].APIKind != defaultAPIKindName {
		t.Fatalf("raw API kind = %+v, want %q", resp.Raw, defaultAPIKindName)
	}
}

func TestClientGeneratesAndCachesAWSToken(t *testing.T) {
	var calls atomic.Int32
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"openai.gpt-oss-120b","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.output_text.delta","response_id":"resp_1","output_index":0,"delta":"ok"}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"openai.gpt-oss-120b","status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`),
		[]byte(`data: [DONE]`),
	}}
	provider := TokenProviderFunc(func(context.Context) (string, error) {
		calls.Add(1)
		return "generated-token", nil
	})
	client, err := NewClient(WithTokenProvider(provider), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		events, err := client.Request(context.Background(), unified.Request{Model: DefaultModel, Stream: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := unified.Collect(context.Background(), events); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("token provider calls = %d, want 2 transport injections", calls.Load())
	}
	if len(fake.Seen) != 2 {
		t.Fatalf("requests = %d, want 2", len(fake.Seen))
	}
	for _, req := range fake.Seen {
		if got := req.Header.Get("Authorization"); got != "Bearer generated-token" {
			t.Fatalf("authorization header = %q", got)
		}
	}
}

func TestClientDowngradesUnsupportedToolChoiceToAuto(t *testing.T) {
	fake := &transport.FakeByteStreamTransport{Frames: [][]byte{
		[]byte(`data: {"type":"response.created","response":{"id":"resp_1","model":"openai.gpt-oss-120b","status":"in_progress"}}`),
		[]byte(`data: {"type":"response.output_text.delta","response_id":"resp_1","output_index":0,"delta":"ok"}`),
		[]byte(`data: {"type":"response.done","response":{"id":"resp_1","model":"openai.gpt-oss-120b","status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`),
		[]byte(`data: [DONE]`),
	}}
	client, err := NewClient(WithAPIKey("key"), WithTransport(fake))
	if err != nil {
		t.Fatal(err)
	}
	events, err := client.Request(context.Background(), unified.Request{
		Model: DefaultModel,
		Messages: []unified.Message{{
			Role:    unified.RoleUser,
			Content: []unified.ContentPart{unified.TextPart{Text: "use the tool"}},
		}},
		Tools: []unified.Tool{{
			Kind:        unified.ToolKindFunction,
			Name:        "lookup_city",
			Description: "Looks up a city.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
		}},
		ToolChoice: &unified.ToolChoice{Mode: unified.ToolChoiceTool, Name: "lookup_city"},
		Stream:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := unified.Collect(context.Background(), events)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Warnings) != 1 || resp.Warnings[0].Code != "unsupported_field_dropped" {
		t.Fatalf("warnings = %+v", resp.Warnings)
	}
	var body map[string]any
	if err := json.NewDecoder(fake.Seen[0].Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if got, want := body["tool_choice"], "auto"; got != want {
		t.Fatalf("tool_choice = %#v, want %q; body=%#v", got, want, body)
	}
}

func responseText(resp unified.Response) string {
	var b strings.Builder
	for _, part := range resp.Content {
		if text, ok := part.(unified.TextPart); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}
