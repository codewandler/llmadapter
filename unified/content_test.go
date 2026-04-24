package unified

import "testing"

func TestContentPartKinds(t *testing.T) {
	tests := []struct {
		part ContentPart
		kind ContentKind
	}{
		{TextPart{}, ContentKindText},
		{ImagePart{}, ContentKindImage},
		{AudioPart{}, ContentKindAudio},
		{VideoPart{}, ContentKindVideo},
		{FilePart{}, ContentKindFile},
		{ReasoningPart{}, ContentKindReasoning},
		{RefusalPart{}, ContentKindRefusal},
	}
	for _, tt := range tests {
		if got := tt.part.contentKind(); got != tt.kind {
			t.Fatalf("kind = %q, want %q", got, tt.kind)
		}
	}
}
