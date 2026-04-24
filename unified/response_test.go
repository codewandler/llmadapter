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
	ch <- UsageEvent{InputTokens: 1, OutputTokens: 2}
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
	if resp.Usage.TotalTokens != 3 {
		t.Fatalf("usage total = %d, want 3", resp.Usage.TotalTokens)
	}

	want := errors.New("boom")
	errCh := make(chan Event, 1)
	errCh <- ErrorEvent{Err: want}
	close(errCh)
	if _, err := Collect(context.Background(), errCh); !errors.Is(err, want) {
		t.Fatalf("Collect err = %v, want %v", err, want)
	}
}
