package messages

import (
	"testing"

	"github.com/codewandler/llmadapter/transport"
)

func TestNewClientOptions(t *testing.T) {
	if _, err := NewClient(); err == nil {
		t.Fatalf("expected missing API key error")
	}
	client, err := NewClient(
		WithAPIKey("key"),
		WithBaseURL("https://anthropic.test/"),
		WithVersion("2024-01-01"),
		WithBeta("beta-a"),
		WithTransport(&transport.FakeByteStreamTransport{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if client == nil {
		t.Fatalf("client is nil")
	}
}
