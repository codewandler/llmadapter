package unified

import (
	"context"
	"errors"
	"testing"
)

func TestCollectTextToolUsageAndError(t *testing.T) {
	ch := make(chan Event, 10)
	ch <- MessageStartEvent{ID: "msg", Model: "model"}
	ch <- ContentBlockStartEvent{Index: 0, Kind: ContentKindText}
	ch <- TextDeltaEvent{Index: 0, Text: "hello "}
	ch <- TextDeltaEvent{Index: 0, Text: "world"}
	ch <- ContentBlockDoneEvent{Index: 0, Kind: ContentKindText}
	ch <- ToolCallStartEvent{Index: 1, ID: "toolu", Name: "lookup"}
	ch <- ToolCallArgsDeltaEvent{Index: 1, Delta: `{"q":"x"}`}
	ch <- ToolCallDoneEvent{Index: 1, ID: "toolu", Name: "lookup"}
	ch <- NewUsageEvent(TokenItems{
		{Kind: TokenKindInputNew, Count: 1},
		{Kind: TokenKindOutput, Count: 2},
	}, nil)
	ch <- CompletedEvent{FinishReason: FinishReasonStop}
	close(ch)

	resp, err := Collect(context.Background(), ch)
	if err != nil {
		t.Fatal(err)
	}
	if resp.ID != "msg" || resp.Model != "model" || resp.FinishReason != FinishReasonStop {
		t.Fatalf("unexpected response metadata: %+v", resp)
	}
	if len(resp.Content) != 1 || resp.Content[0].(TextPart).Text != "hello world" {
		t.Fatalf("unexpected content: %+v", resp.Content)
	}
	if len(resp.ToolCalls) != 1 || string(resp.ToolCalls[0].Arguments) != `{"q":"x"}` {
		t.Fatalf("unexpected tools: %+v", resp.ToolCalls)
	}
	if resp.Usage.TotalTokens() != 3 {
		t.Fatalf("usage total = %d, want 3", resp.Usage.TotalTokens())
	}

	want := errors.New("boom")
	errCh := make(chan Event, 1)
	errCh <- ErrorEvent{Err: want}
	close(errCh)
	if _, err := Collect(context.Background(), errCh); !errors.Is(err, want) {
		t.Fatalf("Collect err = %v, want %v", err, want)
	}
}

func TestCollectReasoningSignature(t *testing.T) {
	ch := make(chan Event, 5)
	ch <- ContentBlockStartEvent{Index: 0, Kind: ContentKindReasoning}
	ch <- ReasoningDeltaEvent{Index: 0, Text: "think"}
	ch <- ReasoningDeltaEvent{Index: 0, Signature: "sig"}
	ch <- ContentBlockDoneEvent{Index: 0, Kind: ContentKindReasoning}
	close(ch)

	resp, err := Collect(context.Background(), ch)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Content) != 1 {
		t.Fatalf("content = %+v", resp.Content)
	}
	part, ok := resp.Content[0].(ReasoningPart)
	if !ok || part.Text != "think" || part.Signature != "sig" {
		t.Fatalf("reasoning = %+v", resp.Content[0])
	}
}

func TestCollectPreservesCitationsAndRawEvents(t *testing.T) {
	ch := make(chan Event, 2)
	ch <- CitationEvent{Index: 0, Citation: Citation{Type: "url", URL: "https://example.test", Title: "Example"}}
	ch <- RawEvent{APIKind: "test.api", Type: "provider_specific", JSON: []byte(`{"x":1}`)}
	close(ch)

	resp, err := Collect(context.Background(), ch)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Citations) != 1 || resp.Citations[0].Citation.URL != "https://example.test" {
		t.Fatalf("citations = %+v", resp.Citations)
	}
	if len(resp.Raw) != 1 || resp.Raw[0].APIKind != "test.api" || string(resp.Raw[0].JSON) != `{"x":1}` {
		t.Fatalf("raw = %+v", resp.Raw)
	}
}
