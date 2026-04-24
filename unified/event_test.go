package unified

import "testing"

func TestEventsSatisfyInterface(t *testing.T) {
	events := []Event{
		MessageStartEvent{},
		MessageDoneEvent{},
		ContentBlockStartEvent{},
		ContentBlockDoneEvent{},
		TextDeltaEvent{},
		ReasoningDeltaEvent{},
		RefusalDeltaEvent{},
		ToolCallStartEvent{},
		ToolCallArgsDeltaEvent{},
		ToolCallDoneEvent{},
		CitationEvent{},
		UsageEvent{},
		CompletedEvent{},
		WarningEvent{},
		RawEvent{},
		ErrorEvent{},
	}
	if len(events) != 16 {
		t.Fatalf("unexpected event count")
	}
}
