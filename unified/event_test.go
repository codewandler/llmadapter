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
		RouteEvent{},
		ProviderExecutionEvent{},
		ErrorEvent{},
	}
	if len(events) != 18 {
		t.Fatalf("unexpected event count")
	}
}
