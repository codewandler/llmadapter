package pipeline

import (
	"context"
	"testing"

	"github.com/codewandler/llmadapter/unified"
)

func TestBuiltInProcessors(t *testing.T) {
	coalescer := &TextCoalescer{MaxBytes: 5}
	if got, err := coalescer.Push(context.Background(), unified.TextDeltaEvent{Index: 0, Text: "he"}); err != nil || len(got) != 0 {
		t.Fatalf("first push = %v,%v", got, err)
	}
	got, err := coalescer.Push(context.Background(), unified.TextDeltaEvent{Index: 0, Text: "llo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].(unified.TextDeltaEvent).Text != "hello" {
		t.Fatalf("coalesced = %+v", got)
	}

	filter := ReasoningFilter{Expose: false}
	got, err = filter.Push(context.Background(), unified.ReasoningDeltaEvent{Text: "hidden"})
	if err != nil || len(got) != 0 {
		t.Fatalf("reasoning filter = %+v,%v", got, err)
	}

	injector := &CompletionInjector{}
	got, err = injector.Close(context.Background())
	if err != nil || len(got) != 1 {
		t.Fatalf("injector close = %+v,%v", got, err)
	}
	if got[0].(unified.CompletedEvent).FinishReason != unified.FinishReasonUnknown {
		t.Fatalf("unexpected completion: %+v", got[0])
	}
}
